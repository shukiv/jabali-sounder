-- 000010_sessions.up.sql
-- Server-side session records for listing + revoking active logins (M3).
CREATE TABLE IF NOT EXISTS sessions (
    id           CHAR(26)     NOT NULL,
    admin_id     CHAR(26)     NOT NULL,
    user_agent   VARCHAR(400) NULL,
    ip           VARCHAR(64)  NULL,
    created_at   DATETIME     NOT NULL,
    last_seen_at DATETIME     NOT NULL,
    expires_at   DATETIME     NOT NULL,
    revoked_at   DATETIME     NULL,
    PRIMARY KEY (id),
    KEY idx_sessions_admin (admin_id),
    KEY idx_sessions_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
