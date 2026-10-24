package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	application "notification-service/cmd/notification-service/app"
	"notification-service/config"

	"github.com/whagency/core-common/logger"
	"github.com/whagency/core-common/tracer"
	"golang.org/x/sync/errgroup"
)

const _shutdownTimeout = 10 * time.Second

func main() {
	if err := runMain(); err != nil {
		log.Printf("[main]%s", err)
		os.Exit(1)
	}
}

func runMain() error {
	app := application.NewApplication()

	if err := config.Init(); err != nil {
		return err
	}

	if err := logger.Init(config.LoggerConfig()); err != nil {
		return err
	}
	defer logger.Sync()

	if err := tracer.Init(config.TracerConfig()); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, app); err != nil {
		return err
	}

	return nil
}

// run initializes and starts the application components.
func run(ctx context.Context, app *application.Application) error {
	logger.Log().Info("[main][msg: setting up dependencies]")
	if err := app.SetupDependencies(); err != nil {
		return err
	}

	logger.Log().Info("[main][msg: setting up startup data]")
	if err := app.SetupStartupData(); err != nil {
		return err
	}

	g, gctx := errgroup.WithContext(ctx)

	// Cron jobs
	g.Go(func() error {
		logger.Log().Info("[main][msg: starting cron jobs]")
		app.CronManager().Start()

		<-gctx.Done()

		logger.Log().Info("[main][msg: stopping cron jobs]")
		cronCtx := app.CronManager().Stop()
		<-cronCtx.Done()
		return nil
	})

	// HTTP server
	g.Go(func() error {
		logger.Log().Infof("[main][msg: http server is starting on port %s]", config.HTTP().Port)

		err := app.HTTPServer().ListenAndServe()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})

	// WebSocket server
	g.Go(func() error {
		logger.Log().Infof("[main][msg: WS server is starting on port %s]", config.Websocket().Port)

		err := app.WebsocketServer().ListenAndServe()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})

	// Consumers
	g.Go(func() error {
		logger.Log().Info("[main][msg: starting consumers]")

		err := app.QueueManager().RunConsumers()
		if err == nil {
			return nil
		}
		return err
	})

	select {
	case <-ctx.Done():
		logger.Log().Info("[main][msg: shutdown requested]")
	case <-gctx.Done():
		logger.Log().Info("[main][msg: background task stopped]")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), _shutdownTimeout)
	defer cancel()
	shutdown(app, shutdownCtx)

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// shutdown performs a graceful shutdown of the application components.
func shutdown(app *application.Application, ctx context.Context) {
	// HTTP
	if err := app.HTTPServer().Shutdown(ctx); err != nil {
		logger.Log().Errorf("[main][err: http server shutdown failed: %s]", err)
	} else {
		logger.Log().Info("[main][msg: http server shutdown gracefully]")
	}

	// Stop accepting WS connections
	if err := app.WebsocketServer().Shutdown(ctx); err != nil {
		logger.Log().Errorf("[main][err: WS server shutdown failed: %s]", err)
	} else {
		logger.Log().Info("[main][msg: WS server shutdown gracefully]")
	}

	if err := tracer.Shutdown(ctx); err != nil {
		logger.Log().Errorf("[main][err: tracer shutdown failed: %s]", err)
	}

	// Stop queue consumers + close queue connections
	if err := app.QueueManager().CloseConsumers(); err != nil {
		logger.Log().Errorf("[main][err: close consumers: %s]", err)
	}
	app.CloseQueueConnections()

	// Close outgoing gRPC connections (clients)
	if core := app.Core(); core != nil {
		if err := core.CloseGRPC(); err != nil {
			logger.Log().Errorf("[main][err: core grpc close failed: %s]", err)
		}
	}
}
