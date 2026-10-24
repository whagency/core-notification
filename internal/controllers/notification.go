package controllers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"notification-service/internal/domain"
	svcErrors "notification-service/internal/domain/errors"

	appErrors "github.com/whagency/core-common/errorsx"
	"github.com/whagency/core-common/logger"
	"github.com/whagency/core-common/services/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const _userIdParamKey = "userId"

type NotificationCtrl struct {
	mailer        domain.MailerManager
	websocketCtrl domain.WebsocketController
	userGRPC      domain.UserServiceGRPC

	dedupeKeys domain.DedupeKeyRepository
	dedupeTTL  time.Duration
}

// NewNotificationController creates a new NotificationCtrl with the given options.
// dedupeKeys may be nil, in which case no deduplication is performed.
func NewNotificationController(
	mailer domain.MailerManager,
	websocketCtrl domain.WebsocketController,
	userGRPC domain.UserServiceGRPC,
	dedupeKeys domain.DedupeKeyRepository,
	dedupeTTL time.Duration,
) *NotificationCtrl {
	return &NotificationCtrl{
		mailer:        mailer,
		websocketCtrl: websocketCtrl,
		userGRPC:      userGRPC,
		dedupeKeys:    dedupeKeys,
		dedupeTTL:     dedupeTTL,
	}
}

// Send sends a notification, fanning it out to every configured channel.
//
// When cmd carries a DedupeKey each channel is fenced independently: the fence
// is reserved before delivery and released again if that channel fails, so a
// redelivery redelivers only what actually failed.
//
// The fence is per channel rather than per command because a command usually
// fans out to several (a successful deposit goes to both WebSocket and e-mail)
// and those succeed or fail independently. Send reports failure if any channel
// failed, so the push consumer's own retry pipeline redelivers the whole
// message — and without a per-channel fence that redelivery would send the
// channels that had already succeeded a second time.
//
// That local retry is the main source of duplicates, not the producer: this
// consumer retries a failed push three times by default, so a single e-mail
// outage during a deposit notification meant up to four WebSocket messages for
// one event. At-least-once producers — anything publishing from a transactional
// outbox — are a second, rarer source.
//
// OTP commands are single-address, so a redelivery there repeats only the
// channel that failed. They carry no key and need none.
func (c NotificationCtrl) Send(ctx context.Context, cmd domain.SendCommand) error {
	logger.LogCtx(ctx).Debugf("[NotificationCtrl:Send][cmd: %+v]", cmd)

	type sendResult struct {
		index int
		err   error
	}

	resultChan := make(chan sendResult, len(cmd.To))
	var wg sync.WaitGroup

	for i, address := range cmd.To {
		wg.Add(1)
		go func(index int, address domain.Address) {
			defer wg.Done()

			if err := c.sendAddressFenced(ctx, cmd, address); err != nil {
				logger.LogCtx(ctx).Errorf("[NotificationCtrl:Send][channel:%d][to:%s]%v", address.Channel, address.Value, err)
				resultChan <- sendResult{
					index: index,
					err:   fmt.Errorf("[channel:%d][to:%s]%w", address.Channel, address.Value, err),
				}
			}
		}(i, address)
	}

	wg.Wait()
	close(resultChan)

	if len(resultChan) == 0 {
		return nil
	}

	orderedErrors := make([]error, len(cmd.To))
	for result := range resultChan {
		orderedErrors[result.index] = result.err
	}

	sendErrors := make([]error, 0, len(cmd.To))
	for _, err := range orderedErrors {
		if err != nil {
			sendErrors = append(sendErrors, err)
		}
	}

	if len(sendErrors) > 0 {
		return fmt.Errorf("[NotificationCtrl:Send]%w", errors.Join(sendErrors...))
	}

	return nil
}

// sendAddressFenced delivers to one address behind its own dedupe fence.
func (c NotificationCtrl) sendAddressFenced(ctx context.Context, cmd domain.SendCommand, address domain.Address) error {
	release, duplicate, err := c.reserveDelivery(ctx, fenceKey(cmd.DedupeKey, address.Channel))
	if err != nil {
		return err
	}
	if duplicate {
		logger.LogCtx(ctx).Infof(
			"[NotificationCtrl:Send][msg: duplicate delivery skipped][dedupe_key: %s][channel: %s]",
			cmd.DedupeKey, address.Channel,
		)
		return nil
	}

	if sendErr := c.sendAddress(ctx, cmd, address); sendErr != nil {
		release(ctx)
		return sendErr
	}
	return nil
}

// fenceKey scopes a command's dedupe key to one channel. An empty command key
// yields an empty fence key, which disables fencing.
func fenceKey(dedupeKey string, channel domain.Channel) string {
	if dedupeKey == "" {
		return ""
	}
	return dedupeKey + ":" + channel.String()
}

