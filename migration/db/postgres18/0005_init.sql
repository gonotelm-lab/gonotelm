-- +goose Up

-- ── account ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users_v2 (
  id                BYTEA        NOT NULL PRIMARY KEY,
  email             VARCHAR(255) NOT NULL,
  email_verified_at BIGINT       NOT NULL DEFAULT 0,
  nickname          VARCHAR(255) NOT NULL,
  status            VARCHAR(16)  NOT NULL DEFAULT 'active',
  avatar            VARCHAR(255) NOT NULL DEFAULT '',
  password_hash     VARCHAR(255) NOT NULL DEFAULT '',
  created_at        BIGINT       NOT NULL DEFAULT 0,
  updated_at        BIGINT       NOT NULL DEFAULT 0,
  CONSTRAINT uq_users_v2_email        UNIQUE (email),
  CONSTRAINT chk_users_v2_id_len      CHECK (octet_length(id) = 16),
  CONSTRAINT chk_users_v2_email_shape CHECK (email ~ '^[^@[:space:]]+@[^@[:space:]]+$')
);

COMMENT ON TABLE  users_v2                   IS 'account, one verified email, one optional password, N sign-in identities';
COMMENT ON COLUMN users_v2.id                IS 'account id, primary key (ulid 16 bytes)';
COMMENT ON COLUMN users_v2.email             IS 'lower-cased email, normalized by application code';
COMMENT ON COLUMN users_v2.email_verified_at IS 'unix ms email ownership was proven, audit only';
COMMENT ON COLUMN users_v2.nickname          IS 'display nickname';
COMMENT ON COLUMN users_v2.status            IS 'active | banned';
COMMENT ON COLUMN users_v2.avatar            IS 'encoded store key of the avatar object';
COMMENT ON COLUMN users_v2.password_hash     IS 'bcrypt hash, empty means no password';
COMMENT ON COLUMN users_v2.created_at        IS 'create time (unix ms)';
COMMENT ON COLUMN users_v2.updated_at        IS 'update time (unix ms)';

-- ── federated sign-in identity ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS user_identities_v2 (
  id                      BYTEA        NOT NULL PRIMARY KEY,
  user_id                 BYTEA        NOT NULL,
  provider                VARCHAR(64)  NOT NULL,
  subject                 VARCHAR(512) NOT NULL,
  provider_email          VARCHAR(255) NOT NULL DEFAULT '',
  provider_email_verified BOOLEAN      NOT NULL DEFAULT false,
  last_login_at           BIGINT       NOT NULL DEFAULT 0,
  created_at              BIGINT       NOT NULL DEFAULT 0,
  updated_at              BIGINT       NOT NULL DEFAULT 0,
  CONSTRAINT uq_user_identities_v2_provider_subject UNIQUE (provider, subject),
  CONSTRAINT chk_user_identities_v2_id_len   CHECK (octet_length(id) = 16),
  CONSTRAINT chk_user_identities_v2_user_id  CHECK (octet_length(user_id) = 16),
  CONSTRAINT chk_user_identities_v2_provider CHECK (provider <> ''),
  CONSTRAINT chk_user_identities_v2_subject  CHECK (subject <> '')
);

CREATE INDEX IF NOT EXISTS idx_user_identities_v2_user_id ON user_identities_v2 (user_id);

-- 一个账号在每个 provider 上最多一个身份
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_identities_v2_user_provider ON user_identities_v2 (user_id, provider);

COMMENT ON TABLE  user_identities_v2                         IS 'federated sign-in identity; unique per provider+subject, at most one per account+provider';
COMMENT ON COLUMN user_identities_v2.id                      IS 'identity id, primary key (uuid 16 bytes)';
COMMENT ON COLUMN user_identities_v2.user_id                 IS 'owner account id, no foreign key, cleaned up by application';
COMMENT ON COLUMN user_identities_v2.provider                IS 'github | google | ...';
COMMENT ON COLUMN user_identities_v2.subject                 IS 'provider side stable user id (oidc sub)';
COMMENT ON COLUMN user_identities_v2.provider_email          IS 'email reported by provider, audit snapshot only';
COMMENT ON COLUMN user_identities_v2.provider_email_verified IS 'verified flag reported by provider at link time';
COMMENT ON COLUMN user_identities_v2.last_login_at           IS 'last successful login through this identity (unix ms)';

-- ── deprecated old table ───────────────────────────────────────────────────
COMMENT ON TABLE users IS 'DEPRECATED: replaced by users_v2 / user_identities_v2';

-- +goose Down
DROP TABLE IF EXISTS user_identities_v2;
DROP TABLE IF EXISTS users_v2;
