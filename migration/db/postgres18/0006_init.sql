-- +goose Up

ALTER TABLE notebooks ADD COLUMN visibility VARCHAR(16) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_notebooks_owner_id ON notebooks (owner_id);

CREATE TABLE IF NOT EXISTS notebook_members (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  notebook_id UUID NOT NULL,
  user_id BYTEA NOT NULL,
  role VARCHAR(16) NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  CONSTRAINT chk_notebook_members_user_id_len CHECK (octet_length(user_id) = 16)
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_notebook_members_notebook_user
  ON notebook_members (notebook_id, user_id);
CREATE INDEX IF NOT EXISTS idx_notebook_members_user_id
  ON notebook_members (user_id);

COMMENT ON TABLE notebook_members IS 'notebook members, explicit grants only';
COMMENT ON COLUMN notebook_members.notebook_id IS 'notebook id';
COMMENT ON COLUMN notebook_members.user_id IS 'member user id (ulid 16 bytes)';
COMMENT ON COLUMN notebook_members.role IS 'member role, validated by application';
COMMENT ON COLUMN notebook_members.created_at IS 'member created time (unix ms)';
COMMENT ON COLUMN notebook_members.updated_at IS 'member updated time (unix ms)';

ALTER TABLE sources RENAME COLUMN owner_id TO creator_id;
ALTER TABLE sources RENAME CONSTRAINT chk_sources_owner_id_len TO chk_sources_creator_id_len;
COMMENT ON COLUMN sources.creator_id IS 'source creator id (audit only)';

ALTER TABLE artifacts RENAME COLUMN user_id TO creator_id;
ALTER TABLE artifacts RENAME CONSTRAINT chk_artifacts_user_id_len TO chk_artifacts_creator_id_len;
COMMENT ON COLUMN artifacts.creator_id IS 'artifact creator id (audit/usage only)';

-- +goose Down
ALTER TABLE artifacts RENAME CONSTRAINT chk_artifacts_creator_id_len TO chk_artifacts_user_id_len;
ALTER TABLE artifacts RENAME COLUMN creator_id TO user_id;
COMMENT ON COLUMN artifacts.user_id IS 'artifact user id (ulid 16 bytes)';

ALTER TABLE sources RENAME CONSTRAINT chk_sources_creator_id_len TO chk_sources_owner_id_len;
ALTER TABLE sources RENAME COLUMN creator_id TO owner_id;
COMMENT ON COLUMN sources.owner_id IS 'source owner id (ulid 16 bytes)';

DROP TABLE IF EXISTS notebook_members;
DROP INDEX IF EXISTS idx_notebooks_owner_id;
ALTER TABLE notebooks DROP COLUMN IF EXISTS visibility;
