package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

var queueSettings QueueSettings

type QueueSettings struct {
	Host              string        `envconfig:"QUEUE_HOST" required:"true"`
	Port              string        `envconfig:"QUEUE_PORT" required:"true"`
	User              string        `envconfig:"QUEUE_USER" required:"true"`
	Password          string        `envconfig:"QUEUE_PASSWORD" required:"true"`
	ReconnectInterval time.Duration `envconfig:"QUEUE_RECONNECT_INTERVAL" default:"30s"`

	OTPExchangeName    string          `envconfig:"QUEUE_OTP_EXCHANGE_NAME" default:"account.otp"`
	OTPExchangeType    string          `envconfig:"QUEUE_OTP_EXCHANGE_TYPE" default:"direct"`
	OTPRoutingKey      string          `envconfig:"QUEUE_OTP_ROUTING_KEY" default:"otp.dispatch"`
	OTPQueueName       string          `envconfig:"QUEUE_OTP_QUEUE_NAME" default:"otp.dispatch"`
	OTPPrefetchCount   int             `envconfig:"QUEUE_OTP_PREFETCH_COUNT" default:"100"`
	OTPConsumerName    string          `envconfig:"QUEUE_OTP_CONSUMER_NAME" default:"otp_consumer"`
	OTPConsumers       int             `envconfig:"QUEUE_OTP_CONSUMERS" default:"1"`
	OTPRetryMaxRetries int             `envconfig:"QUEUE_OTP_RETRY_MAX_RETRIES" default:"3"`
	OTPRetryDelays     []time.Duration `envconfig:"QUEUE_OTP_RETRY_DELAYS" default:"10s,30s,2m"`

	PushExchangeName    string          `envconfig:"QUEUE_PUSH_EXCHANGE_NAME" default:"account.push"`
	PushExchangeType    string          `envconfig:"QUEUE_PUSH_EXCHANGE_TYPE" default:"direct"`
	PushRoutingKey      string          `envconfig:"QUEUE_PUSH_ROUTING_KEY" default:"push.dispatch"`
	PushQueueName       string          `envconfig:"QUEUE_PUSH_QUEUE_NAME" default:"push.dispatch"`
	PushPrefetchCount   int             `envconfig:"QUEUE_PUSH_PREFETCH_COUNT" default:"100"`
	PushConsumerName    string          `envconfig:"QUEUE_PUSH_CONSUMER_NAME" default:"push_consumer"`
	PushConsumers       int             `envconfig:"QUEUE_PUSH_CONSUMERS" default:"1"`
	PushRetryMaxRetries int             `envconfig:"QUEUE_PUSH_RETRY_MAX_RETRIES" default:"3"`
	PushRetryDelays     []time.Duration `envconfig:"QUEUE_PUSH_RETRY_DELAYS" default:"10s,1m,5m"`
}

// Queue returns the queue settings
func Queue() QueueSettings {
	return queueSettings
}

// loadQueueConfig loads the queue configuration from the environment variables
func loadQueueConfig() error {
	if err := envconfig.Process("", &queueSettings); err != nil {
		return fmt.Errorf("[loadQueueConfig][err: %w]", err)
	}
	return nil
}
