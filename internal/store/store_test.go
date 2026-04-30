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

	detail, err := s.GetBoardDetail(ctx, board.ID)
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
