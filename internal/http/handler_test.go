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
	"strconv"
	"strings"
	"testing"
	"time"

	"golangkanban/internal/store"
)

func TestRootRedirectsToBoards(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	New(&fakeStore{}).ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusSeeOther)
	if location := rec.Header().Get("Location"); location != "/boards" {
		t.Fatalf("location = %q, want /boards", location)
	}
}

func TestSecurityHeadersAndCSRFSessionOnGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/boards", nil)
	rec := httptest.NewRecorder()

	New(&fakeStore{}).ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing nosniff header")
	}
	if rec.Header().Get("Referrer-Policy") != "same-origin" {
		t.Fatalf("missing referrer policy")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing frame options")
	}
	if rec.Header().Get("Content-Security-Policy-Report-Only") == "" {
		t.Fatalf("missing report-only csp")
	}
	var session *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			session = cookie
			break
		}
	}
	if session == nil {
		t.Fatalf("missing %s cookie", sessionCookieName)
	}
	if !session.HttpOnly || session.SameSite != http.SameSiteStrictMode || session.Path != "/" {
		t.Fatalf("session cookie attributes = %#v", session)
	}
	body := rec.Body.String()
	assertContains(t, body, `<meta name="csrf-token" content="`, `name="_csrf"`)
}

func TestCSRFRejectsUnsafeRequests(t *testing.T) {
	tests := []struct {
		name  string
		setup func(http.Handler, *http.Request)
	}{
		{
			name:  "missing token",
			setup: func(http.Handler, *http.Request) {},
		},
		{
			name: "mismatched token",
			setup: func(handler http.Handler, req *http.Request) {
				authorizeUnsafeRequest(handler, req)
				req.Header.Set(csrfHeaderName, "wrong")
			},
		},
		{
			name: "unknown session",
			setup: func(_ http.Handler, req *http.Request) {
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "unknown"})
				req.Header.Set(csrfHeaderName, "token")
				req.Header.Set("Origin", "http://"+req.Host)
			},
		},
		{
			name: "cross origin",
			setup: func(handler http.Handler, req *http.Request) {
				authorizeUnsafeRequest(handler, req)
				req.Header.Set("Origin", "http://attacker.example")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := formRequest(http.MethodPost, "/boards", url.Values{"title": {"Blocked"}})
			rec := httptest.NewRecorder()
			fake := &fakeStore{}
			handler := New(fake)
			tt.setup(handler, req)

			handler.ServeHTTP(rec, req)

			assertStatus(t, rec, http.StatusForbidden)
			if fake.createdBoardTitle != "" {
				t.Fatalf("unsafe request reached store with title %q", fake.createdBoardTitle)
			}
		})
	}
}

func TestCSRFRejectsJSONWithJSONError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/lists/reorder", bytes.NewBufferString(`{"boardId":7,"listIds":[3]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	New(&fakeStore{}).ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusForbidden)
	assertContains(t, rec.Header().Get("Content-Type"), "application/json")
	assertContains(t, rec.Body.String(), `"error":"forbidden"`)
}

func TestCreateBoardRedirectsToCreatedBoard(t *testing.T) {
	fake := &fakeStore{}
	rec := serve(formRequest(http.MethodPost, "/boards", url.Values{"title": {"Roadmap"}}), fake)

	assertStatus(t, rec, http.StatusSeeOther)
	if location := rec.Header().Get("Location"); location != "/boards/7" {
		t.Fatalf("location = %q, want /boards/7", location)
	}
	if fake.createdBoardTitle != "Roadmap" {
		t.Fatalf("created title = %q, want Roadmap", fake.createdBoardTitle)
	}
}

func TestCSRFAllowsFormFieldToken(t *testing.T) {
	fake := &fakeStore{}
	handler := New(fake)
	prime := httptest.NewRequest(http.MethodGet, "/boards", nil)
	primeRec := httptest.NewRecorder()
	handler.ServeHTTP(primeRec, prime)
	token := csrfTokenFromBody(primeRec.Body.String())
	if token == "" {
		t.Fatal("missing csrf token")
	}
	var session *http.Cookie
	for _, cookie := range primeRec.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			session = cookie
			break
		}
	}
	if session == nil {
		t.Fatal("missing session cookie")
	}
	req := formRequest(http.MethodPost, "/boards", url.Values{"title": {"Roadmap"}, csrfFormField: {token}})
	req.AddCookie(session)
	req.Header.Set("Origin", "http://"+req.Host)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusSeeOther)
	if fake.createdBoardTitle != "Roadmap" {
		t.Fatalf("created title = %q, want Roadmap", fake.createdBoardTitle)
	}
}

