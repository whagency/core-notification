package app

import (
	"fmt"
	"time"

	"notification-service/config"
	"notification-service/internal/domain"
	queueHandlers "notification-service/internal/transport/queue/handlers"

	wagsrmq "github.com/wagslane/go-rabbitmq"
	"github.com/whagency/core-common/logger"
	"github.com/whagency/core-common/queue"
	"github.com/whagency/core-common/queue/rabbitmq"
)

// retryConfig groups the retry-policy parameters needed to enable the
// platform retry/DLQ pipeline on a consumer.
type retryConfig struct {
	queueName  string
	maxRetries int
	delays     []time.Duration
}

// retryConfigForConsumer returns the ConsumerConfig fields that enable the
// retry/DLQ pipeline. When maxRetries <= 0 the pipeline is disabled and the
// consumer falls back to legacy NackRequeue behaviour.
func retryConfigForConsumer(rc retryConfig) (*rabbitmq.RetryPolicy, string, string) {
	if rc.maxRetries <= 0 {
		return nil, "", ""
	}
	policy := &rabbitmq.RetryPolicy{
		MaxRetries: rc.maxRetries,
		Delays:     rc.delays,
	}
	return policy,
		rabbitmq.DefaultRetryExchangeName(rc.queueName),
		rabbitmq.DefaultDLQExchangeName(rc.queueName)
}

// retryTopologiesForQueueSettings returns the retry+DLQ topologies for the
// given queue settings.
func retryTopologiesForQueueSettings(q config.QueueSettings) []rabbitmq.RetryTopology {
	topologies := make([]rabbitmq.RetryTopology, 0, 2)
	if q.OTPRetryMaxRetries > 0 {
		topologies = append(topologies, rabbitmq.RetryTopology{
			QueueName:     q.OTPQueueName,
			RoutingKey:    q.OTPRoutingKey,
			RetryExchange: rabbitmq.DefaultRetryExchangeName(q.OTPQueueName),
			DLQExchange:   rabbitmq.DefaultDLQExchangeName(q.OTPQueueName),
			DLQQueueName:  rabbitmq.DefaultDLQQueueName(q.OTPQueueName),
		})
	}
	if q.PushRetryMaxRetries > 0 {
		topologies = append(topologies, rabbitmq.RetryTopology{
			QueueName:     q.PushQueueName,
			RoutingKey:    q.PushRoutingKey,
			RetryExchange: rabbitmq.DefaultRetryExchangeName(q.PushQueueName),
			DLQExchange:   rabbitmq.DefaultDLQExchangeName(q.PushQueueName),
			DLQQueueName:  rabbitmq.DefaultDLQQueueName(q.PushQueueName),
		})
	}
	return topologies
}

// setupQueueRetryTopologies declares per-queue retry+DLQ topology for every
// consumer that opts into the retry pipeline. Idempotent and safe to call on
// every service start.
func (app *Application) setupQueueRetryTopologies() error {
	amqpURL := rabbitmq.BuildAMQPURL(queue.ConnectionConfig{
		Host:     config.Queue().Host,
		Port:     config.Queue().Port,
		User:     config.Queue().User,
		Password: config.Queue().Password,
	})

	topologies := retryTopologiesForQueueSettings(config.Queue())
	if len(topologies) == 0 {
		logger.Log().Info("[Application:setupQueueRetryTopologies][msg: retry topology disabled]")
		return nil
	}

	for _, t := range topologies {
		if err := rabbitmq.DeclareRetryTopology(amqpURL, t); err != nil {
			return fmt.Errorf("[setupQueueRetryTopologies][queue:%s][err:%w]", t.QueueName, err)
		}
		logger.Log().Infof("[Application:setupQueueRetryTopologies][queue:%s][retry:%s][dlq:%s]",
			t.QueueName, t.RetryExchange, t.DLQExchange)
	}

	return nil
}

// CloseQueueConnections closes the queue connections.
func (app *Application) CloseQueueConnections() {
	logger.Log().Info("[Application:CloseQueueConnections][msg: closing queue connections]")

	for _, conn := range app.queueConnections {
		if err := conn.Close(); err != nil {
			logger.Log().Errorf("[Application:CloseQueueConnections]%s", err)
		}
	}
}

