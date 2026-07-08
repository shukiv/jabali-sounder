-- 000001_servers.up.sql
-- Managed Jabali Panel server enrollment.
CREATE TABLE IF NOT EXISTS servers (
    id                  CHAR(26)     NOT NULL PRIMARY KEY,
    name                VARCHAR(200) NOT NULL,
    base_url            VARCHAR(500) NOT NULL,
    token_id            CHAR(26)     NOT NULL DEFAULT '',
    token_secret_enc    BLOB         NULL,
    scopes              TEXT         NULL,
    version             VARCHAR(50)  NOT NULL DEFAULT '',
    capabilities        TEXT         NULL,
    health_url          VARCHAR(500) NOT NULL DEFAULT '',
    status              ENUM('active','disabled','unreachable') NOT NULL DEFAULT 'active',
    credential_status   ENUM('valid','invalid','unknown')     NOT NULL DEFAULT 'unknown',
    last_heartbeat_at   DATETIME(3)  NULL,
    last_checked_at     DATETIME(3)  NULL,
    created_at          DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at          DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    disabled_at         DATETIME(3)  NULL,
    UNIQUE KEY uk_servers_name (name),
    UNIQUE KEY uk_servers_token (token_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
