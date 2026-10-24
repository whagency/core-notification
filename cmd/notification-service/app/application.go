package app

import (
	"fmt"
	"net/http"

	"notification-service/internal/domain"
	"notification-service/tools"

	"github.com/imroc/req/v3"
	"github.com/wagslane/go-rabbitmq"
	"github.com/whagency/core-common/queue"
	"github.com/wneessen/go-mail"
	"gorm.io/gorm"
)

type Application struct {
	websocketServer  *http.Server
	httpServer       *http.Server
	httpClient       *req.Client
	db               *gorm.DB
	queueConnections map[queue.ConnectionType]*rabbitmq.Conn
	mailerClient     *mail.Client

	queueManager  domain.QueueManager
	mailerManager domain.MailerManager
	cronManager   domain.CronManager

	// Core
	core *Core
}

// NewApplication creates a new instance of Application.
func NewApplication() *Application {
	return &Application{
		queueConnections: make(map[queue.ConnectionType]*rabbitmq.Conn),
	}
}

// SetupDependencies initializes the dependencies for the application.
func (app *Application) SetupDependencies() error {
	if err := app.setupDB(); err != nil {
		return fmt.Errorf("[Application:SetupDependencies]%w", err)
	}

	if err := app.setupQueueConnections(); err != nil {
		return fmt.Errorf("[Application:SetupDependencies]%w", err)
	}

	if err := app.setupQueueRetryTopologies(); err != nil {
		return fmt.Errorf("[Application:SetupDependencies]%w", err)
	}

	if err := app.setupHTTPClient(); err != nil {
		return fmt.Errorf("[Application:SetupDependencies]%w", err)
	}

	if err := app.setupMailerClient(); err != nil {
		return fmt.Errorf("[Application:SetupDependencies]%w", err)
	}

	return nil
}

// SetupStartupData initializes any necessary startup data for the application.
func (app *Application) SetupStartupData() error {
	tools.RegisterMetrics()

	if err := app.buildCore(); err != nil {
		return fmt.Errorf("[Application:SetupStartupData]%w", err)
	}

	if err := app.setupQueueManager(); err != nil {
		return fmt.Errorf("[Application:SetupStartupData]%w", err)
	}

	if err := app.setupWebsocketServer(); err != nil {
		return fmt.Errorf("[Application:SetupStartupData]%w", err)
	}

	if err := app.setupHTTPServer(); err != nil {
		return fmt.Errorf("[Application:SetupStartupData]%w", err)
	}

	if err := app.setupCronManager(); err != nil {
		return fmt.Errorf("[Application:SetupStartupData]%w", err)
	}

	return nil
}

// CronManager returns the cron manager for the application.
func (app *Application) CronManager() domain.CronManager {
	return app.cronManager
}

// WebsocketServer returns the WS server for the application.
func (app *Application) WebsocketServer() *http.Server {
	return app.websocketServer
}

// HTTPServer returns the HTTP server for the application.
func (app *Application) HTTPServer() *http.Server {
	return app.httpServer
}

// DB returns the database connection for the application.
func (app *Application) DB() *gorm.DB {
	return app.db
}

// Core returns the core components of the application.
func (app *Application) Core() *Core {
	return app.core
}

// QueueManager returns the queue manager for the application.
func (app *Application) QueueManager() domain.QueueManager {
	return app.queueManager
}