// setupQueueConnections sets up the queue connections.
func (app *Application) setupQueueConnections() error {
	// Add the default queue connection
	if err := app.addNewQueueConnection(domain.DefaultQueueConnectionType, queue.ConnectionConfig{
		Host:     config.Queue().Host,
		Port:     config.Queue().Port,
		User:     config.Queue().User,
		Password: config.Queue().Password,
	}); err != nil {
		return fmt.Errorf("[app:setupQueue]%w", err)
	}

	return nil
}

// addNewQueueConnection adds a new queue connection to the application.
func (app *Application) addNewQueueConnection(queueType queue.ConnectionType, queueConnectionConfig queue.ConnectionConfig) error {
	logger.Log().Infof("[Application:addNewQueueConnection][queueType: %d][amqp://%s:hidden@%s:%s/%s]", queueType, queueConnectionConfig.User, queueConnectionConfig.Host, queueConnectionConfig.Port, queueConnectionConfig.VHost)

	conn, err := wagsrmq.NewConn(
		fmt.Sprintf("amqp://%s:%s@%s:%s/%s", queueConnectionConfig.User, queueConnectionConfig.Password, queueConnectionConfig.Host, queueConnectionConfig.Port, queueConnectionConfig.VHost),
		wagsrmq.WithConnectionOptionsLogging,
		wagsrmq.WithConnectionOptionsLogger(rabbitmq.QueueLogger{}),
		wagsrmq.WithConnectionOptionsReconnectInterval(config.Queue().ReconnectInterval),
	)
	if err != nil {
		return fmt.Errorf("[addNewQueueConnection][err: %w]", err)
	}

	app.queueConnections[queueType] = conn

	return nil
}

// setupQueueManager sets up the queue manager for the application.
func (app *Application) setupQueueManager() error {
	// Define queue manager
	queueManager := queue.NewManager()

	otpRetry, otpRetryEx, otpDLQEx := retryConfigForConsumer(retryConfig{
		queueName:  config.Queue().OTPQueueName,
		maxRetries: config.Queue().OTPRetryMaxRetries,
		delays:     config.Queue().OTPRetryDelays,
	})
	pushRetry, pushRetryEx, pushDLQEx := retryConfigForConsumer(retryConfig{
		queueName:  config.Queue().PushQueueName,
		maxRetries: config.Queue().PushRetryMaxRetries,
		delays:     config.Queue().PushRetryDelays,
	})

	// Define consumers
	otpConsumer, err := rabbitmq.NewConsumer(
		app.queueConnections[domain.DefaultQueueConnectionType],
		queueHandlers.NewOTPHandler(app.core.NotificationCtrl),
		rabbitmq.ConsumerConfig{
			RoutingKey:     config.Queue().OTPRoutingKey,
			QueueName:      config.Queue().OTPQueueName,
			ExchangeName:   config.Queue().OTPExchangeName,
			ExchangeType:   config.Queue().OTPExchangeType,
			ConsumerName:   config.Queue().OTPConsumerName,
			ConsumerNumber: config.Queue().OTPConsumers,
			PrefetchCount:  config.Queue().OTPPrefetchCount,
			Retry:          otpRetry,
			RetryExchange:  otpRetryEx,
			DLQExchange:    otpDLQEx,
		})
	if err != nil {
		return fmt.Errorf("[setupQueueManager]%w", err)
	}

	pushConsumer, err := rabbitmq.NewConsumer(
		app.queueConnections[domain.DefaultQueueConnectionType],
		queueHandlers.NewPushHandler(app.core.NotificationCtrl),
		rabbitmq.ConsumerConfig{
			RoutingKey:     config.Queue().PushRoutingKey,
			QueueName:      config.Queue().PushQueueName,
			ExchangeName:   config.Queue().PushExchangeName,
			ExchangeType:   config.Queue().PushExchangeType,
			ConsumerName:   config.Queue().PushConsumerName,
			ConsumerNumber: config.Queue().PushConsumers,
			PrefetchCount:  config.Queue().PushPrefetchCount,
			Retry:          pushRetry,
			RetryExchange:  pushRetryEx,
			DLQExchange:    pushDLQEx,
		})
	if err != nil {
		return fmt.Errorf("[setupQueueManager]%w", err)
	}

	// Add consumers to the manager
	if err = queueManager.AddConsumer(otpConsumer); err != nil {
		return fmt.Errorf("[setupQueueManager]%w", err)
	}

	if err = queueManager.AddConsumer(pushConsumer); err != nil {
		return fmt.Errorf("[setupQueueManager]%w", err)
	}

	app.queueManager = queueManager

	return nil
}
