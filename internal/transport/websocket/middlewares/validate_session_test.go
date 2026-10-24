package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"notification-service/config"
	"notification-service/internal/domain"
	"notification-service/test/bootstrap"
	testmocks "notification-service/test/mocks"

	"github.com/stretchr/testify/mock"
	appErrors "github.com/whagency/core-common/errorsx"
	"github.com/whagency/core-common/services/user"
)

func TestValidateUserSession(t *testing.T) {
	bootstrap.InitLogger(t)
	initWebsocketMiddlewareTestConfig(t)

	cases := []struct {
		name        string
		token       string
		session     user.Session
		sessionErr  error
		wantStatus  int
		wantBody    string
		wantUserID  uint64
		wantCanSend bool
		wantCall    bool
	}{
		{
			name:       "missing token",
			wantStatus: http.StatusUnauthorized,
			wantBody:   "session token is invalid",
		},
		{
			name:        "privileged token can send",
			token:       "privileged-token",
			wantStatus:  http.StatusOK,
			wantCanSend: true,
		},
		{
			name:  "valid user session listens only",
			token: "session-token",
			session: user.Session{
				User: user.User{ID: 42},
			},
			wantStatus: http.StatusOK,
			wantUserID: 42,
			wantCall:   true,
		},
		{
			name:       "expired session",
			token:      "expired-token",
			sessionErr: user.ErrSessionExpired,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "user session has expired",
			wantCall:   true,
		},
		{
			name:       "invalid session",
			token:      "invalid-token",
			sessionErr: user.ErrSessionInvalid,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "session token is invalid",
			wantCall:   true,
		},
		{
			name:       "unknown validation error",
			token:      "broken-token",
			sessionErr: errors.New("user service unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal error",
			wantCall:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userService := testmocks.NewMockUserService(t)
			if tc.wantCall {
				userService.EXPECT().
					ValidateSession(mock.Anything, tc.token, true).
					Return(tc.session, tc.sessionErr)
			}

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true

				gotUserID, _ := r.Context().Value(domain.ContextWebsocketUserIDKey).(uint64)
				gotCanSend, _ := r.Context().Value(domain.ContextWebsocketCanSendKey).(bool)
				if gotUserID != tc.wantUserID {
					t.Fatalf("user id = %d; want %d", gotUserID, tc.wantUserID)
				}
				if gotCanSend != tc.wantCanSend {
					t.Fatalf("canSend = %v; want %v", gotCanSend, tc.wantCanSend)
				}

				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			if tc.token != "" {
				q := req.URL.Query()
				q.Set(config.Websocket().SecurityTokenKey, tc.token)
				req.URL.RawQuery = q.Encode()
			}
			rec := httptest.NewRecorder()

			ValidateUserSession(next, userService).ServeHTTP(rec, req.WithContext(context.Background()))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d; want %d; body=%q", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusOK && !nextCalled {
				t.Fatal("expected next handler to be called")
			}
			if tc.wantStatus != http.StatusOK && nextCalled {
				t.Fatal("expected next handler not to be called")
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("expected body containing %q, got %q", tc.wantBody, rec.Body.String())
			}
		})
	}
}

func TestDecodeErrCodeBasedOnUserSessionError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "expired", err: user.ErrSessionExpired, want: appErrors.UserSessionExpiredError},
		{name: "invalid", err: user.ErrSessionInvalid, want: appErrors.UserSessionTokenInvalidError},
		{name: "user missing", err: user.ErrUserNotFound, want: appErrors.UserSessionTokenInvalidError},
		{name: "unknown", err: errors.New("boom"), want: appErrors.InternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := decodeErrCodeBasedOnUserSessionError(tc.err); got != tc.want {
				t.Fatalf("code = %d; want %d", got, tc.want)
			}
		})
	}
}

func initWebsocketMiddlewareTestConfig(t testing.TB) {
	t.Helper()

	env := map[string]string{
		"APP_NAME":              "notification-service",
		"APP_ENVIRONMENT":       "test",
		"SECURITY_TOKEN":        "api-token",
		"USER_SERVICE_URL":      "http://user-service",
		"USER_SERVICE_GRPC_URL": "user-service:5051",
		"QUEUE_HOST":            "localhost",
		"QUEUE_PORT":            "5672",
		"QUEUE_USER":            "guest",
		"QUEUE_PASSWORD":        "guest",
		"DB_HOST":               "localhost",
		"DB_PORT":               "5432",
		"DB_USER":               "postgres",
		"DB_PASSWORD":           "postgres",
		"DB_NAME":               "notification_test",
		"MAIL_HOST":             "localhost",
		"MAIL_PORT":             "1025",
		"MAIL_USERNAME":         "user",
		"MAIL_PASSWORD":         "pass",
		"MAIL_FROM_ADDRESS":     "no-reply@example.test",
		"MAIL_FROM_NAME":        "app",
		"MAIL_ADMIN":            "admin@example.test",
		"WS_SECURITY_TOKEN":     "privileged-token",
		"WS_SECURITY_TOKEN_KEY": "t",
	}
	for key, value := range env {
		t.Setenv(key, value)
	}

	if err := config.Init(); err != nil {
		t.Fatalf("init config: %v", err)
	}
}
