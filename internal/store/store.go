package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrInvalidOrder = errors.New("invalid order")
	ErrNotFound     = errors.New("not found")
)

type Board struct {
	ID        int64
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type List struct {
	ID        int64
	BoardID   int64
	Title     string
	Position  int
	Cards     []Card
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Card struct {
	ID          int64
	ListID      int64
	Title       string
	Description string
	Position    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type BoardDetail struct {
	Board Board
	Lists []List
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) ListBoards(ctx context.Context) ([]Board, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, created_at, updated_at
		FROM boards
		ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var board Board
		if err := rows.Scan(&board.ID, &board.Title, &board.CreatedAt, &board.UpdatedAt); err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}
	return boards, rows.Err()
}

func (s *Store) CreateBoard(ctx context.Context, title string) (Board, error) {
	title = cleanTitle(title)
	if title == "" {
		return Board{}, ErrInvalidInput
	}

	var board Board
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO boards (title)
		VALUES ($1)
		RETURNING id, title, created_at, updated_at`, title).
		Scan(&board.ID, &board.Title, &board.CreatedAt, &board.UpdatedAt)
	if err != nil {
		return Board{}, err
	}
	return board, nil
}

func (s *Store) GetBoardDetail(ctx context.Context, boardID int64) (BoardDetail, error) {
	var detail BoardDetail
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, created_at, updated_at
		FROM boards
		WHERE id = $1`, boardID).
		Scan(&detail.Board.ID, &detail.Board.Title, &detail.Board.CreatedAt, &detail.Board.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return BoardDetail{}, ErrNotFound
	}
	if err != nil {
		return BoardDetail{}, err
	}

	listRows, err := s.db.QueryContext(ctx, `
		SELECT id, board_id, title, position, created_at, updated_at
		FROM lists
		WHERE board_id = $1
		ORDER BY position ASC, id ASC`, boardID)
	if err != nil {
		return BoardDetail{}, err
	}
	defer listRows.Close()

	listIndex := make(map[int64]int)
	for listRows.Next() {
		var list List
		if err := listRows.Scan(&list.ID, &list.BoardID, &list.Title, &list.Position, &list.CreatedAt, &list.UpdatedAt); err != nil {
			return BoardDetail{}, err
		}
		listIndex[list.ID] = len(detail.Lists)
		detail.Lists = append(detail.Lists, list)
	}
	if err := listRows.Err(); err != nil {
		return BoardDetail{}, err
	}

	cardRows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.list_id, c.title, c.description, c.position, c.created_at, c.updated_at
		FROM cards c
		JOIN lists l ON l.id = c.list_id
		WHERE l.board_id = $1
		ORDER BY l.position ASC, l.id ASC, c.position ASC, c.id ASC`, boardID)
	if err != nil {
		return BoardDetail{}, err
	}
	defer cardRows.Close()

	for cardRows.Next() {
		var card Card
		if err := cardRows.Scan(&card.ID, &card.ListID, &card.Title, &card.Description, &card.Position, &card.CreatedAt, &card.UpdatedAt); err != nil {
			return BoardDetail{}, err
		}
		if idx, ok := listIndex[card.ListID]; ok {
			detail.Lists[idx].Cards = append(detail.Lists[idx].Cards, card)
		}
	}
	if err := cardRows.Err(); err != nil {
		return BoardDetail{}, err
	}

	return detail, nil
}

func (s *Store) RenameBoard(ctx context.Context, boardID int64, title string) error {
	title = cleanTitle(title)
	if title == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE boards
		SET title = $2, updated_at = now()
		WHERE id = $1`, boardID, title)
	if err != nil {
		return err
	}
	return ensureAffected(result)
}

