# Notification Service

## Overview

`notification-service` is the app backend service responsible for delivering
OTP and business-event notifications over e-mail and WebSocket.

Today it is primarily a queue-driven service:

- it consumes shared notification messages from RabbitMQ
- renders localized HTML e-mails from templates under `data/mailer/`
- sends structured WebSocket events to connected clients
- validates WebSocket sessions and enriches some deliveries through
  `user-service`

It also starts two servers:

- a small Gin HTTP server for health and observability middleware
- a separate WebSocket server for real-time delivery

## What the service currently owns

This repository currently owns:

- OTP e-mail delivery
- push/business-event delivery over e-mail and WebSocket
- localized e-mail templates and translation catalogs
- WebSocket room management and fan-out behavior
- queue-consumer retry classification for notification failures

This repository currently does not expose:

- a notification CRUD API
- a gRPC server
- queue publishers
- an SMS sender implementation

## Runtime model

### Startup flow

`cmd/notification-service/main.go` starts the application in this order:

1. load config, logger, and tracer
2. connect to PostgreSQL
3. connect to RabbitMQ
4. build outbound HTTP and gRPC clients
5. initialize the mailer and WebSocket sender
6. build controllers
7. register queue consumers
8. register cron jobs
9. start the HTTP server
10. start the WebSocket server
11. run queue consumers and cron jobs

Graceful shutdown stops the HTTP server, the WebSocket server, tracing, queue
consumers, cron jobs, AMQP connections, and outgoing gRPC connections.

### Cron jobs

| Job | Default schedule | Purpose |
|---|---|---|
| `notification.dedupe_key.cleanup` | `@every 1h` | Deletes expired rows from `notification_dedupe_keys` |

Configure with `CRON_DEDUPE_KEY_CLEANUP_SPEC` and `CRON_DEDUPE_KEY_TTL`.

### Duplicate delivery fence

Push notifications fan out to several channels that succeed or fail
independently, and `Send` reports failure if any of them failed. The push
consumer's retry pipeline then redelivers the whole message — three times by
default — which means a single e-mail outage during a deposit notification used
to produce up to four WebSocket messages for one event. That local retry is the
main source of duplicates.

A second, rarer source is the producer: account-service publishes push
notifications from a transactional outbox, where a publish that succeeded before
its status update failed is republished on the next attempt.

Either way the user cannot tell a duplicate from a genuine second event.

OTP commands are single-address, so a redelivery repeats only the channel that
failed. They carry no key and need none.

`PushMessage.DedupeKey` fences that. When set, the key is reserved in
`notification_dedupe_keys` before delivery and released again if delivery fails,
so a retry after a successful send is dropped while a retry after a failed send
still goes through. When empty — the default, and correct for producers that
publish exactly once — no fencing happens and behaviour is unchanged.

The fence is per channel, not per message: most notifications fan out to several
channels (a successful deposit goes to both WebSocket and e-mail) that succeed or
fail independently. Stored keys are therefore `<dedupe_key>:<channel>`. If the
WebSocket lands and e-mail fails, the producer's retry redelivers e-mail only.

Fencing applies to every send that carries a key, whatever the channel, because
OTP and push both funnel through `NotificationCtrl.Send`. OTP messages carry no
key today: user-service publishes them directly rather than from an outbox, so
there is no republish to fence against.

The reservation is committed on its own, before the delivery it guards, and is
deliberately not wrapped in a database transaction. A transaction spanning
reserve-send-commit would roll the fence back if the process died after the
e-mail went out, letting the retry send it again, and would hold a pooled
connection and the unique-index lock across an SMTP round trip. This differs
from restriction-service, where the equivalent idempotency key does join the
business transaction — there the guarded effect is itself a database write, so
key and effect must land atomically.

`CRON_DEDUPE_KEY_TTL` (default 48h) must exceed the longest producer retry
window; account-service retries an outbox message for roughly four hours before
dead-lettering it.

A crash between reserving the fence and delivering loses that notification. That
is a deliberate trade: for these notifications a duplicate is worse than a rare
miss, and the producer's own dead-letter queue surfaces the failure.

Migrations live under `data/db/migrations/postgres` and are applied with the CLI
under `cmd/commands/migrate`. The production deploy runs that CLI as
`migrate --command=up` in a throwaway container before starting the service, so
a release carrying a migration applies it on its own. The dedupe fence is this
service's first migration, so before that step existed a release could ship a
table the database never received.

## Architecture

```text
cmd/notification-service/app/         composition root and runtime wiring
config/                               environment-driven settings
internal/controllers/                 notification and websocket orchestration
internal/domain/                      message kinds, channels, contracts, errors
internal/infrastructure/mailer/       SMTP/log mailer implementation
internal/infrastructure/websocket/    websocket room and sender implementation
internal/transport/queue/handlers/    RabbitMQ consumer handlers
internal/transport/websocket/         websocket middleware
data/mailer/                          HTML templates and i18n catalogs
docs/                                 generated Swagger artifacts
```

## Delivery flows

### OTP flow

