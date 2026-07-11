CREATE TABLE IF NOT EXISTS audit_logs (
    id          CHAR(26)     NOT NULL,
    event       VARCHAR(60)  NOT NULL,
    actor       VARCHAR(120) NULL,
    actor_id    VARCHAR(40)  NULL,
    server_id   CHAR(26)     NULL,
    server_name VARCHAR(200) NULL,
    source_ip   VARCHAR(64)  NULL,
    request_id  VARCHAR(64)  NULL,
    created_at  DATETIME     NOT NULL,
    PRIMARY KEY (id),
    KEY idx_audit_event (event),
    KEY idx_audit_actor (actor),
    KEY idx_audit_server (server_id),
    KEY idx_audit_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
