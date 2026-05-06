package view

import (
	"context"
	"database/sql"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/a-h/templ"

	"golangkanban/internal/store"
)

func TestBoardDynamicRendersListsAndDialogs(t *testing.T) {
	card := store.Card{ID: 9, ListID: 3, Title: "Write templ tests", Description: "DOM assertions"}
	detail := store.BoardDetail{
		Board: store.Board{ID: 7, Title: "Roadmap"},
		Labels: []store.Label{
			{ID: 5, BoardID: 7, Name: "Testing", Color: "green"},
		},
		Lists: []store.List{{
			ID:      3,
			BoardID: 7,
			Title:   "Doing",
			Cards:   []store.Card{card},
		}},
	}

	doc := renderComponent(t, BoardDynamic(detail))

	assertSelectionLength(t, doc, "#board-dynamic", 1)
	assertSelectionLength(t, doc, "#board-lists", 1)
	assertSelectionLength(t, doc, "#board-dialogs", 1)
	assertSelectionLength(t, doc, `[data-card-id="9"]`, 1)
	assertSelectionLength(t, doc, `[data-card-id="9"] button[x-sort\:handle][aria-label="カードをドラッグして移動"]`, 1)
	assertSelectionLength(t, doc, `[data-card-detail]`, 1)
	if title := strings.TrimSpace(doc.Find(`[data-card-id="9"] button span.text-sm`).Text()); title != "Write templ tests" {
		t.Fatalf("card title = %q, want %q", title, "Write templ tests")
	}
}

func TestCardChecklistSectionRendersItemsAndHTMXTargets(t *testing.T) {
	checkedAt := time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)
	card := store.Card{
		ID:                 9,
		Title:              "Launch checklist",
		ChecklistTotal:     2,
		ChecklistCompleted: 1,
		Checklists: []store.Checklist{{
			ID:             11,
			CardID:         9,
			Title:          "Release",
			CompletedCount: 1,
			TotalCount:     2,
			Items: []store.ChecklistItem{
				{ID: 21, ChecklistID: 11, Title: "Write tests", CheckedAt: sql.NullTime{Time: checkedAt, Valid: true}},
				{ID: 22, ChecklistID: 11, Title: "Ship it"},
			},
		}},
	}

	doc := renderComponent(t, CardChecklistSection(card))

	assertSelectionLength(t, doc, `[data-checklist-section]`, 1)
	assertTextContains(t, doc.Find(`[data-checklist-section]`).Text(), "チェックリスト", "Release", "1/2", "Write tests", "Ship it")
	assertSelectionLength(t, doc, `form[hx-post="/cards/9/checklist-items/21/toggle"][hx-target="closest [data-checklist-section]"]`, 1)
	assertSelectionLength(t, doc, `form[hx-post="/cards/9/checklist-items/22/toggle"][hx-target="closest [data-checklist-section]"]`, 1)
	assertSelectionLength(t, doc, `form[hx-post="/cards/9/checklists/11/items"][hx-target="closest [data-checklist-section]"]`, 1)
	assertSelectionLength(t, doc, `form[hx-post="/cards/9/checklists"][hx-target="closest [data-checklist-section]"]`, 1)
	assertSelectionLength(t, doc, `button[aria-label="未完了に戻す"]`, 1)
	assertSelectionLength(t, doc, `button[aria-label="完了にする"]`, 1)
}

func TestCardMetaSectionRendersDatesCoverAndActions(t *testing.T) {
	startAt := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	dueAt := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	card := store.Card{
		ID:         9,
		Title:      "Timeline card",
		StartAt:    sql.NullTime{Time: startAt, Valid: true},
		DueAt:      sql.NullTime{Time: dueAt, Valid: true},
		CoverColor: "blue",
	}

	doc := renderComponent(t, CardMetaSection(card, false))

	assertSelectionLength(t, doc, `[data-card-meta-section]`, 1)
	assertSelectionLength(t, doc, `form[hx-post="/cards/9/dates"][hx-target="closest [data-card-meta-section]"]`, 1)
	assertInputValue(t, doc, `input[name="start_at"]`, "2026-05-04")
	assertInputValue(t, doc, `input[name="due_at"]`, "2026-05-08")
	assertSelectionLength(t, doc, `select[name="cover_color"] option[value="blue"][selected]`, 1)
	assertTextContains(t, doc.Find(`[data-card-meta-section]`).Text(), "日付: 2026-05-04 - 2026-05-08", "完了にする", "アーカイブ")
	assertSelectionLength(t, doc, `form[action="/cards/9/restore"]`, 0)
}

