CREATE TABLE IF NOT EXISTS alert_rules (
    id               CHAR(26)    NOT NULL,
    metric           VARCHAR(40) NOT NULL,
    threshold        DOUBLE      NOT NULL,
    duration_seconds INT         NOT NULL,
    severity         VARCHAR(20) NOT NULL,
    enabled          TINYINT(1)  NOT NULL DEFAULT 1,
    created_at       DATETIME    NOT NULL,
    updated_at       DATETIME    NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_alert_rules_metric (metric)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