1. RabbitMQ delivers a shared `notification.otp` payload.
2. `internal/transport/queue/handlers/otp.go` decodes it into a
   `domain.SendCommand`.
3. `NotificationCtrl.Send(...)` routes the message to the channel handler.
4. For e-mail, the mailer renders the template and sends or logs the message.

Supported OTP purposes:

- `registrationOtp`
- `passwordResetOtp`
- `twoFactorOtp`

### Push flow

1. RabbitMQ delivers a shared `notification.push` payload.
2. `internal/transport/queue/handlers/push.go` decodes purpose, locale, data,
   and delivery addresses.
3. `NotificationCtrl.Send(...)` fans out delivery to the channels required by
   the message kind.
4. E-mail uses the mailer, while WebSocket uses `WebsocketCtrl`.

Supported push purposes:

- `successfulDeposit`
- `requiredVerificationDeposit`
- `successfulWithdrawal`
- `successfulIncomingTransfer`
- `successfulOutgoingTransfer`

### WebSocket delivery path

WebSocket delivery is slightly unusual and important to understand.

`NotificationCtrl` does not write directly into the room. Instead:

1. `NotificationCtrl.Send(...)` builds a `domain.WsMessage`
2. `WebsocketCtrl.Send(...)` converts it into the wire payload
3. `internal/infrastructure/websocket.Sender` dials this service's own `/ws`
   endpoint using the privileged WebSocket token
4. the WebSocket room receives that message and fans it out to connected
   clients

This loopback design means the WebSocket server is part of the internal delivery
path, not only a client-facing endpoint.

## Shared contracts and purpose mapping

The queue payload source of truth is not this repository alone.

Shared queue message definitions live in:

- `repos/shared/common/services/notification/queue/messages.go`

This repository then maps those shared payloads into local behavior in:

- `internal/domain/notification.go`
- `internal/transport/queue/handlers/otp.go`
- `internal/transport/queue/handlers/push.go`

Do not change queue payload shape locally without checking producers and
consumers in other repositories.

## Retry & DLQ pipeline

OTP and push consumers use the platform retry/DLQ pipeline from
`github.com/whagency/core-common/queue/rabbitmq`. Per-queue topology
(`<queue>.retry` delayed-message exchange + `<queue>.dlq` fanout + parking
queue) is declared on startup in `setupQueueRetryTopologies()`.

Default budgets (env-overridable via `QUEUE_OTP_RETRY_*`, `QUEUE_PUSH_RETRY_*`):

| Queue | MaxRetries | Delays | Alert |
|---|-------------|---|---|
| `otp.dispatch` | 3           | 10s, 30s, 2m | info |
| `push.dispatch` | 3           | 10s, 1m, 5m | info |

Delays are intentionally short. OTP loses business value after ~5 minutes and
push notifications are best-effort; aggressive retry budgets here would only
deliver messages the user no longer needs.

`internal/transport/queue/handlers/retry_policy.go` marks deterministic
payload/domain errors as non-retryable; these go straight to DLQ with
`x-reason=permanent`. This includes common `InvalidRequest` and
`RecordNotFound` errors plus notification-specific codes for invalid payloads,
unsupported channels/kinds, invalid recipients, and profile/channel mismatches.
Transient errors (SMTP outage, user-service gRPC `Unavailable`, WebSocket
loopback hiccup) are retried with backoff and parked once the budget is
exhausted. If a fan-out attempt returns mixed permanent and transient errors,
the whole message remains retryable so transient channels still get their retry
budget. Setting the per-queue `*_MAX_RETRIES=0` disables the pipeline and falls
back to legacy NackRequeue behaviour for emergency rollback.

Non-retryable examples:

- malformed JSON, unexpected message type, missing required payload fields
- unsupported OTP/push purpose or delivery channel
- unsupported notification kind, including WebSocket delivery for
  `successfulOutgoingTransfer`
- invalid `userId`, invalid WebSocket recipient, invalid e-mail recipient
- e-mail fallback profile not found or profile contact type not compatible with
  e-mail delivery

Retryable examples:

- SMTP send failure or timeout
- user-service gRPC unavailable/deadline exceeded/connection failure
- WebSocket loopback dial/write/reconnect failure
- missing mail templates or other deployment/runtime rendering failures
- unknown errors and mixed permanent/transient fan-out errors

### DLQ runbook

Both DLQs are operator-driven; messages parked there are typically stale.

- `otp.dispatch.dlq` — OTP codes are time-bound. The pragmatic action is to
  let the user retry the OTP flow from the producer (`user-service`). Inspect
  parked messages to detect SMTP/user-service incidents and drop expired
  payloads.
- `push.dispatch.dlq` — push notifications are non-critical. Inspect
  `x-last-error` to detect systemic SMTP or WebSocket issues and drop messages
  superseded by later events.

## Channels

### E-mail

E-mail delivery is implemented in `internal/infrastructure/mailer/`.

Current templates:

- `signup_confirm_code.html`
- `two_factor_confirm_code.html`
- `successful_deposit.html`
- `successful_withdrawal.html`
- `successful_outgoing_transfer.html`
- `successful_incoming_transfer.html`

Translations currently exist for:

