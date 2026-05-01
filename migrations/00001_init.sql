-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS boards (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'boards' AND column_name = 'name'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'boards' AND column_name = 'title'
    ) THEN
        ALTER TABLE boards RENAME COLUMN name TO title;
    END IF;
END $$;

ALTER TABLE boards
    ADD COLUMN IF NOT EXISTS title TEXT,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE boards SET title = btrim(title) WHERE title IS DISTINCT FROM btrim(title);
ALTER TABLE boards
    ALTER COLUMN title SET NOT NULL,
    ADD CONSTRAINT boards_title_not_blank CHECK (length(btrim(title)) > 0) NOT VALID;
ALTER TABLE boards VALIDATE CONSTRAINT boards_title_not_blank;

CREATE TABLE IF NOT EXISTS lists (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id BIGINT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    position INTEGER NOT NULL CHECK (position > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'lists' AND column_name = 'name'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'lists' AND column_name = 'title'
    ) THEN
        ALTER TABLE lists RENAME COLUMN name TO title;
    END IF;
END $$;

ALTER TABLE lists
    ADD COLUMN IF NOT EXISTS title TEXT,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE lists SET title = btrim(title) WHERE title IS DISTINCT FROM btrim(title);
ALTER TABLE lists
    ALTER COLUMN title SET NOT NULL,
    ADD CONSTRAINT lists_title_not_blank CHECK (length(btrim(title)) > 0) NOT VALID;
ALTER TABLE lists VALIDATE CONSTRAINT lists_title_not_blank;

CREATE INDEX IF NOT EXISTS lists_board_position_idx ON lists (board_id, position, id);

CREATE TABLE IF NOT EXISTS cards (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    list_id BIGINT NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL CHECK (position > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE cards
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE cards SET title = btrim(title) WHERE title IS DISTINCT FROM btrim(title);
ALTER TABLE cards
    ALTER COLUMN title SET NOT NULL,
    ADD CONSTRAINT cards_title_not_blank CHECK (length(btrim(title)) > 0) NOT VALID;
ALTER TABLE cards VALIDATE CONSTRAINT cards_title_not_blank;

CREATE INDEX IF NOT EXISTS cards_list_position_idx ON cards (list_id, position, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS lists;
DROP TABLE IF EXISTS boards;
-- +goose StatementEnd
