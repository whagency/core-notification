package app

import (
	"testing"
	"time"

	"notification-service/config"

	"github.com/whagency/core-common/queue/rabbitmq"
)

func TestRetryConfigForConsumer(t *testing.T) {
	t.Run("disabled when max retries is zero", func(t *testing.T) {
		policy, retryExchange, dlqExchange := retryConfigForConsumer(retryConfig{
			queueName:  "otp.dispatch",
			maxRetries: 0,
			delays:     []time.Duration{time.Second},
		})

		if policy != nil {
			t.Fatalf("expected nil policy, got %#v", policy)
		}
		if retryExchange != "" || dlqExchange != "" {
			t.Fatalf("expected empty exchanges, got retry=%q dlq=%q", retryExchange, dlqExchange)
		}
	})

	t.Run("enabled policy uses max retries and default exchange names", func(t *testing.T) {
		delays := []time.Duration{10 * time.Second, time.Minute}
		policy, retryExchange, dlqExchange := retryConfigForConsumer(retryConfig{
			queueName:  "push.dispatch",
			maxRetries: 2,
			delays:     delays,
		})

		if policy == nil {
			t.Fatalf("expected policy")
		}
		if policy.MaxRetries != 2 {
			t.Fatalf("MaxRetries = %d; want 2", policy.MaxRetries)
		}
		if len(policy.Delays) != len(delays) {
			t.Fatalf("delays len = %d; want %d", len(policy.Delays), len(delays))
		}
		if retryExchange != rabbitmq.DefaultRetryExchangeName("push.dispatch") {
			t.Fatalf("retry exchange = %q", retryExchange)
		}
		if dlqExchange != rabbitmq.DefaultDLQExchangeName("push.dispatch") {
			t.Fatalf("dlq exchange = %q", dlqExchange)
		}
	})
}

func TestRetryTopologiesForQueueSettings(t *testing.T) {
	base := config.QueueSettings{
		OTPQueueName:    "otp.dispatch",
		OTPRoutingKey:   "otp.dispatch",
		PushQueueName:   "push.dispatch",
		PushRoutingKey:  "push.dispatch",
		OTPRetryDelays:  []time.Duration{time.Second},
		PushRetryDelays: []time.Duration{time.Second},
	}

	t.Run("builds both topologies when both retry budgets are enabled", func(t *testing.T) {
		q := base
		q.OTPRetryMaxRetries = 1
		q.PushRetryMaxRetries = 1

		topologies := retryTopologiesForQueueSettings(q)
		if len(topologies) != 2 {
			t.Fatalf("topologies len = %d; want 2", len(topologies))
		}
		assertTopology(t, topologies[0], "otp.dispatch")
		assertTopology(t, topologies[1], "push.dispatch")
	})

	t.Run("skips disabled queues", func(t *testing.T) {
		q := base
		q.OTPRetryMaxRetries = 0
		q.PushRetryMaxRetries = 1

		topologies := retryTopologiesForQueueSettings(q)
		if len(topologies) != 1 {
			t.Fatalf("topologies len = %d; want 1", len(topologies))
		}
		assertTopology(t, topologies[0], "push.dispatch")
	})

	t.Run("returns empty slice when all retry budgets are disabled", func(t *testing.T) {
		topologies := retryTopologiesForQueueSettings(base)
		if len(topologies) != 0 {
			t.Fatalf("topologies len = %d; want 0", len(topologies))
		}
	})
}

func assertTopology(t *testing.T, got rabbitmq.RetryTopology, queueName string) {
	t.Helper()

	if got.QueueName != queueName {
		t.Fatalf("QueueName = %q; want %q", got.QueueName, queueName)
	}
	if got.RoutingKey != queueName {
		t.Fatalf("RoutingKey = %q; want %q", got.RoutingKey, queueName)
	}
	if got.RetryExchange != rabbitmq.DefaultRetryExchangeName(queueName) {
		t.Fatalf("RetryExchange = %q", got.RetryExchange)
	}
	if got.DLQExchange != rabbitmq.DefaultDLQExchangeName(queueName) {
		t.Fatalf("DLQExchange = %q", got.DLQExchange)
	}
	if got.DLQQueueName != rabbitmq.DefaultDLQQueueName(queueName) {
		t.Fatalf("DLQQueueName = %q", got.DLQQueueName)
	}
}
