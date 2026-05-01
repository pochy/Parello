package store

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrInvalidOrder = errors.New("invalid order")
	ErrNotFound     = errors.New("not found")
)

type Board struct {
	ID              int64
	Title           string
	BackgroundColor string
	Starred         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type List struct {
	ID         int64
	BoardID    int64
	Title      string
	Position   int
	ArchivedAt sql.NullTime
	Cards      []Card
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Card struct {
	ID                 int64
	ListID             int64
	Title              string
	Description        string
	Position           int
	StartAt            sql.NullTime
	DueAt              sql.NullTime
	CompletedAt        sql.NullTime
	CoverColor         string
	ArchivedAt         sql.NullTime
	Labels             []Label
	Checklists         []Checklist
	Comments           []Comment
	Attachments        []Attachment
	Activities         []ActivityEvent
	ChecklistTotal     int
	ChecklistCompleted int
	CommentCount       int
	AttachmentCount    int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Label struct {
	ID        int64
	BoardID   int64
	Name      string
	Color     string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Checklist struct {
	ID             int64
	CardID         int64
	Title          string
	Position       int
	Items          []ChecklistItem
	CompletedCount int
	TotalCount     int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ChecklistItem struct {
	ID          int64
	ChecklistID int64
	Title       string
	CheckedAt   sql.NullTime
	Position    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Comment struct {
	ID        int64
	CardID    int64
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Attachment struct {
	ID        int64
	CardID    int64
	Title     string
	URL       string
	CreatedAt time.Time
}

type ActivityEvent struct {
	ID        int64
	BoardID   int64
	CardID    sql.NullInt64
	EventType string
	Message   string
	CreatedAt time.Time
}

type BoardFilter struct {
	Query  string
	Label  int64
	Due    string
	Status string
}

func (f BoardFilter) Active() bool {
	return f.Query != "" || f.Label > 0 || f.Due != "" || f.Status != ""
}

type BoardDetail struct {
	Board  Board
	Lists  []List
	Labels []Label
	Filter BoardFilter
}

type ArchiveDetail struct {
	Board  Board
	Cards  []Card
	Labels []Label
}

type TimelineOptions struct {
	From   time.Time
	Days   int
	Span   string
	Filter BoardFilter
}

type TimelineDetail struct {
	Board       Board
	Lists       []TimelineList
	Labels      []Label
	Filter      BoardFilter
	From        time.Time
	Through     time.Time
	Days        []time.Time
	Span        string
	PrevFrom    time.Time
	NextFrom    time.Time
	Unscheduled []Card
}

type TimelineList struct {
	List        List
	Cards       []TimelineCard
	Unscheduled []Card
	LaneCount   int
}

type TimelineCard struct {
	Card         Card
	StartDate    time.Time
	DueDate      time.Time
	StartOffset  int
	DueOffset    int
	OffsetDays   int
	DurationDays int
	Lane         int
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) ListBoards(ctx context.Context) ([]Board, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, background_color, starred, created_at, updated_at
		FROM boards
		ORDER BY starred DESC, updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var board Board
		if err := rows.Scan(&board.ID, &board.Title, &board.BackgroundColor, &board.Starred, &board.CreatedAt, &board.UpdatedAt); err != nil {
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
		RETURNING id, title, background_color, starred, created_at, updated_at`, title).
		Scan(&board.ID, &board.Title, &board.BackgroundColor, &board.Starred, &board.CreatedAt, &board.UpdatedAt)
	if err != nil {
		return Board{}, err
	}
	return board, nil
}

func (s *Store) GetBoardDetail(ctx context.Context, boardID int64, filter BoardFilter) (BoardDetail, error) {
	board, err := s.getBoard(ctx, boardID)
	if err != nil {
		return BoardDetail{}, err
	}

	labels, err := loadBoardLabels(ctx, s.db, boardID)
	if err != nil {
		return BoardDetail{}, err
	}

	detail := BoardDetail{Board: board, Labels: labels, Filter: normalizeFilter(filter)}

	listRows, err := s.db.QueryContext(ctx, `
		SELECT id, board_id, title, position, archived_at, created_at, updated_at
		FROM lists
		WHERE board_id = $1 AND archived_at IS NULL
		ORDER BY position ASC, id ASC`, boardID)
	if err != nil {
		return BoardDetail{}, err
	}
	defer listRows.Close()

	listIndex := make(map[int64]int)
	for listRows.Next() {
		var list List
		if err := listRows.Scan(&list.ID, &list.BoardID, &list.Title, &list.Position, &list.ArchivedAt, &list.CreatedAt, &list.UpdatedAt); err != nil {
			return BoardDetail{}, err
		}
		listIndex[list.ID] = len(detail.Lists)
		detail.Lists = append(detail.Lists, list)
	}
	if err := listRows.Err(); err != nil {
		return BoardDetail{}, err
	}

	query, args := boardCardsQuery(boardID, detail.Filter, false)
	cardRows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return BoardDetail{}, err
	}
	defer cardRows.Close()

	for cardRows.Next() {
		card, err := scanCard(cardRows)
		if err != nil {
			return BoardDetail{}, err
		}
		if idx, ok := listIndex[card.ListID]; ok {
			detail.Lists[idx].Cards = append(detail.Lists[idx].Cards, card)
		}
	}
	if err := cardRows.Err(); err != nil {
		return BoardDetail{}, err
	}

	cardIndex := cardIndexForLists(detail.Lists)
	if err := loadCardDecorations(ctx, s.db, boardID, cardIndex); err != nil {
		return BoardDetail{}, err
	}
	return detail, nil
}

func (s *Store) GetArchiveDetail(ctx context.Context, boardID int64) (ArchiveDetail, error) {
	board, err := s.getBoard(ctx, boardID)
	if err != nil {
		return ArchiveDetail{}, err
	}
	labels, err := loadBoardLabels(ctx, s.db, boardID)
	if err != nil {
		return ArchiveDetail{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.list_id, c.title, c.description, c.position, c.start_at, c.due_at, c.completed_at, c.cover_color, c.archived_at, c.created_at, c.updated_at
		FROM cards c
		JOIN lists l ON l.id = c.list_id
		WHERE l.board_id = $1 AND c.archived_at IS NOT NULL
		ORDER BY c.archived_at DESC, c.id DESC`, boardID)
	if err != nil {
		return ArchiveDetail{}, err
	}
	defer rows.Close()

	detail := ArchiveDetail{Board: board, Labels: labels}
	for rows.Next() {
		card, err := scanCard(rows)
		if err != nil {
			return ArchiveDetail{}, err
		}
		detail.Cards = append(detail.Cards, card)
	}
	if err := rows.Err(); err != nil {
		return ArchiveDetail{}, err
	}
	cardIndex := cardIndexForCards(detail.Cards)
	if err := loadCardDecorations(ctx, s.db, boardID, cardIndex); err != nil {
		return ArchiveDetail{}, err
	}
	return detail, nil
}

func (s *Store) GetTimelineDetail(ctx context.Context, boardID int64, options TimelineOptions) (TimelineDetail, error) {
	board, err := s.getBoard(ctx, boardID)
	if err != nil {
		return TimelineDetail{}, err
	}
	labels, err := loadBoardLabels(ctx, s.db, boardID)
	if err != nil {
		return TimelineDetail{}, err
	}

	options = normalizeTimelineOptions(options)
	detail := TimelineDetail{
		Board:    board,
		Labels:   labels,
		Filter:   normalizeFilter(options.Filter),
		From:     options.From,
		Through:  options.From.AddDate(0, 0, options.Days-1),
		Days:     timelineDays(options.From, options.Days),
		Span:     options.Span,
		PrevFrom: options.From.AddDate(0, 0, -options.Days),
		NextFrom: options.From.AddDate(0, 0, options.Days),
	}

	listRows, err := s.db.QueryContext(ctx, `
		SELECT id, board_id, title, position, archived_at, created_at, updated_at
		FROM lists
		WHERE board_id = $1 AND archived_at IS NULL
		ORDER BY position ASC, id ASC`, boardID)
	if err != nil {
		return TimelineDetail{}, err
	}
	defer listRows.Close()

	listIndex := make(map[int64]int)
	for listRows.Next() {
		var list List
		if err := listRows.Scan(&list.ID, &list.BoardID, &list.Title, &list.Position, &list.ArchivedAt, &list.CreatedAt, &list.UpdatedAt); err != nil {
			return TimelineDetail{}, err
		}
		listIndex[list.ID] = len(detail.Lists)
		detail.Lists = append(detail.Lists, TimelineList{List: list, LaneCount: 1})
	}
	if err := listRows.Err(); err != nil {
		return TimelineDetail{}, err
	}

	query, args := timelineCardsQuery(boardID, detail.Filter, detail.From, detail.Through)
	cardRows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return TimelineDetail{}, err
	}
	defer cardRows.Close()

	var cards []Card
	for cardRows.Next() {
		card, err := scanCard(cardRows)
		if err != nil {
			return TimelineDetail{}, err
		}
		cards = append(cards, card)
	}
	if err := cardRows.Err(); err != nil {
		return TimelineDetail{}, err
	}

	cardIndex := cardIndexForCards(cards)
	if err := loadCardDecorations(ctx, s.db, boardID, cardIndex); err != nil {
		return TimelineDetail{}, err
	}

	laneEnds := make(map[int64][]time.Time)
	for _, card := range cards {
		idx, ok := listIndex[card.ListID]
		if !ok {
			continue
		}
		start, due, scheduled := cardTimelineRange(card)
		if !scheduled {
			detail.Lists[idx].Unscheduled = append(detail.Lists[idx].Unscheduled, card)
			detail.Unscheduled = append(detail.Unscheduled, card)
			continue
		}
		if due.Before(detail.From) || start.After(detail.Through) {
			continue
		}
		item := buildTimelineCard(card, start, due, detail.From, detail.Through)
		item.Lane = assignTimelineLane(start, due, laneEnds[card.ListID])
		laneEnds[card.ListID] = updateTimelineLaneEnd(start, due, item.Lane, laneEnds[card.ListID])
		detail.Lists[idx].Cards = append(detail.Lists[idx].Cards, item)
		if len(laneEnds[card.ListID]) > detail.Lists[idx].LaneCount {
			detail.Lists[idx].LaneCount = len(laneEnds[card.ListID])
		}
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
			WHERE board_id = $1 AND archived_at IS NULL
		))
		RETURNING id, board_id, title, position, archived_at, created_at, updated_at`, boardID, title).
		Scan(&list.ID, &list.BoardID, &list.Title, &list.Position, &list.ArchivedAt, &list.CreatedAt, &list.UpdatedAt)
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

	boardID, err := boardIDForListTx(ctx, tx, listID)
	if err != nil {
		return Card{}, err
	}

	var card Card
	err = tx.QueryRowContext(ctx, `
		INSERT INTO cards (list_id, title, position)
		VALUES ($1, $2, (
			SELECT COALESCE(MAX(position), 0) + 1
			FROM cards
			WHERE list_id = $1 AND archived_at IS NULL
		))
		RETURNING id, list_id, title, description, position, start_at, due_at, completed_at, cover_color, archived_at, created_at, updated_at`, listID, title).
		Scan(&card.ID, &card.ListID, &card.Title, &card.Description, &card.Position, &card.StartAt, &card.DueAt, &card.CompletedAt, &card.CoverColor, &card.ArchivedAt, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return Card{}, err
	}
	if err := recordActivityTx(ctx, tx, boardID, card.ID, "card_created", "カードを作成しました"); err != nil {
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
	if err := recordActivityTx(ctx, tx, boardID, cardID, "card_updated", "カード本文を更新しました"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) UpdateCardDates(ctx context.Context, cardID int64, startAt sql.NullTime, dueAt sql.NullTime, coverColor string) (int64, error) {
	if !validDateRange(startAt, dueAt) {
		return 0, ErrInvalidInput
	}
	coverColor = cleanColor(coverColor)
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
	_, err = tx.ExecContext(ctx, `
		UPDATE cards
		SET start_at = $2, due_at = $3, cover_color = $4, updated_at = now()
		WHERE id = $1`, cardID, nullableTimeArg(startAt), nullableTimeArg(dueAt), coverColor)
	if err != nil {
		return 0, err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "card_dates_updated", "日付とカバーを更新しました"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) UpdateCardTimeline(ctx context.Context, cardID int64, startAt time.Time, dueAt time.Time) error {
	startAt = truncateDay(startAt)
	dueAt = truncateDay(dueAt)
	if startAt.After(dueAt) {
		return ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	listID, err := listIDForCardTx(ctx, tx, cardID)
	if err != nil {
		return err
	}
	boardID, err := boardIDForListTx(ctx, tx, listID)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE cards
		SET start_at = $2, due_at = $3, updated_at = now()
		WHERE id = $1 AND archived_at IS NULL`, cardID, startAt, dueAt)
	if err != nil {
		return err
	}
	if err := ensureAffected(result); err != nil {
		return err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "card_timeline_updated", "タイムライン日付を更新しました"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetCardComplete(ctx context.Context, cardID int64, complete bool) (int64, error) {
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
	var completedAt any
	message := "カードを未完了に戻しました"
	if complete {
		completedAt = time.Now()
		message = "カードを完了にしました"
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE cards
		SET completed_at = $2, updated_at = now()
		WHERE id = $1`, cardID, completedAt)
	if err != nil {
		return 0, err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "card_completed", message); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) ArchiveCard(ctx context.Context, cardID int64) (int64, error) {
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
	result, err := tx.ExecContext(ctx, `
		UPDATE cards
		SET archived_at = now(), updated_at = now()
		WHERE id = $1 AND archived_at IS NULL`, cardID)
	if err != nil {
		return 0, err
	}
	if err := ensureAffected(result); err != nil {
		return 0, err
	}
	if err := normalizeCardsTx(ctx, tx, listID); err != nil {
		return 0, err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "card_archived", "カードをアーカイブしました"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) RestoreCard(ctx context.Context, cardID int64) (int64, error) {
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
	var nextPosition int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(position), 0) + 1
		FROM cards
		WHERE list_id = $1 AND archived_at IS NULL`, listID).Scan(&nextPosition)
	if err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE cards
		SET archived_at = NULL, position = $2, updated_at = now()
		WHERE id = $1 AND archived_at IS NOT NULL`, cardID, nextPosition)
	if err != nil {
		return 0, err
	}
	if err := ensureAffected(result); err != nil {
		return 0, err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "card_restored", "カードを復元しました"); err != nil {
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

func (s *Store) CreateLabel(ctx context.Context, boardID int64, name string, color string) error {
	name = cleanTitle(name)
	color = cleanColor(color)
	if name == "" || color == "" {
		return ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	if err := ensureBoardExists(ctx, tx, boardID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO labels (board_id, name, color, position)
		VALUES ($1, $2, $3, (
			SELECT COALESCE(MAX(position), 0) + 1
			FROM labels
			WHERE board_id = $1
		))`, boardID, name, color)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetCardLabels(ctx context.Context, cardID int64, labelIDs []int64) (int64, error) {
	if hasDuplicates(labelIDs) {
		return 0, ErrInvalidInput
	}

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
	for _, labelID := range labelIDs {
		if err := ensureLabelBelongsToBoard(ctx, tx, boardID, labelID); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM card_labels WHERE card_id = $1`, cardID); err != nil {
		return 0, err
	}
	for _, labelID := range labelIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO card_labels (card_id, label_id) VALUES ($1, $2)`, cardID, labelID); err != nil {
			return 0, err
		}
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "card_labels_updated", "ラベルを更新しました"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) AddComment(ctx context.Context, cardID int64, body string) (int64, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return 0, ErrInvalidInput
	}

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
	if _, err := tx.ExecContext(ctx, `INSERT INTO comments (card_id, body) VALUES ($1, $2)`, cardID, body); err != nil {
		return 0, err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "comment_added", "コメントを追加しました"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) AddAttachment(ctx context.Context, cardID int64, title string, rawURL string) (int64, error) {
	title = cleanTitle(title)
	rawURL = strings.TrimSpace(rawURL)
	if title == "" || rawURL == "" {
		return 0, ErrInvalidInput
	}
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return 0, ErrInvalidInput
	}

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
	if _, err := tx.ExecContext(ctx, `INSERT INTO attachments (card_id, title, url) VALUES ($1, $2, $3)`, cardID, title, rawURL); err != nil {
		return 0, err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "attachment_added", "添付リンクを追加しました"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) CreateChecklist(ctx context.Context, cardID int64, title string) (int64, error) {
	title = cleanTitle(title)
	if title == "" {
		return 0, ErrInvalidInput
	}

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
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO checklists (card_id, title, position)
		VALUES ($1, $2, (
			SELECT COALESCE(MAX(position), 0) + 1
			FROM checklists
			WHERE card_id = $1
		))`, cardID, title); err != nil {
		return 0, err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "checklist_added", "チェックリストを追加しました"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) CreateChecklistItem(ctx context.Context, checklistID int64, title string) (int64, error) {
	title = cleanTitle(title)
	if title == "" {
		return 0, ErrInvalidInput
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)

	cardID, err := cardIDForChecklistTx(ctx, tx, checklistID)
	if err != nil {
		return 0, err
	}
	listID, err := listIDForCardTx(ctx, tx, cardID)
	if err != nil {
		return 0, err
	}
	boardID, err := boardIDForListTx(ctx, tx, listID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO checklist_items (checklist_id, title, position)
		VALUES ($1, $2, (
			SELECT COALESCE(MAX(position), 0) + 1
			FROM checklist_items
			WHERE checklist_id = $1
		))`, checklistID, title); err != nil {
		return 0, err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "checklist_item_added", "チェック項目を追加しました"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *Store) ToggleChecklistItem(ctx context.Context, itemID int64, checked bool) (int64, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer rollback(tx)

	cardID, err := cardIDForChecklistItemTx(ctx, tx, itemID)
	if err != nil {
		return 0, false, err
	}
	listID, err := listIDForCardTx(ctx, tx, cardID)
	if err != nil {
		return 0, false, err
	}
	boardID, err := boardIDForListTx(ctx, tx, listID)
	if err != nil {
		return 0, false, err
	}
	var checkedAt any
	message := "チェック項目を未完了にしました"
	if checked {
		checkedAt = time.Now()
		message = "チェック項目を完了にしました"
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE checklist_items
		SET checked_at = $2, updated_at = now()
		WHERE id = $1`, itemID, checkedAt)
	if err != nil {
		return 0, false, err
	}
	if err := ensureAffected(result); err != nil {
		return 0, false, err
	}
	if err := recordActivityTx(ctx, tx, boardID, cardID, "checklist_item_toggled", message); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return boardID, checked, nil
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

	tempPositions, err := temporaryListPositionsTx(ctx, tx, boardID, len(listIDs))
	if err != nil {
		return err
	}
	if err := setListPositionsTx(ctx, tx, boardID, listIDs, tempPositions); err != nil {
		return err
	}
	finalPositions, err := activeListPositionsTx(ctx, tx, boardID, len(listIDs))
	if err != nil {
		return err
	}
	if err := setListPositionsTx(ctx, tx, boardID, listIDs, finalPositions); err != nil {
		return err
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

	boardID, err := boardIDForListTx(ctx, tx, toListID)
	if err != nil {
		return err
	}

	currentDestIDs, err := cardIDsForListTx(ctx, tx, toListID)
	if err != nil {
		return err
	}
	if isStrictSubset(cardIDs, currentDestIDs) {
		return tx.Commit()
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

	tempPositions, err := temporaryCardPositionsTx(ctx, tx, toListID, len(cardIDs))
	if err != nil {
		return err
	}
	if err := setCardPositionsTx(ctx, tx, toListID, cardIDs, tempPositions); err != nil {
		return err
	}
	finalPositions, err := activeCardPositionsTx(ctx, tx, toListID, len(cardIDs))
	if err != nil {
		return err
	}
	if err := setCardPositionsTx(ctx, tx, toListID, cardIDs, finalPositions); err != nil {
		return err
	}

	for _, cardID := range cardIDs {
		if err := recordActivityTx(ctx, tx, boardID, cardID, "card_moved", "カードを移動しました"); err != nil {
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

func setListPositionsTx(ctx context.Context, tx *sql.Tx, boardID int64, listIDs []int64, positions []int) error {
	if len(listIDs) != len(positions) {
		return ErrInvalidOrder
	}
	for idx, listID := range listIDs {
		result, err := tx.ExecContext(ctx, `
			UPDATE lists
			SET position = $3, updated_at = now()
			WHERE id = $1 AND board_id = $2 AND archived_at IS NULL`, listID, boardID, positions[idx])
		if err != nil {
			return err
		}
		if err := ensureAffected(result); err != nil {
			return err
		}
	}

	return nil
}

func setCardPositionsTx(ctx context.Context, tx *sql.Tx, listID int64, cardIDs []int64, positions []int) error {
	if len(cardIDs) != len(positions) {
		return ErrInvalidOrder
	}
	for idx, cardID := range cardIDs {
		result, err := tx.ExecContext(ctx, `
			UPDATE cards
			SET list_id = $1, position = $2, updated_at = now()
			WHERE id = $3 AND archived_at IS NULL`, listID, positions[idx], cardID)
		if err != nil {
			return err
		}
		if err := ensureAffected(result); err != nil {
			return err
		}
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type queryExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) getBoard(ctx context.Context, boardID int64) (Board, error) {
	var board Board
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, background_color, starred, created_at, updated_at
		FROM boards
		WHERE id = $1`, boardID).
		Scan(&board.ID, &board.Title, &board.BackgroundColor, &board.Starred, &board.CreatedAt, &board.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Board{}, ErrNotFound
	}
	if err != nil {
		return Board{}, err
	}
	return board, nil
}

func scanCard(scanner rowScanner) (Card, error) {
	var card Card
	err := scanner.Scan(&card.ID, &card.ListID, &card.Title, &card.Description, &card.Position, &card.StartAt, &card.DueAt, &card.CompletedAt, &card.CoverColor, &card.ArchivedAt, &card.CreatedAt, &card.UpdatedAt)
	return card, err
}

func normalizeFilter(filter BoardFilter) BoardFilter {
	filter.Query = strings.TrimSpace(filter.Query)
	switch filter.Due {
	case "overdue", "today", "week", "none":
	default:
		filter.Due = ""
	}
	switch filter.Status {
	case "complete", "incomplete":
	default:
		filter.Status = ""
	}
	if filter.Label < 0 {
		filter.Label = 0
	}
	return filter
}

func boardCardsQuery(boardID int64, filter BoardFilter, archived bool) (string, []any) {
	args := []any{boardID}
	where := []string{"l.board_id = $1"}
	if archived {
		where = append(where, "c.archived_at IS NOT NULL")
	} else {
		where = append(where, "l.archived_at IS NULL", "c.archived_at IS NULL")
	}

	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		where = append(where, "(c.title ILIKE $"+placeholder(len(args))+" OR c.description ILIKE $"+placeholder(len(args))+")")
	}
	if filter.Label > 0 {
		args = append(args, filter.Label)
		where = append(where, "EXISTS (SELECT 1 FROM card_labels cl WHERE cl.card_id = c.id AND cl.label_id = $"+placeholder(len(args))+")")
	}
	switch filter.Due {
	case "overdue":
		where = append(where, "c.due_at IS NOT NULL AND c.due_at < now() AND c.completed_at IS NULL")
	case "today":
		where = append(where, "c.due_at IS NOT NULL AND c.due_at >= date_trunc('day', now()) AND c.due_at < date_trunc('day', now()) + interval '1 day'")
	case "week":
		where = append(where, "c.due_at IS NOT NULL AND c.due_at >= now() AND c.due_at < now() + interval '7 days'")
	case "none":
		where = append(where, "c.due_at IS NULL")
	}
	switch filter.Status {
	case "complete":
		where = append(where, "c.completed_at IS NOT NULL")
	case "incomplete":
		where = append(where, "c.completed_at IS NULL")
	}

	query := `
		SELECT c.id, c.list_id, c.title, c.description, c.position, c.start_at, c.due_at, c.completed_at, c.cover_color, c.archived_at, c.created_at, c.updated_at
		FROM cards c
		JOIN lists l ON l.id = c.list_id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY l.position ASC, l.id ASC, c.position ASC, c.id ASC`
	return query, args
}

func timelineCardsQuery(boardID int64, filter BoardFilter, from time.Time, through time.Time) (string, []any) {
	args := []any{boardID}
	where := []string{"l.board_id = $1", "l.archived_at IS NULL", "c.archived_at IS NULL"}

	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		where = append(where, "(c.title ILIKE $"+placeholder(len(args))+" OR c.description ILIKE $"+placeholder(len(args))+")")
	}
	if filter.Label > 0 {
		args = append(args, filter.Label)
		where = append(where, "EXISTS (SELECT 1 FROM card_labels cl WHERE cl.card_id = c.id AND cl.label_id = $"+placeholder(len(args))+")")
	}
	switch filter.Due {
	case "overdue":
		where = append(where, "c.due_at IS NOT NULL AND c.due_at < now() AND c.completed_at IS NULL")
	case "today":
		where = append(where, "c.due_at IS NOT NULL AND c.due_at >= date_trunc('day', now()) AND c.due_at < date_trunc('day', now()) + interval '1 day'")
	case "week":
		where = append(where, "c.due_at IS NOT NULL AND c.due_at >= now() AND c.due_at < now() + interval '7 days'")
	case "none":
		where = append(where, "c.due_at IS NULL")
	}
	switch filter.Status {
	case "complete":
		where = append(where, "c.completed_at IS NOT NULL")
	case "incomplete":
		where = append(where, "c.completed_at IS NULL")
	}

	args = append(args, from, through)
	fromPlaceholder := placeholder(len(args) - 1)
	throughPlaceholder := placeholder(len(args))
	where = append(where, `(
		(c.start_at IS NULL AND c.due_at IS NULL)
		OR (
			COALESCE(c.due_at, c.start_at) >= $`+fromPlaceholder+`
			AND COALESCE(c.start_at, c.due_at) <= $`+throughPlaceholder+`
		)
	)`)

	query := `
		SELECT c.id, c.list_id, c.title, c.description, c.position, c.start_at, c.due_at, c.completed_at, c.cover_color, c.archived_at, c.created_at, c.updated_at
		FROM cards c
		JOIN lists l ON l.id = c.list_id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY l.position ASC, l.id ASC, COALESCE(c.start_at, c.due_at) ASC NULLS LAST, COALESCE(c.due_at, c.start_at) ASC NULLS LAST, c.position ASC, c.id ASC`
	return query, args
}

func placeholder(n int) string {
	return strconv.Itoa(n)
}

func loadBoardLabels(ctx context.Context, q queryExecutor, boardID int64) ([]Label, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, board_id, name, color, position, created_at, updated_at
		FROM labels
		WHERE board_id = $1
		ORDER BY position ASC, id ASC`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []Label
	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.ID, &label.BoardID, &label.Name, &label.Color, &label.Position, &label.CreatedAt, &label.UpdatedAt); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

func loadCardDecorations(ctx context.Context, q queryExecutor, boardID int64, cards map[int64]*Card) error {
	if len(cards) == 0 {
		return nil
	}
	if err := loadCardLabels(ctx, q, boardID, cards); err != nil {
		return err
	}
	if err := loadCardChecklists(ctx, q, boardID, cards); err != nil {
		return err
	}
	if err := loadCardComments(ctx, q, boardID, cards); err != nil {
		return err
	}
	if err := loadCardAttachments(ctx, q, boardID, cards); err != nil {
		return err
	}
	return loadCardActivities(ctx, q, boardID, cards)
}

func loadCardLabels(ctx context.Context, q queryExecutor, boardID int64, cards map[int64]*Card) error {
	rows, err := q.QueryContext(ctx, `
		SELECT cl.card_id, l.id, l.board_id, l.name, l.color, l.position, l.created_at, l.updated_at
		FROM card_labels cl
		JOIN labels l ON l.id = cl.label_id
		WHERE l.board_id = $1
		ORDER BY l.position ASC, l.id ASC`, boardID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cardID int64
		var label Label
		if err := rows.Scan(&cardID, &label.ID, &label.BoardID, &label.Name, &label.Color, &label.Position, &label.CreatedAt, &label.UpdatedAt); err != nil {
			return err
		}
		if card, ok := cards[cardID]; ok {
			card.Labels = append(card.Labels, label)
		}
	}
	return rows.Err()
}

func loadCardChecklists(ctx context.Context, q queryExecutor, boardID int64, cards map[int64]*Card) error {
	rows, err := q.QueryContext(ctx, `
		SELECT ch.id, ch.card_id, ch.title, ch.position, ch.created_at, ch.updated_at
		FROM checklists ch
		JOIN cards c ON c.id = ch.card_id
		JOIN lists l ON l.id = c.list_id
		WHERE l.board_id = $1
		ORDER BY ch.position ASC, ch.id ASC`, boardID)
	if err != nil {
		return err
	}
	defer rows.Close()

	byCard := make(map[int64][]Checklist)
	for rows.Next() {
		var checklist Checklist
		if err := rows.Scan(&checklist.ID, &checklist.CardID, &checklist.Title, &checklist.Position, &checklist.CreatedAt, &checklist.UpdatedAt); err != nil {
			return err
		}
		if _, ok := cards[checklist.CardID]; ok {
			byCard[checklist.CardID] = append(byCard[checklist.CardID], checklist)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	checklists := make(map[int64]*Checklist)
	for cardID, items := range byCard {
		card := cards[cardID]
		card.Checklists = items
		for idx := range card.Checklists {
			checklists[card.Checklists[idx].ID] = &card.Checklists[idx]
		}
	}

	itemRows, err := q.QueryContext(ctx, `
		SELECT ci.id, ci.checklist_id, ci.title, ci.checked_at, ci.position, ci.created_at, ci.updated_at
		FROM checklist_items ci
		JOIN checklists ch ON ch.id = ci.checklist_id
		JOIN cards c ON c.id = ch.card_id
		JOIN lists l ON l.id = c.list_id
		WHERE l.board_id = $1
		ORDER BY ci.position ASC, ci.id ASC`, boardID)
	if err != nil {
		return err
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var item ChecklistItem
		if err := itemRows.Scan(&item.ID, &item.ChecklistID, &item.Title, &item.CheckedAt, &item.Position, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		if checklist, ok := checklists[item.ChecklistID]; ok {
			checklist.Items = append(checklist.Items, item)
			checklist.TotalCount++
			card := cards[checklist.CardID]
			card.ChecklistTotal++
			if item.CheckedAt.Valid {
				checklist.CompletedCount++
				card.ChecklistCompleted++
			}
		}
	}
	return itemRows.Err()
}

func loadCardComments(ctx context.Context, q queryExecutor, boardID int64, cards map[int64]*Card) error {
	rows, err := q.QueryContext(ctx, `
		SELECT co.id, co.card_id, co.body, co.created_at, co.updated_at
		FROM comments co
		JOIN cards c ON c.id = co.card_id
		JOIN lists l ON l.id = c.list_id
		WHERE l.board_id = $1
		ORDER BY co.created_at DESC, co.id DESC`, boardID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.ID, &comment.CardID, &comment.Body, &comment.CreatedAt, &comment.UpdatedAt); err != nil {
			return err
		}
		if card, ok := cards[comment.CardID]; ok {
			card.Comments = append(card.Comments, comment)
			card.CommentCount++
		}
	}
	return rows.Err()
}

func loadCardAttachments(ctx context.Context, q queryExecutor, boardID int64, cards map[int64]*Card) error {
	rows, err := q.QueryContext(ctx, `
		SELECT a.id, a.card_id, a.title, a.url, a.created_at
		FROM attachments a
		JOIN cards c ON c.id = a.card_id
		JOIN lists l ON l.id = c.list_id
		WHERE l.board_id = $1
		ORDER BY a.created_at DESC, a.id DESC`, boardID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var attachment Attachment
		if err := rows.Scan(&attachment.ID, &attachment.CardID, &attachment.Title, &attachment.URL, &attachment.CreatedAt); err != nil {
			return err
		}
		if card, ok := cards[attachment.CardID]; ok {
			card.Attachments = append(card.Attachments, attachment)
			card.AttachmentCount++
		}
	}
	return rows.Err()
}

func loadCardActivities(ctx context.Context, q queryExecutor, boardID int64, cards map[int64]*Card) error {
	rows, err := q.QueryContext(ctx, `
		SELECT id, board_id, card_id, event_type, COALESCE(payload->>'message', event_type), created_at
		FROM activity_events
		WHERE board_id = $1 AND card_id IS NOT NULL
		ORDER BY created_at DESC, id DESC`, boardID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var event ActivityEvent
		if err := rows.Scan(&event.ID, &event.BoardID, &event.CardID, &event.EventType, &event.Message, &event.CreatedAt); err != nil {
			return err
		}
		if event.CardID.Valid {
			if card, ok := cards[event.CardID.Int64]; ok {
				card.Activities = append(card.Activities, event)
			}
		}
	}
	return rows.Err()
}

func cardIndexForLists(lists []List) map[int64]*Card {
	cards := make(map[int64]*Card)
	for listIdx := range lists {
		for cardIdx := range lists[listIdx].Cards {
			cards[lists[listIdx].Cards[cardIdx].ID] = &lists[listIdx].Cards[cardIdx]
		}
	}
	return cards
}

func cardIndexForCards(cardList []Card) map[int64]*Card {
	cards := make(map[int64]*Card)
	for idx := range cardList {
		cards[cardList[idx].ID] = &cardList[idx]
	}
	return cards
}

func cleanTitle(title string) string {
	return strings.TrimSpace(title)
}

func cleanColor(color string) string {
	color = strings.TrimSpace(color)
	if color == "" {
		return ""
	}
	if _, ok := allowedColors[color]; ok {
		return color
	}
	return ""
}

var allowedColors = map[string]struct{}{
	"green":  {},
	"yellow": {},
	"orange": {},
	"red":    {},
	"blue":   {},
	"purple": {},
	"pink":   {},
	"gray":   {},
}

func nullableTimeArg(value sql.NullTime) any {
	if value.Valid {
		return truncateDay(value.Time)
	}
	return nil
}

func validDateRange(startAt sql.NullTime, dueAt sql.NullTime) bool {
	if !startAt.Valid || !dueAt.Valid {
		return true
	}
	return !truncateDay(startAt.Time).After(truncateDay(dueAt.Time))
}

func normalizeTimelineOptions(options TimelineOptions) TimelineOptions {
	if options.Days <= 0 {
		options.Days = 42
	}
	if options.Span != "quarter" {
		options.Span = "6w"
	}
	if options.Span == "quarter" {
		options.Days = 91
	}
	if options.From.IsZero() {
		options.From = weekStart(time.Now())
	}
	options.From = truncateDay(options.From)
	options.Filter = normalizeFilter(options.Filter)
	return options
}

func timelineDays(from time.Time, count int) []time.Time {
	days := make([]time.Time, count)
	for i := range days {
		days[i] = from.AddDate(0, 0, i)
	}
	return days
}

func cardTimelineRange(card Card) (time.Time, time.Time, bool) {
	if !card.StartAt.Valid && !card.DueAt.Valid {
		return time.Time{}, time.Time{}, false
	}
	start := card.StartAt.Time
	if !card.StartAt.Valid {
		start = card.DueAt.Time
	}
	due := card.DueAt.Time
	if !card.DueAt.Valid {
		due = card.StartAt.Time
	}
	start = truncateDay(start)
	due = truncateDay(due)
	if start.After(due) {
		return due, start, true
	}
	return start, due, true
}

func buildTimelineCard(card Card, start time.Time, due time.Time, from time.Time, through time.Time) TimelineCard {
	startOffset := daysBetween(from, start)
	dueOffset := daysBetween(from, due)
	visibleStart := maxInt(0, startOffset)
	visibleDue := minInt(daysBetween(from, through), dueOffset)
	return TimelineCard{
		Card:         card,
		StartDate:    start,
		DueDate:      due,
		StartOffset:  startOffset,
		DueOffset:    dueOffset,
		OffsetDays:   visibleStart,
		DurationDays: maxInt(1, visibleDue-visibleStart+1),
	}
}

func assignTimelineLane(start time.Time, due time.Time, laneEnds []time.Time) int {
	for idx, laneEnd := range laneEnds {
		if start.After(laneEnd) {
			return idx
		}
	}
	return len(laneEnds)
}

func updateTimelineLaneEnd(_ time.Time, due time.Time, lane int, laneEnds []time.Time) []time.Time {
	for len(laneEnds) <= lane {
		laneEnds = append(laneEnds, time.Time{})
	}
	laneEnds[lane] = due
	return laneEnds
}

func weekStart(value time.Time) time.Time {
	day := truncateDay(value)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func truncateDay(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	year, month, day := value.In(time.Local).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}

func daysBetween(from time.Time, to time.Time) int {
	return int(truncateDay(to).Sub(truncateDay(from)).Hours() / 24)
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
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

func ensureLabelBelongsToBoard(ctx context.Context, tx *sql.Tx, boardID int64, labelID int64) error {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM labels WHERE id = $1 AND board_id = $2`, labelID, boardID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidInput
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

func cardIDForChecklistTx(ctx context.Context, tx *sql.Tx, checklistID int64) (int64, error) {
	var cardID int64
	err := tx.QueryRowContext(ctx, `SELECT card_id FROM checklists WHERE id = $1`, checklistID).Scan(&cardID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return cardID, nil
}

func cardIDForChecklistItemTx(ctx context.Context, tx *sql.Tx, itemID int64) (int64, error) {
	var cardID int64
	err := tx.QueryRowContext(ctx, `
		SELECT ch.card_id
		FROM checklist_items ci
		JOIN checklists ch ON ch.id = ci.checklist_id
		WHERE ci.id = $1`, itemID).Scan(&cardID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return cardID, nil
}

func listIDsForBoardTx(ctx context.Context, tx *sql.Tx, boardID int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM lists
		WHERE board_id = $1 AND archived_at IS NULL
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
		WHERE list_id = $1 AND archived_at IS NULL
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

func temporaryListPositionsTx(ctx context.Context, tx *sql.Tx, boardID int64, count int) ([]int, error) {
	if count == 0 {
		return nil, nil
	}
	var start int
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(position), 0) + 1
		FROM lists
		WHERE board_id = $1`, boardID).Scan(&start)
	if err != nil {
		return nil, err
	}
	return consecutivePositions(start, count), nil
}

func activeListPositionsTx(ctx context.Context, tx *sql.Tx, boardID int64, count int) ([]int, error) {
	occupied, err := archivedListPositionsTx(ctx, tx, boardID)
	if err != nil {
		return nil, err
	}
	return nextAvailablePositions(occupied, count), nil
}

func archivedListPositionsTx(ctx context.Context, tx *sql.Tx, boardID int64) (map[int]struct{}, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT position
		FROM lists
		WHERE board_id = $1 AND archived_at IS NOT NULL`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := map[int]struct{}{}
	for rows.Next() {
		var position int
		if err := rows.Scan(&position); err != nil {
			return nil, err
		}
		positions[position] = struct{}{}
	}
	return positions, rows.Err()
}

func temporaryCardPositionsTx(ctx context.Context, tx *sql.Tx, listID int64, count int) ([]int, error) {
	if count == 0 {
		return nil, nil
	}
	var start int
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(position), 0) + 1
		FROM cards
		WHERE list_id = $1`, listID).Scan(&start)
	if err != nil {
		return nil, err
	}
	return consecutivePositions(start, count), nil
}

func activeCardPositionsTx(ctx context.Context, tx *sql.Tx, listID int64, count int) ([]int, error) {
	occupied, err := archivedCardPositionsTx(ctx, tx, listID)
	if err != nil {
		return nil, err
	}
	return nextAvailablePositions(occupied, count), nil
}

func archivedCardPositionsTx(ctx context.Context, tx *sql.Tx, listID int64) (map[int]struct{}, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT position
		FROM cards
		WHERE list_id = $1 AND archived_at IS NOT NULL`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := map[int]struct{}{}
	for rows.Next() {
		var position int
		if err := rows.Scan(&position); err != nil {
			return nil, err
		}
		positions[position] = struct{}{}
	}
	return positions, rows.Err()
}

func consecutivePositions(start int, count int) []int {
	positions := make([]int, count)
	for idx := range positions {
		positions[idx] = start + idx
	}
	return positions
}

func nextAvailablePositions(occupied map[int]struct{}, count int) []int {
	positions := make([]int, 0, count)
	for position := 1; len(positions) < count; position++ {
		if _, ok := occupied[position]; ok {
			continue
		}
		positions = append(positions, position)
	}
	return positions
}

func normalizeListsTx(ctx context.Context, tx *sql.Tx, boardID int64) error {
	listIDs, err := listIDsForBoardTx(ctx, tx, boardID)
	if err != nil {
		return err
	}
	tempPositions, err := temporaryListPositionsTx(ctx, tx, boardID, len(listIDs))
	if err != nil {
		return err
	}
	if err := setListPositionsTx(ctx, tx, boardID, listIDs, tempPositions); err != nil {
		return err
	}
	finalPositions, err := activeListPositionsTx(ctx, tx, boardID, len(listIDs))
	if err != nil {
		return err
	}
	return setListPositionsTx(ctx, tx, boardID, listIDs, finalPositions)
}

func normalizeCardsTx(ctx context.Context, tx *sql.Tx, listID int64) error {
	cardIDs, err := cardIDsForListTx(ctx, tx, listID)
	if err != nil {
		return err
	}
	tempPositions, err := temporaryCardPositionsTx(ctx, tx, listID, len(cardIDs))
	if err != nil {
		return err
	}
	if err := setCardPositionsTx(ctx, tx, listID, cardIDs, tempPositions); err != nil {
		return err
	}
	finalPositions, err := activeCardPositionsTx(ctx, tx, listID, len(cardIDs))
	if err != nil {
		return err
	}
	return setCardPositionsTx(ctx, tx, listID, cardIDs, finalPositions)
}

func recordActivityTx(ctx context.Context, tx *sql.Tx, boardID int64, cardID int64, eventType string, message string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO activity_events (board_id, card_id, event_type, payload)
		VALUES ($1, $2, $3, jsonb_build_object('message', $4::text))`, boardID, cardID, eventType, message)
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

func isStrictSubset(subset []int64, set []int64) bool {
	if len(subset) >= len(set) {
		return false
	}
	seen := make(map[int64]struct{}, len(set))
	for _, id := range set {
		seen[id] = struct{}{}
	}
	for _, id := range subset {
		if _, ok := seen[id]; !ok {
			return false
		}
	}
	return true
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}
