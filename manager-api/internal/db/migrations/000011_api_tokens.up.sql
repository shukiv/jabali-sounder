-- 000011_api_tokens.up.sql
-- Read-only API tokens for external tooling (M4).
CREATE TABLE IF NOT EXISTS api_tokens (
    id           CHAR(26)     NOT NULL,
    name         VARCHAR(200) NOT NULL,
    secret_hash  CHAR(64)     NOT NULL,
    created_by   CHAR(26)     NULL,
    created_at   DATETIME     NOT NULL,
    last_used_at DATETIME     NULL,
    expires_at   DATETIME     NULL,
    revoked_at   DATETIME     NULL,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