- `en`
- `uk`

Important behavior:

- if `MAIL_MAILER=log`, the service does not open an SMTP transport and logs the
  rendered e-mail instead
- for some push flows, if e-mail is missing but `userId` is present,
  `NotificationCtrl` fetches the profile from `user-service` and derives contact
  value and locale from there

### WebSocket

The WebSocket server listens on its own port and upgrades requests on `GET /ws`.

Handshake behavior:

- the query parameter named by `WS_SECURITY_TOKEN_KEY` must be present
- if it equals `WS_SECURITY_TOKEN`, the connection is allowed to send messages
  into the room
- otherwise the token is treated as a user session token and is validated
  through `user-service`
- session-authenticated clients are listeners only; unauthorized write attempts
  are ignored

Important correction versus older docs:

- the current handshake does not read a `uid` query parameter
- user identity comes from the validated session, not from a URL argument

Current outbound WebSocket event types:

- `sd` for successful deposit
- `rvd` for required verification deposit
- `sw` for successful withdrawal
- `st` for successful incoming transfer

`successfulOutgoingTransfer` currently has e-mail delivery only and no dedicated
WebSocket payload type.

### SMS

`ChannelSMS` exists in the domain model, but there is no SMS sender in
`NotificationCtrl`.

That means SMS is not a working delivery channel today. Do not document or rely
on SMS as supported behavior unless the implementation is added.

## External dependencies

This service currently depends on:

- RabbitMQ for OTP and push consumers
- `user-service` gRPC for profile lookup and session validation
- SMTP or `MAIL_MAILER=log` for e-mail dispatch
- PostgreSQL because startup always initializes a DB connection
- `common` for logger, tracer, queue wrappers, user-service clients, and shared
  notification contracts

## HTTP surface

The HTTP server is intentionally small.

Current routes in `cmd/notification-service/app/http_server.go`:

- `GET /api/v1/health`

Prometheus Gin instrumentation is enabled through middleware/helpers, but this
repository does not currently register a notification-specific REST API.

Generated Swagger artifacts exist under `docs/`, but the router does not
currently expose a Swagger UI or Swagger JSON endpoint.

## Configuration notes

All settings are loaded from environment variables via `config/`.

The most important groups are:

- application/common settings:
  - `APP_NAME`
  - `APP_ENVIRONMENT`
  - `APP_VERSION`
  - `TIME_ZONE`
  - `SECURITY_TOKEN`
  - `USER_SERVICE_GRPC_URL`
- database settings:
  - `DB_*`
- queue settings:
  - `QUEUE_*`
- mailer settings:
  - `MAIL_*`
  - `DEFAULT_TEMPLATE_DIR`
  - `DEFAULT_TRANSLATIONS_DIR`
  - `DEFAULT_APP_NAME`
  - `DEFAULT_APP_URL`
- websocket settings:
  - `WS_*`

Important configuration caveats:

- `.env.example` is not a full runtime contract; it is only a minimal starter
  file
- `USER_SERVICE_URL` is still required by config and an HTTP user client is
  constructed in `core.go`, but the current hot paths use the gRPC client
- `WS_SECURITY_TOKEN_KEY` defaults to `t`, not `token`

### Security token and database TLS

`SECURITY_TOKEN` authenticates this service's calls to other services' internal
endpoints. This service exposes no token-guarded HTTP endpoints of its own; the
WebSocket transport uses `WS_SECURITY_TOKEN` instead.

The token must not be empty: `config.Init()` fails at startup when it is blank,
because envconfig's `required` tag only rejects an *unset* variable and an empty
token authenticates nothing.

`DB_SSL_MODE` is passed through as the PostgreSQL `sslmode` and accepts
`disable|allow|prefer|require|verify-ca|verify-full`. It defaults to `disable`
for local infrastructure; deployed environments should set at least `require`.
An unrecognised value fails at startup rather than silently downgrading the
connection. The same value is used by `cmd/commands/migrate`.

## Running locally

1. provide the required env vars or extend `.env.example`
2. start PostgreSQL, RabbitMQ, and `user-service`
3. choose a mailer mode:
   - use a local SMTP sandbox, or
   - set `MAIL_MAILER=log` for local development
4. run the service:

```bash
go run ./cmd/notification-service
```

5. connect a client to:

```text
ws://<ws-host>:<ws-port>/ws?<ws-security-token-key>=<session-or-internal-token>
```

6. publish shared OTP or push messages to the configured RabbitMQ exchange and
   routing key

## Tooling and verification

Useful commands:

- `go test ./...`
- `make lint`
- `make vulnerable-check`
- `make swagger-gen`

Current tests cover:

- controller fan-out and aggregated errors
- queue decode and retry-policy classification
- websocket room cleanup behavior

## Development notes

- `cmd/notification-service/app/` is the source of truth for active runtime
  wiring
- `internal/domain/notification.go` is the source of truth for purpose-to-kind
  and kind-to-channel mapping
- `internal/transport/queue/handlers/retry_policy.go` is the source of truth
  for non-retryable notification error classification
- generated Swagger files may lag behind reality; trust route wiring and code
  first
