ALTER TABLE notifications
    ADD COLUMN severity      VARCHAR(20) NOT NULL DEFAULT 'warning',
    ADD COLUMN acked_at      DATETIME    NULL,
    ADD COLUMN acked_by      VARCHAR(120) NULL,
    ADD COLUMN snoozed_until DATETIME    NULL,
    ADD COLUMN escalated_at  DATETIME    NULL;
