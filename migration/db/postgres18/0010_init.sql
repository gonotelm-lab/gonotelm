-- +goose Up
CREATE TABLE favourites (
    id          UUID   PRIMARY KEY DEFAULT uuidv7(),
    user_id     BYTEA  NOT NULL,
    name        TEXT   NOT NULL,
    created_at  BIGINT NOT NULL DEFAULT 0,
    updated_at  BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT chk_favourites_user_id_len CHECK (octet_length(user_id) = 16)
);

CREATE INDEX idx_favourites_user_created_at ON favourites (user_id, created_at DESC);

COMMENT ON TABLE favourites IS 'per-user notebook favourite folder; private index only';
COMMENT ON COLUMN favourites.user_id IS 'owner user id (ulid 16 bytes)';
COMMENT ON COLUMN favourites.name IS 'display name; duplicate names allowed per user';
COMMENT ON COLUMN favourites.created_at IS 'create time (unix ms)';
COMMENT ON COLUMN favourites.updated_at IS 'update time (unix ms)';

CREATE TABLE favourite_notebooks (
    favourite_id  UUID  NOT NULL,
    notebook_id   UUID  NOT NULL,
    favourited_at BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (favourite_id, notebook_id)
);

CREATE INDEX idx_favourite_notebooks_fav_at ON favourite_notebooks (favourite_id, favourited_at DESC);
CREATE INDEX idx_favourite_notebooks_notebook_id ON favourite_notebooks (notebook_id);

COMMENT ON TABLE favourite_notebooks IS 'notebooks inside a favourite; rows kept when notebook becomes inaccessible';
COMMENT ON COLUMN favourite_notebooks.favourited_at IS 'unix ms; first-add time; not refreshed on idempotent re-add';

-- +goose Down
DROP TABLE IF EXISTS favourite_notebooks;
DROP TABLE IF EXISTS favourites;
