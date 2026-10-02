-- +goose Up
CREATE TABLE IF NOT EXISTS users (
  id BYTEA NOT NULL PRIMARY KEY,
  email VARCHAR(255) NOT NULL DEFAULT '',
  nickname VARCHAR(255) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  avatar VARCHAR(255) NOT NULL,
  provider VARCHAR(128) NOT NULL, -- custom (provider)issuer enum
  sub VARCHAR(512) NOT NULL, -- subject in jwt token
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  CONSTRAINT chk_users_id_len CHECK (octet_length(id) = 16),
  CONSTRAINT chk_users_provider_sub_uniq UNIQUE (provider, sub)
);

COMMENT ON TABLE users IS 'user table';
COMMENT ON COLUMN users.id IS 'user id, primary key (ulid 16 bytes)';
COMMENT ON COLUMN users.email IS 'user email';
COMMENT ON COLUMN users.nickname IS 'user nickname';
COMMENT ON COLUMN users.status IS 'user status';
COMMENT ON COLUMN users.avatar IS 'user avatar';
COMMENT ON COLUMN users.provider IS 'user provider';
COMMENT ON COLUMN users.sub IS 'user subject';
COMMENT ON COLUMN users.created_at IS 'user created time (unix ms)';
COMMENT ON COLUMN users.updated_at IS 'user updated time (unix ms)';

-- +goose Down
DROP TABLE IF EXISTS users;
