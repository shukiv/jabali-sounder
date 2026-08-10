CREATE TABLE webauthn_credentials (
  id            CHAR(26)       NOT NULL PRIMARY KEY,
  admin_id      CHAR(26)       NOT NULL,
  credential_id VARBINARY(255) NOT NULL,
  data          BLOB           NOT NULL,
  label         VARCHAR(120)   NOT NULL DEFAULT '',
  created_at    DATETIME       NOT NULL,
  last_used_at  DATETIME       NULL,
  UNIQUE KEY idx_wac_cred (credential_id),
  KEY idx_wac_admin (admin_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
