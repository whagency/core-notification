package middlewares

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"notification-service/config"
	"notification-service/internal/domain"

	appErrors "github.com/whagency/core-common/errorsx"
	"github.com/whagency/core-common/logger"
	"github.com/whagency/core-common/services/user"
)

// ValidateUserSession validates the user session
func ValidateUserSession(next http.Handler, userService domain.UserService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var canSend bool
		var userID uint64
		authToken := r.URL.Query().Get(config.Websocket().SecurityTokenKey)
		if authToken == "" {
			http.Error(w, appErrors.UserSessionTokenInvalidErrMessage, http.StatusUnauthorized)
			return
		}

		if authToken == config.Websocket().SecurityToken {
			canSend = true
		} else {
			userSession, err := userService.ValidateSession(ctx, authToken, true)
			if err != nil {
				er := appErrors.New(
					decodeErrCodeBasedOnUserSessionError(err),
					appErrors.WithCause(fmt.Errorf("[middleware:ValidateUserSession]%w", err)),
				)

				if er.Code == appErrors.InternalServerError {
					logger.LogCtx(ctx).Error(er.Cause)
				}

				http.Error(w, er.Message, er.HTTPStatus)
				return
			}
			userID = userSession.User.ID
		}

		ctx = context.WithValue(ctx, domain.ContextWebsocketUserIDKey, userID)
		ctx = context.WithValue(ctx, domain.ContextWebsocketCanSendKey, canSend)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// decodeErrCodeBasedOnUserSessionError decodes the error code based on the type of user session error.
func decodeErrCodeBasedOnUserSessionError(err error) int {
	switch {
	case errors.Is(err, user.ErrSessionExpired):
		return appErrors.UserSessionExpiredError
	case errors.Is(err, user.ErrSessionInvalid), errors.Is(err, user.ErrUserNotFound):
		return appErrors.UserSessionTokenInvalidError
	default:
		return appErrors.InternalServerError
	}
}
