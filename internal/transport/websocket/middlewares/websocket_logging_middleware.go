package middlewares

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/whagency/core-common/logger"
	"go.uber.org/zap"
)

// WebsocketLoggingMiddleware is a middleware that adds logging capabilities to WebSocket handlers.
func WebsocketLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract or generate Request ID
		reqID := r.Header.Get(logger.DefaultRequestIDHeader)
		if reqID == "" {
			reqID = uuid.New().String()
		}

		// Generate Internal Trace ID
		internalID := uuid.New().String()

		// Create a logger with standard fields
		newLogger := logger.Log().With(
			zap.String(logger.ContextKeyRequestID.String(), reqID),
			zap.String(logger.ContextKeyInternalTraceID.String(), internalID),
			zap.String("client_ip", r.RemoteAddr),
			zap.String("path", r.URL.Path),
		)

		ctx := r.Context()
		ctx = context.WithValue(ctx, logger.ContextKeyRequestID, reqID)
		ctx = context.WithValue(ctx, logger.ContextKeyLogger, newLogger)

		r = r.WithContext(ctx)

		start := time.Now()

		next.ServeHTTP(w, r)

		newLogger.With(zap.Duration("latency", time.Since(start))).Debugf("[logger:WebsocketLoggingMiddleware][WS handler completed]")
	})
}
