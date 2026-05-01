-- +goose Up
-- +goose StatementBegin
ALTER TABLE boards
    ADD COLUMN IF NOT EXISTS background_color TEXT NOT NULL DEFAULT 'zinc',
    ADD COLUMN IF NOT EXISTS starred BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE lists
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

ALTER TABLE cards
    ADD COLUMN IF NOT EXISTS due_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cover_color TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS labels (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id BIGINT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    color TEXT NOT NULL CHECK (length(btrim(color)) > 0),
    position INTEGER NOT NULL CHECK (position > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS labels_board_position_idx ON labels (board_id, position, id);

CREATE TABLE IF NOT EXISTS card_labels (
    card_id BIGINT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    label_id BIGINT NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (card_id, label_id)
);

CREATE TABLE IF NOT EXISTS checklists (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id BIGINT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    position INTEGER NOT NULL CHECK (position > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS checklists_card_position_idx ON checklists (card_id, position, id);

CREATE TABLE IF NOT EXISTS checklist_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    checklist_id BIGINT NOT NULL REFERENCES checklists(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    checked_at TIMESTAMPTZ,
    position INTEGER NOT NULL CHECK (position > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS checklist_items_checklist_position_idx ON checklist_items (checklist_id, position, id);

CREATE TABLE IF NOT EXISTS comments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id BIGINT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    body TEXT NOT NULL CHECK (length(btrim(body)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS comments_card_created_idx ON comments (card_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS attachments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id BIGINT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    url TEXT NOT NULL CHECK (length(btrim(url)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS attachments_card_created_idx ON attachments (card_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS activity_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id BIGINT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    card_id BIGINT REFERENCES cards(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL CHECK (length(btrim(event_type)) > 0),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS activity_events_board_created_idx ON activity_events (board_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS activity_events_card_created_idx ON activity_events (card_id, created_at DESC, id DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS activity_events;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS checklist_items;
DROP TABLE IF EXISTS checklists;
DROP TABLE IF EXISTS card_labels;
DROP TABLE IF EXISTS labels;

ALTER TABLE cards
    DROP COLUMN IF EXISTS archived_at,
    DROP COLUMN IF EXISTS cover_color,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS due_at;

ALTER TABLE lists
    DROP COLUMN IF EXISTS archived_at;

ALTER TABLE boards
    DROP COLUMN IF EXISTS starred,
    DROP COLUMN IF EXISTS background_color;
-- +goose StatementEnd
