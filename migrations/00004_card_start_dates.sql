-- +goose Up
-- +goose StatementBegin
ALTER TABLE cards
    ADD COLUMN IF NOT EXISTS start_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS cards_timeline_idx ON cards (list_id, start_at, due_at, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS cards_timeline_idx;

ALTER TABLE cards
    DROP COLUMN IF EXISTS start_at;
-- +goose StatementEnd