func TestCardBodyFormAutosavesOnBlur(t *testing.T) {
	card := store.Card{ID: 9, Title: "Autosave title", Description: "Autosave body"}

	doc := renderComponent(t, CardBodyForm(card))

	assertSelectionLength(t, doc, `form[data-card-body-form][hx-post="/cards/9/update"][hx-trigger="blur from:input, blur from:textarea"][hx-target="closest [data-card-body-form]"][hx-swap="outerHTML"]`, 1)
	assertInputValue(t, doc, `input[name="title"]`, "Autosave title")
	assertSelectionLength(t, doc, `textarea[name="description"]`, 1)
	assertTextContains(t, doc.Find(`textarea[name="description"]`).Text(), "Autosave body")
	assertSelectionLength(t, doc, `button[type="submit"]`, 0)
}

func TestCardLabelsSectionRendersFormAndTargets(t *testing.T) {
	card := store.Card{
		ID:    9,
		Title: "Label card",
		Labels: []store.Label{
			{ID: 5, BoardID: 7, Name: "Testing", Color: "green"},
		},
	}
	labels := []store.Label{
		{ID: 5, BoardID: 7, Name: "Testing", Color: "green"},
		{ID: 6, BoardID: 7, Name: "Backend", Color: "blue"},
	}

	doc := renderComponent(t, CardLabelsSection(card, labels))

	assertSelectionLength(t, doc, `[data-card-labels-section]`, 1)
	assertSelectionLength(t, doc, `form[hx-post="/cards/9/labels"][hx-target="closest [data-card-labels-section]"]`, 1)
	assertSelectionLength(t, doc, `input[name="label_id"][value="5"][checked]`, 1)
	assertSelectionLength(t, doc, `input[name="label_id"][value="6"]`, 1)
	assertTextContains(t, doc.Find(`[data-card-labels-section]`).Text(), "ラベル", "Testing", "Backend")
}

func TestCardCommentsSectionRendersFormAndCommentList(t *testing.T) {
	card := store.Card{
		ID:    9,
		Title: "Comment card",
		Comments: []store.Comment{
			{
				ID:        14,
				CardID:    9,
				Body:      "Looks good",
				CreatedAt: time.Date(2026, 5, 6, 11, 30, 0, 0, time.UTC),
			},
		},
	}

	doc := renderComponent(t, CardCommentsSection(card))

	assertSelectionLength(t, doc, `[data-card-comments-section]`, 1)
	assertSelectionLength(t, doc, `form[hx-post="/cards/9/comments"][hx-target="closest [data-card-comments-section]"]`, 1)
	assertSelectionLength(t, doc, `textarea[name="body"]`, 1)
	assertTextContains(t, doc.Find(`[data-card-comments-section]`).Text(), "コメント", "Looks good")
}

func TestCardAttachmentsSectionRendersFormAndLinks(t *testing.T) {
	card := store.Card{
		ID:    9,
		Title: "Attachment card",
		Attachments: []store.Attachment{
			{ID: 30, CardID: 9, Title: "Spec", URL: "https://example.com/spec"},
		},
	}

	doc := renderComponent(t, CardAttachmentsSection(card))

	assertSelectionLength(t, doc, `[data-card-attachments-section]`, 1)
	assertSelectionLength(t, doc, `form[hx-post="/cards/9/attachments"][hx-target="closest [data-card-attachments-section]"]`, 1)
	assertSelectionLength(t, doc, `input[name="title"]`, 1)
	assertSelectionLength(t, doc, `input[name="url"]`, 1)
	assertSelectionLength(t, doc, `a[href="https://example.com/spec"]`, 1)
	assertTextContains(t, doc.Find(`[data-card-attachments-section]`).Text(), "添付リンク", "Spec")
}

func renderComponent(t *testing.T, component templ.Component) *goquery.Document {
	t.Helper()

	r, w := io.Pipe()
	errc := make(chan error, 1)
	go func() {
		err := component.Render(WithCSRFToken(context.Background(), "test-csrf-token"), w)
		if err != nil {
			_ = w.CloseWithError(err)
		} else {
			_ = w.Close()
		}
		errc <- err
	}()

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		t.Fatalf("failed to parse rendered component: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("failed to render component: %v", err)
	}
	return doc
}

func assertSelectionLength(t *testing.T, doc *goquery.Document, selector string, want int) {
	t.Helper()
	if got := doc.Find(selector).Length(); got != want {
		t.Fatalf("selector %q length = %d, want %d", selector, got, want)
	}
}

func assertInputValue(t *testing.T, doc *goquery.Document, selector string, want string) {
	t.Helper()
	value, ok := doc.Find(selector).Attr("value")
	if !ok {
		t.Fatalf("selector %q has no value attribute", selector)
	}
	if value != want {
		t.Fatalf("selector %q value = %q, want %q", selector, value, want)
	}
}

func assertTextContains(t *testing.T, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q does not contain %q", text, want)
		}
	}
}
