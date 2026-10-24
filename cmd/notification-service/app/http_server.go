package app

import (
	"net/http"

	"notification-service/config"
	"notification-service/tools"

	"github.com/gin-gonic/gin"
	httpCommonHandlers "github.com/whagency/core-common/http/handler"
	httpMiddleware "github.com/whagency/core-common/http/middleware"
	"github.com/whagency/core-common/logger"
)

// setupHTTPServer sets up the server
//
//	@title						Notification Service
//	@version					1.0.0
//	@description				API for Notification Service
//	@description				[Read me]()
//	@termsOfService				http://swagger.io/terms/
//	@BasePath					/notification/
//	@securityDefinitions.apikey	Authorization
//	@in							header
//	@name						Authorization
//	@description				Provide your auth token
func (app *Application) setupHTTPServer() error {
	logger.Log().Info("[Application:setupHTTPServer][msg: setting up server]")

	if config.Commons().IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(
		gin.Recovery(),
		httpMiddleware.GinObservabilityMiddleware(),
		httpMiddleware.CORS(),
	)
	router.NoRoute(httpCommonHandlers.RouteNotFound)

	// Adding Metrics
	tools.LinkToPrometheusToGin(router)

	// API v1
	v1 := router.Group("api/v1")

	v1.GET("/health", httpCommonHandlers.HealthCheck)

	app.httpServer = &http.Server{
		Addr:              ":" + config.HTTP().Port,
		Handler:           router,
		ReadTimeout:       config.HTTP().ReadTimeout,
		ReadHeaderTimeout: config.HTTP().ReadHeaderTimeout,
		WriteTimeout:      config.HTTP().WriteTimeout,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
	}

	return nil
}
