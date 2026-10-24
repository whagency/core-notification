package app

import (
	"net/http"

	"notification-service/config"
	"notification-service/internal/infrastructure/websocket"
	"notification-service/internal/transport/websocket/middlewares"
)

// setupWebsocketServer sets up the WebSocket server.
func (app *Application) setupWebsocketServer() error {
	publicRoom := websocket.NewWsRoom(config.Websocket().PublicChannelName)
	websocketHandler := &websocket.WsHandler{Room: publicRoom}
	websocketHandlerProtected := middlewares.WebsocketLoggingMiddleware(
		middlewares.ValidateUserSession(websocketHandler, app.Core().servicesGRPC.User),
	)

	http.Handle("/ws", websocketHandlerProtected)

	go publicRoom.Run(websocket.MessageWsHandler)

	app.websocketServer = &http.Server{
		Addr:         ":" + config.Websocket().Port,
		Handler:      nil,
		ReadTimeout:  config.Websocket().ReadTimeout,
		WriteTimeout: config.Websocket().WriteTimeout,
		IdleTimeout:  config.Websocket().IdleTimeout,
	}

	return nil
}