func TestBoardHTMLDoesNotRenderUserContentAsHTML(t *testing.T) {
	payload := `<script>alert(1)</script><span hx-get="/pwn">x</span>`
	req := httptest.NewRequest(http.MethodGet, "/boards/7", nil)
	rec := httptest.NewRecorder()
	fake := &fakeStore{boardDetail: store.BoardDetail{
		Board: store.Board{ID: 7, Title: payload},
		Lists: []store.List{{
			ID:      3,
			BoardID: 7,
			Title:   payload,
			Cards: []store.Card{{
				ID:          9,
				ListID:      3,
				Title:       payload,
				Description: payload,
				Labels:      []store.Label{{ID: 5, BoardID: 7, Name: payload, Color: "red"}},
				Comments:    []store.Comment{{ID: 1, CardID: 9, Body: payload}},
				Checklists: []store.Checklist{{
					ID:     11,
					CardID: 9,
					Title:  payload,
					Items:  []store.ChecklistItem{{ID: 12, ChecklistID: 11, Title: payload}},
				}},
				Attachments: []store.Attachment{{ID: 2, CardID: 9, Title: "bad link", URL: "javascript:alert(1)"}},
			}},
		}},
		Labels: []store.Label{{ID: 5, BoardID: 7, Name: payload, Color: "red"}},
	}}

	New(fake).ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	if strings.Contains(body, payload) || strings.Contains(body, `<script>`) || strings.Contains(body, `hx-get="/pwn"`) || strings.Contains(body, `javascript:alert(1)`) {
		t.Fatalf("body rendered unsafe user content: %s", body)
	}
	assertContains(t, body, `&lt;script&gt;alert(1)&lt;/script&gt;`, `hx-get=&#34;/pwn&#34;`)
}

func formRequest(method string, path string, form url.Values) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func htmxPost(path string, form url.Values) *http.Request {
	req := formRequest(http.MethodPost, path, form)
	req.Header.Set("HX-Request", "true")
	return req
}

func serve(req *http.Request, fake *fakeStore) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler := New(fake)
	authorizeUnsafeRequest(handler, req)
	handler.ServeHTTP(rec, req)
	return rec
}

func authorizeUnsafeRequest(handler http.Handler, req *http.Request) {
	if safeMethod(req.Method) {
		return
	}
	prime := httptest.NewRequest(http.MethodGet, "/boards", nil)
	primeRec := httptest.NewRecorder()
	handler.ServeHTTP(primeRec, prime)
	token := csrfTokenFromBody(primeRec.Body.String())
	for _, cookie := range primeRec.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			req.AddCookie(cookie)
			break
		}
	}
	if token != "" {
		req.Header.Set(csrfHeaderName, token)
	}
	if req.Header.Get("Origin") == "" {
		req.Header.Set("Origin", "http://"+req.Host)
	}
}

func csrfTokenFromBody(body string) string {
	const marker = `<meta name="csrf-token" content="`
	start := strings.Index(body, marker)
	if start == -1 {
		return ""
	}
	start += len(marker)
	end := strings.Index(body[start:], `"`)
	if end == -1 {
		return ""
	}
	return body[start : start+end]
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d", rec.Code, status)
	}
}

func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q: %s", want, body)
		}
	}
}

func TestUpdateCardHTMXReturnsBodyFormAndTrigger(t *testing.T) {
	req := htmxPost("/cards/9/update", url.Values{"title": {"Updated"}, "description": {"Body"}})
	req.Header.Set("HX-Current-URL", "http://example.com/boards/7?q=launch")
	fake := &fakeStore{}
	rec := serve(req, fake)

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body, `data-card-body-form`, `hx-trigger="blur from:input, blur from:textarea"`, `hx-target="closest [data-card-body-form]"`, `name="title" value="Updated"`, `Body`)
	if strings.Contains(body, `data-card-detail`) {
		t.Fatalf("body contains full card detail article: %s", body)
	}
	if strings.Contains(body, `hx-swap-oob`) {
		t.Fatalf("body contains hx-swap-oob: %s", body)
	}
	assertContains(t, rec.Header().Get("HX-Trigger"), `cardUpdated`, `"boardId":7`, `"cardId":9`, `"source":"card-detail"`)
}

