CREATE TABLE IF NOT EXISTS notification_dedupe_keys (
    dedupe_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ  NOT NULL,
    CONSTRAINT pk_notification_dedupe_keys PRIMARY KEY (dedupe_key)
);

COMMENT ON TABLE notification_dedupe_keys IS 'Idempotency fence for notification delivery; prevents a producer retry from reaching the user twice';
COMMENT ON COLUMN notification_dedupe_keys.dedupe_key IS 'Stable identity supplied by the producer, e.g. the originating outbox row dedupe_key';
COMMENT ON COLUMN notification_dedupe_keys.expires_at IS 'Fence lifetime; must exceed the longest producer retry window';

CREATE INDEX IF NOT EXISTS notification_dedupe_keys_expires_at_idx
    ON notification_dedupe_keys (expires_at);
