-- +goose Up
ALTER TABLE notebooks ADD COLUMN cover         VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE notebooks ADD COLUMN pending_cover VARCHAR(255) NOT NULL DEFAULT '';

COMMENT ON COLUMN notebooks.cover         IS 'active cover, encoded store key';
COMMENT ON COLUMN notebooks.pending_cover IS 'in-flight cover key pending confirm, encoded store key';

-- +goose Down
ALTER TABLE notebooks DROP COLUMN IF EXISTS pending_cover;
ALTER TABLE notebooks DROP COLUMN IF EXISTS cover;