func TestUpdateCardHTMXErrorReturnsDetailArticleWithoutTrigger(t *testing.T) {
	fake := &fakeStore{boardDetail: testBoardDetail()}
	rec := serve(htmxPost("/cards/9/update", url.Values{"title": {""}, "description": {"Body"}}), fake)

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body, `id="card-9-detail"`, "カード名を入力してください。")
	if trigger := rec.Header().Get("HX-Trigger"); trigger != "" {
		t.Fatalf("HX-Trigger = %q, want empty", trigger)
	}
	if fake.updatedCardID != 0 {
		t.Fatalf("updated card id = %d, want 0", fake.updatedCardID)
	}
}

func TestUpdateCardHTMXArchiveReturnsBodyForm(t *testing.T) {
	req := htmxPost("/cards/9/update", url.Values{"title": {"Archived Updated"}, "description": {"Body"}})
	req.Header.Set("HX-Current-URL", "http://example.com/boards/7/archive")
	fake := &fakeStore{archiveDetail: store.ArchiveDetail{
		Board:  store.Board{ID: 7, Title: "Board"},
		Cards:  []store.Card{{ID: 9, ListID: 3, Title: "Archived Updated", Description: "Body"}},
		Labels: []store.Label{{ID: 5, BoardID: 7, Name: "Bug", Color: "red"}},
	}}
	rec := serve(req, fake)

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body, `data-card-body-form`, `name="title" value="Archived Updated"`, `Body`)
	if strings.Contains(body, `name="return_to" value="archive"`) {
		t.Fatalf("body contains archive actions: %s", body)
	}
	if strings.Contains(body, `hx-swap-oob`) {
		t.Fatalf("body contains hx-swap-oob: %s", body)
	}
	assertContains(t, rec.Header().Get("HX-Trigger"), `cardUpdated`)
}

func TestBoardFilterHTMXRendersListTarget(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/boards/7?q=launch", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	fake := &fakeStore{boardDetail: testBoardDetail()}

	New(fake).ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body, `id="board-dynamic"`, `hx-target="#board-dynamic"`, `id="board-lists"`)
	if fake.boardFilter.Query != "launch" {
		t.Fatalf("board filter query = %q, want launch", fake.boardFilter.Query)
	}
}

func TestBoardRefreshHTMXUsesCurrentURLFilter(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/boards/7", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Current-URL", "http://example.com/boards/7?q=launch")
	rec := httptest.NewRecorder()
	fake := &fakeStore{boardDetail: testBoardDetail()}

	New(fake).ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	if fake.boardFilter.Query != "launch" {
		t.Fatalf("board filter query = %q, want launch", fake.boardFilter.Query)
	}
}

func TestCardActivitiesHTMXRendersRequestedPage(t *testing.T) {
	detail := testBoardDetail()
	detail.Lists[0].Cards[0].Activities = activityEvents(12)
	req := httptest.NewRequest(http.MethodGet, "/cards/9/activities?page=2", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	fake := &fakeStore{boardDetail: detail}

	New(fake).ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body, `data-card-activity-section`, `2/2`, `Event 11`, `Event 12`, `hx-get="/cards/9/activities?page=1"`)
	if strings.Contains(body, "Event 10") {
		t.Fatalf("second activity page contains previous page event: %s", body)
	}
}

func TestReorderListsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/lists/reorder", bytes.NewBufferString(`{"boardId":7,"listIds":[3,1,2]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	handler := New(fake)
	authorizeUnsafeRequest(handler, req)
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusNoContent)
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

	handler := New(fake)
	authorizeUnsafeRequest(handler, req)
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
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

	handler := New(fake)
	authorizeUnsafeRequest(handler, req)
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	if fake.timelineBoardID != 7 {
		t.Fatalf("timeline board id = %d, want 7", fake.timelineBoardID)
	}
	assertContains(t, rec.Body.String(), "Timeline Board のタイムライン")
}

func TestTimelineFilterHTMXRendersTimelineTarget(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/boards/7/timeline?q=launch", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	handler := New(fake)
	authorizeUnsafeRequest(handler, req)
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	assertContains(t, body, `id="timeline-dynamic"`, `hx-target="#timeline-dynamic"`, `id="timeline-display"`)
	if fake.timelineFilter.Query != "launch" {
		t.Fatalf("timeline filter query = %q, want launch", fake.timelineFilter.Query)
	}
}

