ALTER TABLE api_tokens
    ADD COLUMN rate_limit_per_min INT NOT NULL DEFAULT 0;
