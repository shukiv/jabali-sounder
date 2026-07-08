-- 000002_heartbeats.up.sql
CREATE TABLE IF NOT EXISTS heartbeats (
    id          CHAR(26)     NOT NULL PRIMARY KEY,
    server_id   CHAR(26)     NOT NULL,
    healthy     TINYINT(1)   NOT NULL DEFAULT 0,
    version     VARCHAR(50)  NOT NULL DEFAULT '',
    details     TEXT         NULL,
    checked_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_heartbeats_server_checked (server_id, checked_at),
    CONSTRAINT fk_heartbeats_server FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
