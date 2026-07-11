CREATE TABLE IF NOT EXISTS muted_alerts (
    id         CHAR(26)    NOT NULL,
    server_id  CHAR(26)    NOT NULL,
    kind       VARCHAR(40) NOT NULL,
    created_by VARCHAR(120) NULL,
    created_at DATETIME    NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_muted_server_kind (server_id, kind)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
