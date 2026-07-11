CREATE TABLE IF NOT EXISTS alert_channels (
    id           CHAR(26)     NOT NULL,
    name         VARCHAR(120) NOT NULL,
    type         VARCHAR(20)  NOT NULL,
    config_enc   BLOB         NULL,
    min_severity VARCHAR(20)  NOT NULL,
    enabled      TINYINT(1)   NOT NULL DEFAULT 1,
    created_at   DATETIME     NOT NULL,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
