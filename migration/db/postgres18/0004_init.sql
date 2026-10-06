-- +goose Up
CREATE TABLE IF NOT EXISTS artifact_style_previews (
  id BIGSERIAL PRIMARY KEY,
  store_key VARCHAR(255) NOT NULL DEFAULT '',
  identifier VARCHAR(64) UNIQUE NOT NULL DEFAULT '', 
  md5sum BYTEA,
  original_filename VARCHAR(255) DEFAULT '',
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE artifact_style_previews IS 'preview images store key for variables artifacts';
COMMENT ON COLUMN artifact_style_previews.id IS 'primary key';
COMMENT ON COLUMN artifact_style_previews.store_key IS 'store key of preview asset';
COMMENT ON COLUMN artifact_style_previews.identifier IS 'unique identifier';
COMMENT ON COLUMN artifact_style_previews.md5sum IS 'md5 sum of the stored preview asset';
COMMENT ON COLUMN artifact_style_previews.original_filename IS 'the original filename of preview asset';
COMMENT ON COLUMN artifact_style_previews.created_at IS 'create time (unix ms)';
COMMENT ON COLUMN artifact_style_previews.updated_at IS 'update time (unix ms)';

-- +goose Down
DROP TABLE IF EXISTS artifact_style_previews;