// reserveDelivery claims the fence for key. The returned release function undoes
// the reservation and is a no-op when nothing was reserved.
//
// The reservation is deliberately committed on its own, outside any transaction,
// before the delivery it guards. Wrapping reserve-send-release in a database
// transaction would defeat the fence: the insert would only become visible on
// commit, so a crash after the e-mail went out but before the commit would roll
// the fence back and let the retry send it again. It would also hold a pooled
// connection and the unique-index lock across an SMTP round trip, blocking every
// other consumer racing the same key instead of letting them return immediately.
//
// This is why it differs from restriction-service, where the equivalent
// idempotency key does join the business transaction: there the guarded effect
// is itself a database write, so key and effect must land atomically. Here the
// effect is external I/O and there is nothing to be atomic with.
func (c NotificationCtrl) reserveDelivery(ctx context.Context, key string) (release func(context.Context), duplicate bool, err error) {
	noop := func(context.Context) {}
	if key == "" || c.dedupeKeys == nil {
		return noop, false, nil
	}

	alreadyReserved, err := c.dedupeKeys.Reserve(ctx, key, c.dedupeTTL)
	if err != nil {
		// Treat a fence failure as retryable rather than delivering unfenced:
		// the producer retries, and a delayed notification beats a duplicate one.
		return noop, false, fmt.Errorf("[NotificationCtrl:reserveDelivery]%w", err)
	}
	if alreadyReserved {
		return noop, true, nil
	}

	return func(releaseCtx context.Context) {
		// Best effort. A failure here costs a retry that is skipped as a
		// duplicate, so it is logged loudly rather than swallowed.
		if releaseErr := c.dedupeKeys.Release(releaseCtx, key); releaseErr != nil {
			logger.LogCtx(releaseCtx).Errorf(
				"[NotificationCtrl:Send][msg: failed to release dedupe fence after send failure, retry will be skipped][dedupe_key: %s]%s",
				key, releaseErr,
			)
		}
	}, false, nil
}

// sendAddress sends the notification to a specific address.
func (c NotificationCtrl) sendAddress(ctx context.Context, cmd domain.SendCommand, address domain.Address) error {
	switch address.Channel {
	case domain.ChannelEmail:
		if err := c.sendEmail(ctx, cmd, address.Value); err != nil {
			return err
		}
	case domain.ChannelWebSocket:
		if err := c.sendWebsocket(ctx, cmd, address.Value); err != nil {
			return err
		}
	default:
		return svcErrors.New(
			svcErrors.UnsupportedNotificationChannelError,
			appErrors.WithCause(fmt.Errorf("[sendAddress][err: unsupported channel]")),
		)
	}

	return nil
}

// sendEmail sends an email.
func (c NotificationCtrl) sendEmail(ctx context.Context, cmd domain.SendCommand, to string) error {
	if to == "" || cmd.Locale == "" {

		userId := cmd.Data.GetString(_userIdParamKey)
		if userId == "" {
			if to == "" {
				return svcErrors.New(
					svcErrors.InvalidNotificationRecipientError,
					appErrors.WithCause(fmt.Errorf("[sendEmail][err: empty userId value in params]")),
				)
			}
			if cmd.Locale == "" {
				cmd.Locale = domain.LocaleEn
			}
		} else {
			dataUser, errUser := c.getDataFromUserProfile(ctx, userId)
			if errUser != nil {
				return fmt.Errorf("[sendEmail]%w", errUser)
			}

			if to == "" {
				if dataUser.ContactType != user.ContactTypeEmail {
					return svcErrors.New(
						svcErrors.NotificationProfileMismatchError,
						appErrors.WithCause(fmt.Errorf("[sendEmail][err: user contact type is not email]")),
					)
				}
				to = dataUser.ContactValue
			}

			if cmd.Locale == "" {
				cmd.Locale = domain.NewLocaleFromString(dataUser.Locale)
			}
		}
	}

	if err := c.mailer.Send(ctx, domain.MailMessage{
		Kind:   cmd.Kind,
		To:     to,
		Params: cmd.Data,
		Locale: cmd.Locale,
	}); err != nil {
		return fmt.Errorf("[sendEmail]%w", err)
	}

	return nil
}

// sendWebsocket sends a websocket message.
func (c NotificationCtrl) sendWebsocket(ctx context.Context, cmd domain.SendCommand, to string) error {
	toUint64, errTo := strconv.ParseUint(to, 10, 64)
	if errTo != nil {
		return svcErrors.New(
			svcErrors.InvalidNotificationRecipientError,
			appErrors.WithCause(fmt.Errorf("[sendWebsocket][err: invalid websocket recipient id]")),
		)
	}

	if err := c.websocketCtrl.Send(ctx, domain.WsMessage{
		Kind:   cmd.Kind,
		To:     toUint64,
		Params: cmd.Data,
		Locale: cmd.Locale,
	}); err != nil {
		return fmt.Errorf("[sendWebsocket]%w", err)
	}

	return nil
}

// getDataFromUserProfile retrieves the contact email and locale from the user profile.
func (c NotificationCtrl) getDataFromUserProfile(ctx context.Context, userId string) (user.Profile, error) {
	userIdFormatted, errId := domain.NewIDFromString(userId)
	if errId != nil {
		return user.Profile{}, svcErrors.New(
			svcErrors.InvalidNotificationRecipientError,
			appErrors.WithCause(fmt.Errorf("[getContactEmailFromUserProfile][err: invalid userId value in params]")),
		)
	}
	dataUser, errUser := c.userGRPC.GetProfile(ctx, userIdFormatted.Uint64())
	if errUser != nil {
		if status.Code(errUser) == codes.NotFound {
			return user.Profile{}, svcErrors.New(
				appErrors.RecordNotFoundError,
				appErrors.WithCause(fmt.Errorf("[getContactEmailFromUserProfile]%w", errUser)),
			)
		}
		return user.Profile{}, fmt.Errorf("[getContactEmailFromUserProfile]%w", errUser)
	}
	return dataUser, nil
}
