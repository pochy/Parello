package web

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestToggleChecklistItemJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/checklist-items/9", bytes.NewBufferString(`{"checked":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	New(fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fake.toggledItemID != 9 {
		t.Fatalf("item id = %d, want 9", fake.toggledItemID)
	}
	if !fake.toggledChecked {
		t.Fatal("checked = false, want true")
	}
}

func TestBoardTimelineRenders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/boards/7/timeline", nil)
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	New(fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fake.timelineBoardID != 7 {
		t.Fatalf("timeline board id = %d, want 7", fake.timelineBoardID)
	}
	if !strings.Contains(rec.Body.String(), "Timeline Board のタイムライン") {
		t.Fatalf("body does not include timeline title: %s", rec.Body.String())
	}
}

func TestUpdateCardTimelineJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/cards/9/timeline", bytes.NewBufferString(`{"startAt":"2026-05-04","dueAt":"2026-05-08"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	New(fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if fake.timelineCardID != 9 {
		t.Fatalf("card id = %d, want 9", fake.timelineCardID)
	}
	if got := fake.timelineStart.Format("2006-01-02"); got != "2026-05-04" {
		t.Fatalf("start = %s, want 2026-05-04", got)
	}
	if got := fake.timelineDue.Format("2006-01-02"); got != "2026-05-08" {
		t.Fatalf("due = %s, want 2026-05-08", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["startAt"] != "2026-05-04" || body["dueAt"] != "2026-05-08" {
		t.Fatalf("response = %#v", body)
	}
}

func TestUpdateCardTimelineRejectsInvertedDates(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/cards/9/timeline", bytes.NewBufferString(`{"startAt":"2026-05-08","dueAt":"2026-05-04"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	New(fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if fake.timelineCardID != 0 {
		t.Fatalf("timeline update should not be called, got card id %d", fake.timelineCardID)
	}
}

func TestUpdateCardDatesAcceptsStartAt(t *testing.T) {
	form := url.Values{
		"start_at":    {"2026-05-04"},
		"due_at":      {"2026-05-08"},
		"cover_color": {"blue"},
	}
	req := httptest.NewRequest(http.MethodPost, "/cards/9/dates", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	New(fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if fake.datesCardID != 9 {
		t.Fatalf("card id = %d, want 9", fake.datesCardID)
	}
	if got := fake.datesStart.Time.Format("2006-01-02"); got != "2026-05-04" {
		t.Fatalf("start = %s, want 2026-05-04", got)
	}
	if got := fake.datesDue.Time.Format("2006-01-02"); got != "2026-05-08" {
		t.Fatalf("due = %s, want 2026-05-08", got)
	}
	if fake.datesCover != "blue" {
		t.Fatalf("cover = %q, want blue", fake.datesCover)
	}
}

func TestCreateChecklistFallbackRedirectsToBoard(t *testing.T) {
	form := url.Values{"title": {"Launch"}}
	req := httptest.NewRequest(http.MethodPost, "/cards/9/checklists", strings.NewReader(form.Encode()))
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
	if fake.createdChecklistCardID != 9 {
		t.Fatalf("checklist card id = %d, want 9", fake.createdChecklistCardID)
	}
}

func TestToggleChecklistItemFallbackRedirectsToBoard(t *testing.T) {
	form := url.Values{"checked": {"true"}}
	req := httptest.NewRequest(http.MethodPost, "/checklist-items/11/toggle", strings.NewReader(form.Encode()))
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
	if fake.toggledItemID != 11 || !fake.toggledChecked {
		t.Fatalf("toggle = item %d checked %v, want item 11 checked true", fake.toggledItemID, fake.toggledChecked)
	}
}

type fakeStore struct {
	createdBoardTitle      string
	createdChecklistCardID int64
	datesCardID            int64
	datesStart             sql.NullTime
	datesDue               sql.NullTime
	datesCover             string
	reorderBoardID         int64
	reorderListIDs         []int64
	timelineBoardID        int64
	timelineCardID         int64
	timelineStart          time.Time
	timelineDue            time.Time
	toggledItemID          int64
	toggledChecked         bool
}

func (f *fakeStore) ListBoards(context.Context) ([]store.Board, error) {
	return nil, nil
}

func (f *fakeStore) CreateBoard(_ context.Context, title string) (store.Board, error) {
	f.createdBoardTitle = title
	return store.Board{ID: 7, Title: title}, nil
}

func (f *fakeStore) GetBoardDetail(context.Context, int64, store.BoardFilter) (store.BoardDetail, error) {
	return store.BoardDetail{}, store.ErrNotFound
}

func (f *fakeStore) GetArchiveDetail(context.Context, int64) (store.ArchiveDetail, error) {
	return store.ArchiveDetail{}, store.ErrNotFound
}

func (f *fakeStore) GetTimelineDetail(_ context.Context, boardID int64, options store.TimelineOptions) (store.TimelineDetail, error) {
	f.timelineBoardID = boardID
	if options.From.IsZero() {
		options.From = time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local)
	}
	if options.Days == 0 {
		options.Days = 42
	}
	return store.TimelineDetail{
		Board:    store.Board{ID: boardID, Title: "Timeline Board"},
		From:     options.From,
		Through:  options.From.AddDate(0, 0, options.Days-1),
		Days:     []time.Time{options.From},
		Span:     options.Span,
		PrevFrom: options.From.AddDate(0, 0, -options.Days),
		NextFrom: options.From.AddDate(0, 0, options.Days),
	}, nil
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

func (f *fakeStore) UpdateCardDates(_ context.Context, cardID int64, startAt sql.NullTime, dueAt sql.NullTime, coverColor string) (int64, error) {
	f.datesCardID = cardID
	f.datesStart = startAt
	f.datesDue = dueAt
	f.datesCover = coverColor
	return 7, nil
}

func (f *fakeStore) UpdateCardTimeline(_ context.Context, cardID int64, startAt time.Time, dueAt time.Time) error {
	f.timelineCardID = cardID
	f.timelineStart = startAt
	f.timelineDue = dueAt
	return nil
}

func (f *fakeStore) SetCardComplete(context.Context, int64, bool) (int64, error) {
	return 0, nil
}

func (f *fakeStore) ArchiveCard(context.Context, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) RestoreCard(context.Context, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) DeleteCard(context.Context, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) CreateLabel(context.Context, int64, string, string) error {
	return nil
}

func (f *fakeStore) SetCardLabels(context.Context, int64, []int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) AddComment(context.Context, int64, string) (int64, error) {
	return 0, nil
}

func (f *fakeStore) AddAttachment(context.Context, int64, string, string) (int64, error) {
	return 0, nil
}

func (f *fakeStore) CreateChecklist(_ context.Context, cardID int64, _ string) (int64, error) {
	f.createdChecklistCardID = cardID
	return 7, nil
}

func (f *fakeStore) CreateChecklistItem(context.Context, int64, string) (int64, error) {
	return 7, nil
}

func (f *fakeStore) ToggleChecklistItem(_ context.Context, itemID int64, checked bool) (int64, bool, error) {
	f.toggledItemID = itemID
	f.toggledChecked = checked
	return 7, checked, nil
}

func (f *fakeStore) BoardIDForList(context.Context, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) BoardIDForCard(context.Context, int64) (int64, error) {
	return 7, nil
}

func (f *fakeStore) ReorderLists(_ context.Context, boardID int64, listIDs []int64) error {
	f.reorderBoardID = boardID
	f.reorderListIDs = append([]int64(nil), listIDs...)
	return nil
}

func (f *fakeStore) ReorderCards(context.Context, int64, []int64) error {
	return nil
}
