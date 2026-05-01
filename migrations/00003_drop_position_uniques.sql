-- +goose Up
-- +goose StatementBegin
ALTER TABLE lists DROP CONSTRAINT IF EXISTS lists_board_id_position_key;
ALTER TABLE cards DROP CONSTRAINT IF EXISTS cards_list_id_position_key;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Position uniqueness is intentionally not restored. Archived rows can share
-- display positions with active rows, and the application maintains ordering.
-- +goose StatementEnd
