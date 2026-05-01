package store

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestStoreLifecycleAndReorder(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s := New(db)
	board, err := s.CreateBoard(ctx, "test board")
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	defer func() {
		_ = s.DeleteBoard(context.Background(), board.ID)
	}()

	todo, err := s.CreateList(ctx, board.ID, "todo")
	if err != nil {
		t.Fatalf("create todo list: %v", err)
	}
	done, err := s.CreateList(ctx, board.ID, "done")
	if err != nil {
		t.Fatalf("create done list: %v", err)
	}

	first, err := s.CreateCard(ctx, todo.ID, "first")
	if err != nil {
		t.Fatalf("create first card: %v", err)
	}
	second, err := s.CreateCard(ctx, todo.ID, "second")
	if err != nil {
		t.Fatalf("create second card: %v", err)
	}
	third, err := s.CreateCard(ctx, done.ID, "third")
	if err != nil {
		t.Fatalf("create third card: %v", err)
	}

	if err := s.ReorderLists(ctx, board.ID, []int64{done.ID, todo.ID}); err != nil {
		t.Fatalf("reorder lists: %v", err)
	}
	if err := s.ReorderCards(ctx, done.ID, []int64{third.ID, first.ID}); err != nil {
		t.Fatalf("move card into done list: %v", err)
	}

	detail, err := s.GetBoardDetail(ctx, board.ID, BoardFilter{})
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if got := detail.Lists[0].ID; got != done.ID {
		t.Fatalf("first list id = %d, want %d", got, done.ID)
	}
	if got := detail.Lists[0].Cards[1].ID; got != first.ID {
		t.Fatalf("moved card id = %d, want %d", got, first.ID)
	}
	if got := detail.Lists[1].Cards[0].ID; got != second.ID {
		t.Fatalf("remaining card id = %d, want %d", got, second.ID)
	}
}

func TestStoreTrelloFeatures(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s := New(db)
	board, err := s.CreateBoard(ctx, "feature board")
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	defer func() {
		_ = s.DeleteBoard(context.Background(), board.ID)
	}()

	list, err := s.CreateList(ctx, board.ID, "todo")
	if err != nil {
		t.Fatalf("create list: %v", err)
	}
	card, err := s.CreateCard(ctx, list.ID, "ship feature")
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	if err := s.CreateLabel(ctx, board.ID, "Important", "red"); err != nil {
		t.Fatalf("create label: %v", err)
	}

	detail, err := s.GetBoardDetail(ctx, board.ID, BoardFilter{})
	if err != nil {
		t.Fatalf("get detail with label: %v", err)
	}
	if len(detail.Labels) != 1 {
		t.Fatalf("labels = %d, want 1", len(detail.Labels))
	}
	labelID := detail.Labels[0].ID

	if _, err := s.SetCardLabels(ctx, card.ID, []int64{labelID}); err != nil {
		t.Fatalf("set labels: %v", err)
	}
	if _, err := s.UpdateCardDates(ctx, card.ID, sql.NullTime{Time: time.Now().Add(48 * time.Hour), Valid: true}, "blue"); err != nil {
		t.Fatalf("update dates: %v", err)
	}
	if _, err := s.AddComment(ctx, card.ID, "Looks good"); err != nil {
		t.Fatalf("add comment: %v", err)
	}
	if _, err := s.AddAttachment(ctx, card.ID, "Spec", "https://example.com/spec"); err != nil {
		t.Fatalf("add attachment: %v", err)
	}
	if _, err := s.CreateChecklist(ctx, card.ID, "Launch"); err != nil {
		t.Fatalf("create checklist: %v", err)
	}

	detail, err = s.GetBoardDetail(ctx, board.ID, BoardFilter{Label: labelID, Due: "week", Status: "incomplete", Query: "ship"})
	if err != nil {
		t.Fatalf("filtered detail: %v", err)
	}
	gotCard := detail.Lists[0].Cards[0]
	if len(gotCard.Labels) != 1 || gotCard.CommentCount != 1 || gotCard.AttachmentCount != 1 || gotCard.CoverColor != "blue" {
		t.Fatalf("decorated card = %#v", gotCard)
	}
	if len(gotCard.Checklists) != 1 {
		t.Fatalf("checklists = %d, want 1", len(gotCard.Checklists))
	}

	checklistID := gotCard.Checklists[0].ID
	if boardID, err := s.CreateChecklistItem(ctx, checklistID, "Write tests"); err != nil {
		t.Fatalf("create checklist item: %v", err)
	} else if boardID != board.ID {
		t.Fatalf("checklist item board id = %d, want %d", boardID, board.ID)
	}
	detail, err = s.GetBoardDetail(ctx, board.ID, BoardFilter{})
	if err != nil {
		t.Fatalf("detail after item: %v", err)
	}
	itemID := detail.Lists[0].Cards[0].Checklists[0].Items[0].ID
	if boardID, checked, err := s.ToggleChecklistItem(ctx, itemID, true); err != nil || !checked {
		t.Fatalf("toggle item checked=%v err=%v", checked, err)
	} else if boardID != board.ID {
		t.Fatalf("toggle board id = %d, want %d", boardID, board.ID)
	}

	if _, err := s.SetCardComplete(ctx, card.ID, true); err != nil {
		t.Fatalf("complete card: %v", err)
	}
	detail, err = s.GetBoardDetail(ctx, board.ID, BoardFilter{Status: "complete"})
	if err != nil {
		t.Fatalf("complete filter: %v", err)
	}
	if len(detail.Lists[0].Cards) != 1 || !detail.Lists[0].Cards[0].CompletedAt.Valid {
		t.Fatalf("complete filtered cards = %#v", detail.Lists[0].Cards)
	}

	if _, err := s.ArchiveCard(ctx, card.ID); err != nil {
		t.Fatalf("archive card: %v", err)
	}
	detail, err = s.GetBoardDetail(ctx, board.ID, BoardFilter{})
	if err != nil {
		t.Fatalf("detail after archive: %v", err)
	}
	if len(detail.Lists[0].Cards) != 0 {
		t.Fatalf("visible cards after archive = %d, want 0", len(detail.Lists[0].Cards))
	}
	archive, err := s.GetArchiveDetail(ctx, board.ID)
	if err != nil {
		t.Fatalf("archive detail: %v", err)
	}
	if len(archive.Cards) != 1 {
		t.Fatalf("archive cards = %d, want 1", len(archive.Cards))
	}
	if _, err := s.RestoreCard(ctx, card.ID); err != nil {
		t.Fatalf("restore card: %v", err)
	}
}
