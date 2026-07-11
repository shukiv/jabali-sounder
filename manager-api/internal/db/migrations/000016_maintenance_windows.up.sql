CREATE TABLE IF NOT EXISTS maintenance_windows (
    id          CHAR(26)     NOT NULL,
    scope_type  VARCHAR(20)  NOT NULL,
    scope_value VARCHAR(200) NULL,
    starts_at   DATETIME     NOT NULL,
    ends_at     DATETIME     NOT NULL,
    reason      VARCHAR(400) NULL,
    created_by  VARCHAR(120) NULL,
    created_at  DATETIME     NOT NULL,
    PRIMARY KEY (id),
    KEY idx_maint_window_time (starts_at, ends_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
