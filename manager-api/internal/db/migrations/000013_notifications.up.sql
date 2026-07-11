-- 000013_notifications.up.sql
-- In-app fleet notifications (SND-18: sustained high-CPU alerts).
CREATE TABLE IF NOT EXISTS notifications (
    id          CHAR(26)     NOT NULL,
    kind        VARCHAR(40)  NOT NULL,
    server_id   CHAR(26)     NULL,
    server_name VARCHAR(200) NULL,
    metric      VARCHAR(40)  NULL,
    value       DOUBLE       NULL,
    threshold   DOUBLE       NULL,
    message     VARCHAR(400) NULL,
    created_at  DATETIME     NOT NULL,
    read_at     DATETIME     NULL,
    resolved_at DATETIME     NULL,
    PRIMARY KEY (id),
    KEY idx_notifications_kind (kind),
    KEY idx_notifications_server (server_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
