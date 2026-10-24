package controllers

import (
	"context"
	"fmt"

	"notification-service/internal/domain"
	svcErrors "notification-service/internal/domain/errors"

	appErrors "github.com/whagency/core-common/errorsx"
	"github.com/whagency/core-common/logger"
)

type WebsocketCtrl struct {
	websocket domain.WebsocketManager
}

// NewWebsocketController creates a new WebsocketCtrl with the given options.
func NewWebsocketController(websocket domain.WebsocketManager) *WebsocketCtrl {
	return &WebsocketCtrl{
		websocket: websocket,
	}
}

// Send a websocket message.
func (c *WebsocketCtrl) Send(ctx context.Context, wsMessage domain.WsMessage) error {
	logger.LogCtx(ctx).Debugf("[WebsocketCtrl:Send][wsMessage: %+v]", wsMessage)

	message, errDecode := decodeMessageObject(wsMessage)
	if errDecode != nil {
		errDecode.SetCause(fmt.Errorf("[WebsocketCtrl:Send]%w", errDecode.Cause))
		return errDecode
	}

	if err := c.websocket.Send(ctx, message); err != nil {
		return fmt.Errorf("[WebsocketCtrl:Send]%w", err)
	}
	return nil
}

// decodeMessageObject returns the message structure with data for sending.
func decodeMessageObject(wsMessage domain.WsMessage) (any, *appErrors.ErrorResponse) {
	switch wsMessage.Kind {
	case domain.MsgPushSuccessfulDeposit:
		return domain.WsSuccessfulDepositMessage{
			Type:      domain.WsMessageTypeSuccessfulDeposit.String(),
			UserID:    wsMessage.To,
			AssetCode: wsMessage.Params.GetString("asset"),
			Amount:    wsMessage.Params.GetFloat("amount"),
		}, nil
	case domain.MsgPushRequiredVerificationDeposit:
		return domain.WsRequiredVerificationDepositMessage{
			Type:      domain.WsMessageTypeRequiredVerificationDeposit.String(),
			UserID:    wsMessage.To,
			AssetCode: wsMessage.Params.GetString("asset"),
			Amount:    wsMessage.Params.GetFloat("amount"),
		}, nil
	case domain.MsgPushSuccessfulWithdrawal:
		return domain.WsSuccessfulWithdrawalMessage{
			Type:      domain.WsMessageTypeSuccessfulWithdrawal.String(),
			UserID:    wsMessage.To,
			AssetCode: wsMessage.Params.GetString("asset"),
			Amount:    wsMessage.Params.GetFloat("amount"),
		}, nil
	case domain.MsgPushSuccessfulIncomingTransfer:
		return domain.WsSuccessfulIncomingTransfer{
			Type:      domain.WsMessageTypeSuccessfulIncomingTransfer.String(),
			UserID:    wsMessage.To,
			AssetCode: wsMessage.Params.GetString("asset"),
			Amount:    wsMessage.Params.GetFloat("amount"),
		}, nil
	default:
		return nil, svcErrors.New(
			svcErrors.UnsupportedNotificationKindError,
			appErrors.WithCause(fmt.Errorf("[decodeMessageObject][kind: %d][err: unsupported ws message kind]", wsMessage.Kind)),
		)
	}
}