func (s *Store) DeleteBoard(ctx context.Context, boardID int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM boards WHERE id = $1`, boardID)
	if err != nil {
		return err
	}
	return ensureAffected(result)
}

func (s *Store) CreateList(ctx context.Context, boardID int64, title string) (List, error) {
	title = cleanTitle(title)
	if title == "" {
		return List{}, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return List{}, err
	}
	defer rollback(tx)

	if err := ensureBoardExists(ctx, tx, boardID); err != nil {
		return List{}, err
	}

	var list List
	err = tx.QueryRowContext(ctx, `
		INSERT INTO lists (board_id, title, position)
		VALUES ($1, $2, (
			SELECT COALESCE(MAX(position), 0) + 1
			FROM lists
			WHERE board_id = $1
		))
		RETURNING id, board_id, title, position, created_at, updated_at`, boardID, title).
		Scan(&list.ID, &list.BoardID, &list.Title, &list.Position, &list.CreatedAt, &list.UpdatedAt)
	if err != nil {
		return List{}, err
	}

	if err := tx.Commit(); err != nil {
		return List{}, err
	}
	return list, nil
}

func (s *Store) RenameList(ctx context.Context, listID int64, title string) (int64, error) {
	title = cleanTitle(title)
	if title == "" {
		return 0, ErrInvalidInput
	}

	var boardID int64
	err := s.db.QueryRowContext(ctx, `
		UPDATE lists
		SET title = $2, updated_at = now()
		WHERE id = $1
		RETURNING board_id`, listID, title).
		Scan(&boardID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) DeleteList(ctx context.Context, listID int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)

	boardID, err := boardIDForListTx(ctx, tx, listID)
	if err != nil {
		return 0, err
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM lists WHERE id = $1`, listID)
	if err != nil {
		return 0, err
	}
	if err := ensureAffected(result); err != nil {
		return 0, err
	}
	if err := normalizeListsTx(ctx, tx, boardID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) CreateCard(ctx context.Context, listID int64, title string) (Card, error) {
	title = cleanTitle(title)
	if title == "" {
		return Card{}, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Card{}, err
	}
	defer rollback(tx)

	if _, err := boardIDForListTx(ctx, tx, listID); err != nil {
		return Card{}, err
	}

	var card Card
	err = tx.QueryRowContext(ctx, `
		INSERT INTO cards (list_id, title, position)
		VALUES ($1, $2, (
			SELECT COALESCE(MAX(position), 0) + 1
			FROM cards
			WHERE list_id = $1
		))
		RETURNING id, list_id, title, description, position, created_at, updated_at`, listID, title).
		Scan(&card.ID, &card.ListID, &card.Title, &card.Description, &card.Position, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return Card{}, err
	}

	if err := tx.Commit(); err != nil {
		return Card{}, err
	}
	return card, nil
}