func TestTimelineRefreshHTMXUsesCurrentURLFilter(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/boards/7/timeline", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Current-URL", "http://example.com/boards/7/timeline?q=launch&from=2026-05-04&span=quarter")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	handler := New(fake)
	authorizeUnsafeRequest(handler, req)
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
	if fake.timelineFilter.Query != "launch" {
		t.Fatalf("timeline filter query = %q, want launch", fake.timelineFilter.Query)
	}
}

func TestUpdateCardTimelineJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "/api/cards/9/timeline", bytes.NewBufferString(`{"startAt":"2026-05-04","dueAt":"2026-05-08"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fake := &fakeStore{}

	handler := New(fake)
	authorizeUnsafeRequest(handler, req)
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusOK)
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

	handler := New(fake)
	authorizeUnsafeRequest(handler, req)
	handler.ServeHTTP(rec, req)

	assertStatus(t, rec, http.StatusBadRequest)
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
	fake := &fakeStore{boardDetail: testBoardDetail()}
	rec := serve(htmxPost("/cards/9/dates", form), fake)

	assertStatus(t, rec, http.StatusOK)
	if fake.datesCardID != 9 {
		t.Fatalf("card id = %d, want 9", fake.datesCardID)
	}
	assertContains(t, rec.Body.String(), `data-card-meta-section`)
	assertContains(t, rec.Header().Get("HX-Trigger"), `cardUpdated`, `"boardId":7`, `"cardId":9`)
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

func TestChecklistItemHTMXRoutesUsePathCardID(t *testing.T) {
	tests := []struct {
		name                    string
		path                    string
		form                    url.Values
		wantsCardUpdatedTrigger bool
		assert                  func(*testing.T, *fakeStore)
	}{
		{
			name:                    "create item",
			path:                    "/cards/9/checklists/11/items",
			form:                    url.Values{"title": {"Write tests"}},
			wantsCardUpdatedTrigger: true,
			assert: func(t *testing.T, fake *fakeStore) {
				t.Helper()
				if fake.createdChecklistItemID != 11 || fake.createdChecklistItemTitle != "Write tests" {
					t.Fatalf("created checklist item = %d %q, want 11 Write tests", fake.createdChecklistItemID, fake.createdChecklistItemTitle)
				}
			},
		},
		{
			name:                    "toggle item",
			path:                    "/cards/9/checklist-items/11/toggle",
			form:                    url.Values{"checked": {"true"}},
			wantsCardUpdatedTrigger: true,
			assert: func(t *testing.T, fake *fakeStore) {
				t.Helper()
				if fake.toggledItemID != 11 || !fake.toggledChecked {
					t.Fatalf("toggle = item %d checked %v, want item 11 checked true", fake.toggledItemID, fake.toggledChecked)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeStore{boardDetail: testBoardDetail()}
			rec := serve(htmxPost(tt.path, tt.form), fake)

			assertStatus(t, rec, http.StatusOK)
			assertContains(t, rec.Body.String(), `data-checklist-section`)
			if tt.wantsCardUpdatedTrigger {
				assertContains(t, rec.Header().Get("HX-Trigger"), `cardUpdated`)
			} else if got := rec.Header().Get("HX-Trigger"); got != "" {
				t.Fatalf("HX-Trigger = %q, want empty", got)
			}
			tt.assert(t, fake)
		})
	}
}

func TestListRenameHTMXReturnsBoardLists(t *testing.T) {
	fake := &fakeStore{boardDetail: testBoardDetail()}
	rec := serve(htmxPost("/lists/3/rename", url.Values{"title": {"Renamed"}}), fake)

	assertStatus(t, rec, http.StatusOK)
	if fake.renamedListID != 3 || fake.renamedListTitle != "Renamed" {
		t.Fatalf("renamed list = %d %q, want 3 Renamed", fake.renamedListID, fake.renamedListTitle)
	}
	assertContains(t, rec.Body.String(), `id="board-lists"`)
}

func TestCreateCardHTMXReturnsBoardWithDialogs(t *testing.T) {
	fake := &fakeStore{boardDetail: testBoardDetail()}
	rec := serve(htmxPost("/lists/3/cards", url.Values{"title": {"New Card"}}), fake)

	assertStatus(t, rec, http.StatusOK)
	if fake.createdCardListID != 3 || fake.createdCardTitle != "New Card" {
		t.Fatalf("created card = list %d title %q, want list 3 New Card", fake.createdCardListID, fake.createdCardTitle)
	}
	assertContains(t, rec.Body.String(), `id="board-dialogs"`, `id="card-9-detail"`)
}

func testBoardDetail() store.BoardDetail {
	return store.BoardDetail{
		Board: store.Board{ID: 7, Title: "Board"},
		Lists: []store.List{
			{
				ID:      3,
				BoardID: 7,
				Title:   "Doing",
				Cards: []store.Card{
					{ID: 9, ListID: 3, Title: "Updated", Description: "Body"},
				},
			},
		},
		Labels: []store.Label{{ID: 5, BoardID: 7, Name: "Bug", Color: "red"}},
	}
}

func activityEvents(count int) []store.ActivityEvent {
	events := make([]store.ActivityEvent, 0, count)
	base := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	for i := 1; i <= count; i++ {
		events = append(events, store.ActivityEvent{
			ID:        int64(i),
			BoardID:   7,
			CardID:    sql.NullInt64{Int64: 9, Valid: true},
			EventType: "test",
			Message:   "Event " + twoDigit(i),
			CreatedAt: base.Add(-time.Duration(i) * time.Minute),
		})
	}
	return events
}

func twoDigit(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

type fakeStore struct {
	createdBoardTitle         string
	createdChecklistCardID    int64
	createdChecklistItemID    int64
	createdChecklistItemTitle string
	boardDetail               store.BoardDetail
	archiveDetail             store.ArchiveDetail
	boardFilter               store.BoardFilter
	datesCardID               int64
	datesStart                sql.NullTime
	datesDue                  sql.NullTime
	datesCover                string
	updatedCardID             int64
	updatedCardTitle          string
	updatedCardDescription    string
	renamedListID             int64
	renamedListTitle          string
	createdCardListID         int64
	createdCardTitle          string
	reorderBoardID            int64
	reorderListIDs            []int64
	timelineBoardID           int64
	timelineFilter            store.BoardFilter
	timelineCardID            int64
	timelineStart             time.Time
	timelineDue               time.Time
	toggledItemID             int64
	toggledChecked            bool
}

func (f *fakeStore) ListBoards(context.Context) ([]store.Board, error) {
	return nil, nil
}

func (f *fakeStore) CreateBoard(_ context.Context, title string) (store.Board, error) {
	f.createdBoardTitle = title
	return store.Board{ID: 7, Title: title}, nil
}

func (f *fakeStore) GetBoardDetail(_ context.Context, boardID int64, filter store.BoardFilter) (store.BoardDetail, error) {
	f.boardFilter = filter
	if f.boardDetail.Board.ID == 0 {
		return store.BoardDetail{}, store.ErrNotFound
	}
	f.boardDetail.Board.ID = boardID
	f.boardDetail.Filter = filter
	return f.boardDetail, nil
}

func (f *fakeStore) GetArchiveDetail(_ context.Context, boardID int64) (store.ArchiveDetail, error) {
	if f.archiveDetail.Board.ID != 0 {
		f.archiveDetail.Board.ID = boardID
		return f.archiveDetail, nil
	}
	return store.ArchiveDetail{}, store.ErrNotFound
}

func (f *fakeStore) GetTimelineDetail(_ context.Context, boardID int64, options store.TimelineOptions) (store.TimelineDetail, error) {
	f.timelineBoardID = boardID
	f.timelineFilter = options.Filter
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

func (f *fakeStore) RenameList(_ context.Context, listID int64, title string) (int64, error) {
	f.renamedListID = listID
	f.renamedListTitle = title
	return 7, nil
}

func (f *fakeStore) DeleteList(context.Context, int64) (int64, error) {
	return 0, nil
}

func (f *fakeStore) CreateCard(_ context.Context, listID int64, title string) (store.Card, error) {
	f.createdCardListID = listID
	f.createdCardTitle = title
	return store.Card{ID: 10, ListID: listID, Title: title}, nil
}

func (f *fakeStore) UpdateCard(_ context.Context, cardID int64, title string, description string) (int64, error) {
	f.updatedCardID = cardID
	f.updatedCardTitle = title
	f.updatedCardDescription = description
	return 7, nil
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

func (f *fakeStore) CreateChecklistItem(_ context.Context, checklistID int64, title string) (int64, error) {
	f.createdChecklistItemID = checklistID
	f.createdChecklistItemTitle = title
	return 7, nil
}

func (f *fakeStore) ToggleChecklistItem(_ context.Context, itemID int64, checked bool) (int64, bool, error) {
	f.toggledItemID = itemID
	f.toggledChecked = checked
	return 7, checked, nil
}

func (f *fakeStore) BoardIDForList(context.Context, int64) (int64, error) {
	return 7, nil
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
