CREATE TABLE IF NOT EXISTS policy_exceptions (
    id         CHAR(26)     NOT NULL,
    server_id  CHAR(26)     NOT NULL,
    check_name VARCHAR(40)  NOT NULL,
    reason     VARCHAR(400) NOT NULL DEFAULT '',
    expires_at DATETIME     NULL,
    created_by VARCHAR(120) NOT NULL DEFAULT '',
    created_at DATETIME     NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_polex_server_check (server_id, check_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