func (s *Store) UpdateCard(ctx context.Context, cardID int64, title string, description string) (int64, error) {
	title = cleanTitle(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return 0, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)

	var listID int64
	err = tx.QueryRowContext(ctx, `
		UPDATE cards
		SET title = $2, description = $3, updated_at = now()
		WHERE id = $1
		RETURNING list_id`, cardID, title, description).
		Scan(&listID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}

	boardID, err := boardIDForListTx(ctx, tx, listID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) DeleteCard(ctx context.Context, cardID int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)

	listID, err := listIDForCardTx(ctx, tx, cardID)
	if err != nil {
		return 0, err
	}
	boardID, err := boardIDForListTx(ctx, tx, listID)
	if err != nil {
		return 0, err
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM cards WHERE id = $1`, cardID)
	if err != nil {
		return 0, err
	}
	if err := ensureAffected(result); err != nil {
		return 0, err
	}
	if err := normalizeCardsTx(ctx, tx, listID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) BoardIDForList(ctx context.Context, listID int64) (int64, error) {
	return boardIDForList(ctx, s.db, listID)
}

func (s *Store) BoardIDForCard(ctx context.Context, cardID int64) (int64, error) {
	var boardID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT l.board_id
		FROM cards c
		JOIN lists l ON l.id = c.list_id
		WHERE c.id = $1`, cardID).Scan(&boardID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) ReorderLists(ctx context.Context, boardID int64, listIDs []int64) error {
	if hasDuplicates(listIDs) {
		return ErrInvalidOrder
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	if err := ensureBoardExists(ctx, tx, boardID); err != nil {
		return err
	}

	currentIDs, err := listIDsForBoardTx(ctx, tx, boardID)
	if err != nil {
		return err
	}
	if !sameSet(currentIDs, listIDs) {
		return ErrInvalidOrder
	}

	for idx, listID := range listIDs {
		result, err := tx.ExecContext(ctx, `
			UPDATE lists
			SET position = $3, updated_at = now()
			WHERE id = $1 AND board_id = $2`, listID, boardID, idx+1)
		if err != nil {
			return err
		}
		if err := ensureAffected(result); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ReorderCards(ctx context.Context, toListID int64, cardIDs []int64) error {
	if hasDuplicates(cardIDs) {
		return ErrInvalidOrder
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	if _, err := boardIDForListTx(ctx, tx, toListID); err != nil {
		return err
	}

	currentDestIDs, err := cardIDsForListTx(ctx, tx, toListID)
	if err != nil {
		return err
	}

	oldListIDs := make(map[int64]struct{})
	for _, cardID := range cardIDs {
		oldListID, err := listIDForCardTx(ctx, tx, cardID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrInvalidOrder
			}
			return err
		}
		if oldListID != toListID {
			oldListIDs[oldListID] = struct{}{}
		}
	}

	expectedIDs := make(map[int64]struct{}, len(currentDestIDs)+len(cardIDs))
	for _, cardID := range currentDestIDs {
		expectedIDs[cardID] = struct{}{}
	}
	for _, cardID := range cardIDs {
		expectedIDs[cardID] = struct{}{}
	}
	if len(expectedIDs) != len(cardIDs) {
		return ErrInvalidOrder
	}
	for _, cardID := range cardIDs {
		if _, ok := expectedIDs[cardID]; !ok {
			return ErrInvalidOrder
		}
	}

	for idx, cardID := range cardIDs {
		result, err := tx.ExecContext(ctx, `
			UPDATE cards
			SET list_id = $1, position = $2, updated_at = now()
			WHERE id = $3`, toListID, idx+1, cardID)
		if err != nil {
			return err
		}
		if err := ensureAffected(result); err != nil {
			return err
		}
	}

	for oldListID := range oldListIDs {
		if err := normalizeCardsTx(ctx, tx, oldListID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func cleanTitle(title string) string {
	return strings.TrimSpace(title)
}

func ensureAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func ensureBoardExists(ctx context.Context, tx *sql.Tx, boardID int64) error {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM boards WHERE id = $1`, boardID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func boardIDForList(ctx context.Context, q queryer, listID int64) (int64, error) {
	var boardID int64
	err := q.QueryRowContext(ctx, `SELECT board_id FROM lists WHERE id = $1`, listID).Scan(&boardID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return boardID, nil
}

func boardIDForListTx(ctx context.Context, tx *sql.Tx, listID int64) (int64, error) {
	return boardIDForList(ctx, tx, listID)
}

func listIDForCardTx(ctx context.Context, tx *sql.Tx, cardID int64) (int64, error) {
	var listID int64
	err := tx.QueryRowContext(ctx, `SELECT list_id FROM cards WHERE id = $1`, cardID).Scan(&listID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return listID, nil
}

func listIDsForBoardTx(ctx context.Context, tx *sql.Tx, boardID int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM lists
		WHERE board_id = $1
		ORDER BY position ASC, id ASC`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func cardIDsForListTx(ctx context.Context, tx *sql.Tx, listID int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM cards
		WHERE list_id = $1
		ORDER BY position ASC, id ASC`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func normalizeListsTx(ctx context.Context, tx *sql.Tx, boardID int64) error {
	_, err := tx.ExecContext(ctx, `
		WITH ranked AS (
			SELECT id, row_number() OVER (ORDER BY position ASC, id ASC)::integer AS new_position
			FROM lists
			WHERE board_id = $1
		)
		UPDATE lists l
		SET position = ranked.new_position, updated_at = now()
		FROM ranked
		WHERE l.id = ranked.id`, boardID)
	return err
}

func normalizeCardsTx(ctx context.Context, tx *sql.Tx, listID int64) error {
	_, err := tx.ExecContext(ctx, `
		WITH ranked AS (
			SELECT id, row_number() OVER (ORDER BY position ASC, id ASC)::integer AS new_position
			FROM cards
			WHERE list_id = $1
		)
		UPDATE cards c
		SET position = ranked.new_position, updated_at = now()
		FROM ranked
		WHERE c.id = ranked.id`, listID)
	return err
}

func hasDuplicates(ids []int64) bool {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return true
		}
		if _, ok := seen[id]; ok {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}

func sameSet(left []int64, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[int64]int, len(left))
	for _, id := range left {
		seen[id]++
	}
	for _, id := range right {
		seen[id]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}
