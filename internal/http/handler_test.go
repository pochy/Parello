package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"golangkanban/internal/store"
)

func TestRootRedirectsToBoards(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	New(&fakeStore{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if location := rec.Header().Get("Location"); location != "/boards" {
		t.Fatalf("location = %q, want /boards", location)
	}
}

func TestCreateBoardRedirectsToCreatedBoard(t *testing.T) {
	form := url.Values{"title": {"Roadmap"}}
	req := httptest.NewRequest(http.MethodPost, "/boards", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	New(fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if location := rec.Header().Get("Location"); location != "/boards/7" {
		t.Fatalf("location = %q, want /boards/7", location)
	}
	if fake.createdBoardTitle != "Roadmap" {
		t.Fatalf("created title = %q, want Roadmap", fake.createdBoardTitle)
	}
}

func TestReorderListsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/lists/reorder", bytes.NewBufferString(`{"boardId":7,"listIds":[3,1,2]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	New(fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if fake.reorderBoardID != 7 {
		t.Fatalf("board id = %d, want 7", fake.reorderBoardID)
	}
	if !reflect.DeepEqual(fake.reorderListIDs, []int64{3, 1, 2}) {
		t.Fatalf("list ids = %#v", fake.reorderListIDs)
	}
}

type fakeStore struct {
	createdBoardTitle string
	reorderBoardID    int64
	reorderListIDs    []int64
}

func (f *fakeStore) ListBoards(context.Context) ([]store.Board, error) {
	return nil, nil
}

func (f *fakeStore) CreateBoard(_ context.Context, title string) (store.Board, error) {
	f.createdBoardTitle = title
	return store.Board{ID: 7, Title: title}, nil
}

func (f *fakeStore) GetBoardDetail(context.Context, int64) (store.BoardDetail, error) {
	return store.BoardDetail{}, store.ErrNotFound
}

func (f *fakeStore) RenameBoard(context.Context, int64, string) error {
	return nil
}

func (f *fakeStore) DeleteBoard(context.Context, int64) error {
	return nil
}

func (f *fakeStore) CreateList(context.Context, int64, string) (store.List, error) {
	return store.List{}, nil
}

func (f *fakeStore) RenameList(context.Context, int64, string) (int64, error) {
	return 0, nil
}

func (f *fakeStore) DeleteList(context.Context, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) CreateCard(context.Context, int64, string) (store.Card, error) {
	return store.Card{}, nil
}

func (f *fakeStore) UpdateCard(context.Context, int64, string, string) (int64, error) {
	return 0, nil
}

func (f *fakeStore) DeleteCard(context.Context, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) BoardIDForList(context.Context, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) BoardIDForCard(context.Context, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) ReorderLists(_ context.Context, boardID int64, listIDs []int64) error {
	f.reorderBoardID = boardID
	f.reorderListIDs = append([]int64(nil), listIDs...)
	return nil
}

func (f *fakeStore) ReorderCards(context.Context, int64, []int64) error {
	return nil
}
