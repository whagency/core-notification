package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"notification-service/test/bootstrap"

	"github.com/whagency/core-common/logger"
)

func TestWebsocketLoggingMiddleware_AddsRequestContext(t *testing.T) {
	bootstrap.InitLogger(t)

	const requestID = "req-123"
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true

		if got, _ := r.Context().Value(logger.ContextKeyRequestID).(string); got != requestID {
			t.Fatalf("request id = %q; want %q", got, requestID)
		}
		if got := r.Context().Value(logger.ContextKeyLogger); got == nil {
			t.Fatal("expected context logger")
		}

		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set(logger.DefaultRequestIDHeader, requestID)
	rec := httptest.NewRecorder()

	WebsocketLoggingMiddleware(next).ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusNoContent)
	}
}

func TestWebsocketLoggingMiddleware_GeneratesMissingRequestID(t *testing.T) {
	bootstrap.InitLogger(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ := r.Context().Value(logger.ContextKeyRequestID).(string)
		if got == "" {
			t.Fatal("expected generated request id")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()

	WebsocketLoggingMiddleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}
