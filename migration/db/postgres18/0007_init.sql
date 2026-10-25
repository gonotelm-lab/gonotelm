-- +goose Up

ALTER TABLE notebooks ADD COLUMN flags INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN notebooks.flags IS 'notebook bit flags';
COMMENT ON COLUMN notebooks.description IS 'owner custom description';

-- +goose Down

ALTER TABLE notebooks DROP COLUMN IF EXISTS flags;
COMMENT ON COLUMN notebooks.description IS 'notebook description';
