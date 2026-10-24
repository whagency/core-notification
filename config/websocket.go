package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

var websocketSettings WebsocketSettings

type WebsocketSettings struct {
	Host              string        `envconfig:"WS_HOST" default:"127.0.0.1"`
	Port              string        `envconfig:"WS_PORT" default:"8100"`
	Scheme            string        `envconfig:"WS_SCHEME" default:"ws"`
	PublicChannelName string        `envconfig:"WS_PUBLIC_CHANNEL_NAME" default:"public_channel"`
	SocketBufferSize  int           `envconfig:"WS_SOCKET_BUFFER_SIZE" default:"4096"`
	MessageBufferSize int           `envconfig:"WS_MESSAGE_BUFFER_SIZE" default:"4096"`
	SecurityToken     string        `envconfig:"WS_SECURITY_TOKEN" required:"true"`
	SecurityTokenKey  string        `envconfig:"WS_SECURITY_TOKEN_KEY" default:"t"`
	ReadTimeout       time.Duration `envconfig:"WS_SERVER_READ_TIMEOUT" default:"10s"`
	WriteTimeout      time.Duration `envconfig:"WS_SERVER_WRITE_TIMEOUT" default:"10s"`
	IdleTimeout       time.Duration `envconfig:"WS_SERVER_IDLE_TIMEOUT" default:"60s"`
}

// Websocket returns the Websocket settings
func Websocket() WebsocketSettings {
	return websocketSettings
}

// loadWebsocketConfig loads the Websocket settings from the environment variables
func loadWebsocketConfig() error {
	if err := envconfig.Process("", &websocketSettings); err != nil {
		return fmt.Errorf("[loadWebsocketConfig][err: %v]", err)
	}

	return nil
}
