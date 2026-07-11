CREATE TABLE IF NOT EXISTS backup_runs (
    id           CHAR(26)     NOT NULL,
    server_id    CHAR(26)     NOT NULL,
    server_name  VARCHAR(200) NULL,
    operation_id VARCHAR(120) NULL,
    status       VARCHAR(20)  NOT NULL,
    message      VARCHAR(400) NULL,
    triggered_by VARCHAR(120) NULL,
    started_at   DATETIME     NOT NULL,
    finished_at  DATETIME     NULL,
    PRIMARY KEY (id),
    KEY idx_backup_server (server_id),
    KEY idx_backup_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
