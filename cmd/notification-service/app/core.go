package app

import (
	"fmt"
	"io"
	"net/url"

	"notification-service/config"
	"notification-service/internal/controllers"
	"notification-service/internal/domain"
	"notification-service/internal/infrastructure/db/postgres"
	"notification-service/internal/infrastructure/mailer"
	websocketSender "notification-service/internal/infrastructure/websocket"
	"notification-service/tools"

	"github.com/imroc/req/v3"
	userGrpcService "github.com/whagency/core-common/services/user/grpc"
	userHttpService "github.com/whagency/core-common/services/user/http"
	"github.com/wneessen/go-mail"
	"gorm.io/gorm"
)

type Core struct {
	NotificationCtrl domain.NotificationController
	WebsocketCtrl    domain.WebsocketController

	dbRepositories
	notificationSenders

	servicesHTTP
	servicesGRPC
}

type servicesHTTP struct {
	User domain.UserService
}

type servicesGRPC struct {
	User    domain.UserServiceGRPC
	closers []io.Closer
}

// registerCloser registers an io.Closer to be closed when Core is closed.
func (s *servicesGRPC) registerCloser(c io.Closer) {
	s.closers = append(s.closers, c)
}

// Close closes all registered io.Closers in reverse order.
func (s *servicesGRPC) Close() error {
	var firstErr error
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// CloseGRPC closes outgoing gRPC client connections owned by Core.
func (c *Core) CloseGRPC() error {
	return c.servicesGRPC.Close()
}

type dbRepositories struct {
	DedupeKeys domain.DedupeKeyRepository
}

type notificationSenders struct {
	Mailer    domain.MailerManager
	Websocket domain.WebsocketManager
}

// buildCore initializes the core components of the application.
func (app *Application) buildCore() error {
	core := &Core{}

	// Initialize DB Repositories
	core.initDBRepositories(app.db)

	// Initialize Notification senders
	if err := core.initNotificationSenders(app.mailerClient); err != nil {
		return fmt.Errorf("[buildCore]%w", err)
	}

	// Initialize Services
	if err := core.initServices(app.httpClient); err != nil {
		return fmt.Errorf("[buildCore]%w", err)
	}

	// Initialize Controllers
	if err := core.initControllers(); err != nil {
		return fmt.Errorf("[buildCore]%w", err)
	}

	app.core = core

	return nil
}

// initDBRepositories initializes the database repositories for the core components.
func (c *Core) initDBRepositories(db *gorm.DB) {
	c.dbRepositories.DedupeKeys = postgres.NewDedupeKeyRepository(db)
}

// initServices initializes the services for the core components.
func (c *Core) initServices(httpClient *req.Client) error {
	// HTTP
	c.servicesHTTP.User = userHttpService.NewUserService(httpClient, config.Commons().UserServiceURL, config.Commons().SecurityToken)

	// gRPC
	c.servicesGRPC.User = userGrpcService.NewUserService(
		config.Commons().UserServiceGrpcURL,
		userGrpcService.WithInsecureTransport(),
		userGrpcService.WithTimeoutExtCall(config.Commons().GrpcTimeout),
	)
	c.servicesGRPC.registerCloser(c.servicesGRPC.User)

	return nil
}

// initNotificationSenders initializes the notification senders for the core components.
func (c *Core) initNotificationSenders(mailerClient *mail.Client) error {
	mailerFrom := domain.MailMessageFrom{
		FromName:    config.Mailer().MailFromName,
		FromAddress: config.Mailer().MailFromAddress,
	}
	translator, errTranslator := tools.NewTranslator(config.Mailer().DefaultTranslationsDir, "en")
	if errTranslator != nil {
		return fmt.Errorf("[initNotificationSenders]%w", errTranslator)
	}
	c.notificationSenders.Mailer = mailer.NewSender(
		mailerClient,
		mailerFrom,
		translator,
		config.Mailer().SendTimeout,
		config.Mailer().DefaultTemplatesDir,
		config.Mailer().DefaultAppName,
		config.Mailer().DefaultAppUrl,
	)

	websocketConnectionUrl := url.URL{Scheme: config.Websocket().Scheme, Host: config.Websocket().Host + ":" + config.Websocket().Port, Path: "/ws"}
	q := websocketConnectionUrl.Query()
	q.Set(config.Websocket().SecurityTokenKey, config.Websocket().SecurityToken)
	websocketConnectionUrl.RawQuery = q.Encode()
	c.notificationSenders.Websocket = websocketSender.NewSender(websocketConnectionUrl)

	return nil
}

// initControllers initializes the controllers for the core components.
func (c *Core) initControllers() error {
	c.WebsocketCtrl = controllers.NewWebsocketController(
		c.notificationSenders.Websocket,
	)

	c.NotificationCtrl = controllers.NewNotificationController(
		c.notificationSenders.Mailer,
		c.WebsocketCtrl,
		c.servicesGRPC.User,
		c.dbRepositories.DedupeKeys,
		config.Cron().DedupeKeyTTL,
	)

	return nil
}
