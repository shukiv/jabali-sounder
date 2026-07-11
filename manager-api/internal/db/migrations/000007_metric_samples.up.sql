-- 000007_metric_samples.up.sql
-- Compact resource-usage time series sampled by the health poller.
CREATE TABLE IF NOT EXISTS metric_samples (
    id           CHAR(26)     NOT NULL,
    server_id    CHAR(26)     NOT NULL,
    cpu_percent  DOUBLE       NULL,
    ram_percent  DOUBLE       NULL,
    disk_percent DOUBLE       NULL,
    load1        DOUBLE       NULL,
    sampled_at   DATETIME     NOT NULL,
    PRIMARY KEY (id),
    KEY idx_metric_samples_server (server_id),
    KEY idx_metric_samples_time (sampled_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
