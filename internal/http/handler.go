package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"golangkanban/internal/store"
	"golangkanban/internal/view"
)

type DataStore interface {
	ListBoards(context.Context) ([]store.Board, error)
	CreateBoard(context.Context, string) (store.Board, error)
	GetBoardDetail(context.Context, int64, store.BoardFilter) (store.BoardDetail, error)
	GetArchiveDetail(context.Context, int64) (store.ArchiveDetail, error)
	RenameBoard(context.Context, int64, string) error
	DeleteBoard(context.Context, int64) error
	CreateList(context.Context, int64, string) (store.List, error)
	RenameList(context.Context, int64, string) (int64, error)
	DeleteList(context.Context, int64) (int64, error)
	CreateCard(context.Context, int64, string) (store.Card, error)
	UpdateCard(context.Context, int64, string, string) (int64, error)
	UpdateCardDates(context.Context, int64, sql.NullTime, string) (int64, error)
	SetCardComplete(context.Context, int64, bool) (int64, error)
	ArchiveCard(context.Context, int64) (int64, error)
	RestoreCard(context.Context, int64) (int64, error)
	DeleteCard(context.Context, int64) (int64, error)
	CreateLabel(context.Context, int64, string, string) error
	SetCardLabels(context.Context, int64, []int64) (int64, error)
	AddComment(context.Context, int64, string) (int64, error)
	AddAttachment(context.Context, int64, string, string) (int64, error)
	CreateChecklist(context.Context, int64, string) (int64, error)
	CreateChecklistItem(context.Context, int64, string) (int64, error)
	ToggleChecklistItem(context.Context, int64, bool) (int64, bool, error)
	BoardIDForList(context.Context, int64) (int64, error)
	BoardIDForCard(context.Context, int64) (int64, error)
	ReorderLists(context.Context, int64, []int64) error
	ReorderCards(context.Context, int64, []int64) error
}

type App struct {
	store DataStore
}

func New(data DataStore) http.Handler {
	app := &App{store: data}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", app.redirectRoot)
	mux.HandleFunc("GET /boards", app.boardsIndex)
	mux.HandleFunc("POST /boards", app.createBoard)
	mux.HandleFunc("GET /boards/{boardID}", app.boardShow)
	mux.HandleFunc("GET /boards/{boardID}/archive", app.boardArchive)
	mux.HandleFunc("POST /boards/{boardID}/rename", app.renameBoard)
	mux.HandleFunc("POST /boards/{boardID}/delete", app.deleteBoard)
	mux.HandleFunc("POST /boards/{boardID}/lists", app.createList)
	mux.HandleFunc("POST /boards/{boardID}/labels", app.createLabel)
	mux.HandleFunc("POST /lists/{listID}/rename", app.renameList)
	mux.HandleFunc("POST /lists/{listID}/delete", app.deleteList)
	mux.HandleFunc("POST /lists/{listID}/cards", app.createCard)
	mux.HandleFunc("POST /cards/{cardID}/update", app.updateCard)
	mux.HandleFunc("POST /cards/{cardID}/dates", app.updateCardDates)
	mux.HandleFunc("POST /cards/{cardID}/complete", app.completeCard)
	mux.HandleFunc("POST /cards/{cardID}/archive", app.archiveCard)
	mux.HandleFunc("POST /cards/{cardID}/restore", app.restoreCard)
	mux.HandleFunc("POST /cards/{cardID}/delete", app.deleteCard)
	mux.HandleFunc("POST /cards/{cardID}/labels", app.setCardLabels)
	mux.HandleFunc("POST /cards/{cardID}/comments", app.addComment)
	mux.HandleFunc("POST /cards/{cardID}/attachments", app.addAttachment)
	mux.HandleFunc("POST /cards/{cardID}/checklists", app.createChecklist)
	mux.HandleFunc("POST /checklists/{checklistID}/items", app.createChecklistItem)
	mux.HandleFunc("POST /checklist-items/{itemID}/toggle", app.toggleChecklistItem)
	mux.HandleFunc("PATCH /api/lists/reorder", app.reorderLists)
	mux.HandleFunc("PATCH /api/cards/reorder", app.reorderCards)
	mux.HandleFunc("PATCH /api/checklist-items/{itemID}", app.toggleChecklistItemAPI)

	return mux
}

func (a *App) redirectRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/boards", http.StatusSeeOther)
}

func (a *App) boardsIndex(w http.ResponseWriter, r *http.Request) {
	boards, err := a.store.ListBoards(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, view.BoardsPage(boards, errorMessage(r.URL.Query().Get("error"))))
}

func (a *App) createBoard(w http.ResponseWriter, r *http.Request) {
	title, ok := formTitle(w, r)
	if !ok {
		return
	}
	if title == "" {
		redirectWithError(w, r, "/boards", "board_title_required")
		return
	}

	board, err := a.store.CreateBoard(r.Context(), title)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/boards/%d", board.ID), http.StatusSeeOther)
}

func (a *App) boardShow(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "boardID")
	if !ok {
		return
	}
	detail, err := a.store.GetBoardDetail(r.Context(), boardID, filterFromQuery(r))
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	render(w, r, view.BoardPage(detail, errorMessage(r.URL.Query().Get("error"))))
}

func (a *App) boardArchive(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "boardID")
	if !ok {
		return
	}
	detail, err := a.store.GetArchiveDetail(r.Context(), boardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	render(w, r, view.ArchivePage(detail, errorMessage(r.URL.Query().Get("error"))))
}

func (a *App) renameBoard(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "boardID")
	if !ok {
		return
	}
	redirectURL := boardURL(boardID)
	title, ok := formTitle(w, r)
	if !ok {
		return
	}
	if title == "" {
		redirectWithError(w, r, redirectURL, "board_title_required")
		return
	}
	if err := a.store.RenameBoard(r.Context(), boardID, title); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) deleteBoard(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "boardID")
	if !ok {
		return
	}
	if err := a.store.DeleteBoard(r.Context(), boardID); err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	http.Redirect(w, r, "/boards", http.StatusSeeOther)
}

func (a *App) createList(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "boardID")
	if !ok {
		return
	}
	redirectURL := boardURL(boardID)
	title, ok := formTitle(w, r)
	if !ok {
		return
	}
	if title == "" {
		redirectWithError(w, r, redirectURL, "list_title_required")
		return
	}
	if _, err := a.store.CreateList(r.Context(), boardID, title); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) createLabel(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "boardID")
	if !ok {
		return
	}
	redirectURL := boardURL(boardID)
	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, redirectURL, "form_invalid")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	color := strings.TrimSpace(r.FormValue("color"))
	if name == "" || color == "" {
		redirectWithError(w, r, redirectURL, "label_required")
		return
	}
	if err := a.store.CreateLabel(r.Context(), boardID, name, color); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) renameList(w http.ResponseWriter, r *http.Request) {
	listID, ok := pathID(w, r, "listID")
	if !ok {
		return
	}
	boardID, err := a.store.BoardIDForList(r.Context(), listID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	redirectURL := boardURL(boardID)
	title, ok := formTitle(w, r)
	if !ok {
		return
	}
	if title == "" {
		redirectWithError(w, r, redirectURL, "list_title_required")
		return
	}
	if _, err := a.store.RenameList(r.Context(), listID, title); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) deleteList(w http.ResponseWriter, r *http.Request) {
	listID, ok := pathID(w, r, "listID")
	if !ok {
		return
	}
	boardID, err := a.store.DeleteList(r.Context(), listID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	http.Redirect(w, r, boardURL(boardID), http.StatusSeeOther)
}

func (a *App) createCard(w http.ResponseWriter, r *http.Request) {
	listID, ok := pathID(w, r, "listID")
	if !ok {
		return
	}
	boardID, err := a.store.BoardIDForList(r.Context(), listID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	redirectURL := boardURL(boardID)
	title, ok := formTitle(w, r)
	if !ok {
		return
	}
	if title == "" {
		redirectWithError(w, r, redirectURL, "card_title_required")
		return
	}
	if _, err := a.store.CreateCard(r.Context(), listID, title); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) updateCard(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.BoardIDForCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	redirectURL := boardURL(boardID)

	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, redirectURL, "form_invalid")
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	if title == "" {
		redirectWithError(w, r, redirectURL, "card_title_required")
		return
	}
	if _, err := a.store.UpdateCard(r.Context(), cardID, title, description); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) updateCardDates(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.BoardIDForCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	redirectURL := boardURL(boardID)
	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, redirectURL, "form_invalid")
		return
	}
	dueAt, ok := parseDueDate(r.FormValue("due_at"))
	if !ok {
		redirectWithError(w, r, redirectURL, "due_invalid")
		return
	}
	if _, err := a.store.UpdateCardDates(r.Context(), cardID, dueAt, r.FormValue("cover_color")); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) completeCard(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.BoardIDForCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	redirectURL := boardURL(boardID)
	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, redirectURL, "form_invalid")
		return
	}
	if _, err := a.store.SetCardComplete(r.Context(), cardID, parseBool(r.FormValue("complete"))); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) archiveCard(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.ArchiveCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	http.Redirect(w, r, boardURL(boardID), http.StatusSeeOther)
}

func (a *App) restoreCard(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.RestoreCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	if err := r.ParseForm(); err == nil && r.FormValue("return_to") == "archive" {
		http.Redirect(w, r, archiveURL(boardID), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, boardURL(boardID), http.StatusSeeOther)
}

func (a *App) deleteCard(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.DeleteCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	if err := r.ParseForm(); err == nil && r.FormValue("return_to") == "archive" {
		http.Redirect(w, r, archiveURL(boardID), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, boardURL(boardID), http.StatusSeeOther)
}

func (a *App) setCardLabels(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.BoardIDForCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	redirectURL := boardURL(boardID)
	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, redirectURL, "form_invalid")
		return
	}
	labelIDs, ok := parseInt64List(r.Form["label_id"])
	if !ok {
		redirectWithError(w, r, redirectURL, "invalid_input")
		return
	}
	if _, err := a.store.SetCardLabels(r.Context(), cardID, labelIDs); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) addComment(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.BoardIDForCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	redirectURL := boardURL(boardID)
	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, redirectURL, "form_invalid")
		return
	}
	if _, err := a.store.AddComment(r.Context(), cardID, r.FormValue("body")); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) addAttachment(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.BoardIDForCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	redirectURL := boardURL(boardID)
	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, redirectURL, "form_invalid")
		return
	}
	if _, err := a.store.AddAttachment(r.Context(), cardID, r.FormValue("title"), r.FormValue("url")); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) createChecklist(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "cardID")
	if !ok {
		return
	}
	boardID, err := a.store.BoardIDForCard(r.Context(), cardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	redirectURL := boardURL(boardID)
	title, ok := formTitle(w, r)
	if !ok {
		return
	}
	if title == "" {
		redirectWithError(w, r, redirectURL, "checklist_required")
		return
	}
	if _, err := a.store.CreateChecklist(r.Context(), cardID, title); err != nil {
		handleHTMLStoreError(w, r, err, redirectURL)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (a *App) createChecklistItem(w http.ResponseWriter, r *http.Request) {
	checklistID, ok := pathID(w, r, "checklistID")
	if !ok {
		return
	}
	title, ok := formTitle(w, r)
	if !ok {
		return
	}
	boardID, err := a.store.CreateChecklistItem(r.Context(), checklistID, title)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	http.Redirect(w, r, boardURL(boardID), http.StatusSeeOther)
}

func (a *App) toggleChecklistItem(w http.ResponseWriter, r *http.Request) {
	itemID, ok := pathID(w, r, "itemID")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		redirectWithError(w, r, "/boards", "form_invalid")
		return
	}
	boardID, _, err := a.store.ToggleChecklistItem(r.Context(), itemID, parseBool(r.FormValue("checked")))
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	http.Redirect(w, r, boardURL(boardID), http.StatusSeeOther)
}

func (a *App) reorderLists(w http.ResponseWriter, r *http.Request) {
	var request struct {
		BoardID int64   `json:"boardId"`
		ListIDs []int64 `json:"listIds"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.BoardID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid board id")
		return
	}
	if err := a.store.ReorderLists(r.Context(), request.BoardID, request.ListIDs); err != nil {
		handleJSONStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) reorderCards(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ToListID int64   `json:"toListId"`
		CardIDs  []int64 `json:"cardIds"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.ToListID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid list id")
		return
	}
	if err := a.store.ReorderCards(r.Context(), request.ToListID, request.CardIDs); err != nil {
		handleJSONStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) toggleChecklistItemAPI(w http.ResponseWriter, r *http.Request) {
	itemID, ok := pathID(w, r, "itemID")
	if !ok {
		return
	}
	var request struct {
		Checked bool `json:"checked"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	_, checked, err := a.store.ToggleChecklistItem(r.Context(), itemID, request.Checked)
	if err != nil {
		handleJSONStoreError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]bool{"checked": checked})
}

func render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		serverError(w, err)
	}
}

func formTitle(w http.ResponseWriter, r *http.Request) (string, bool) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return "", false
	}
	return strings.TrimSpace(r.FormValue("title")), true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}

func filterFromQuery(r *http.Request) store.BoardFilter {
	values := r.URL.Query()
	labelID, _ := strconv.ParseInt(values.Get("label"), 10, 64)
	return store.BoardFilter{
		Query:  values.Get("q"),
		Label:  labelID,
		Due:    values.Get("due"),
		Status: values.Get("status"),
	}
}

func parseDueDate(value string) (sql.NullTime, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullTime{}, true
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return sql.NullTime{}, false
	}
	return sql.NullTime{Time: parsed, Valid: true}, true
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseInt64List(values []string) ([]int64, bool) {
	var ids []int64
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func boardURL(boardID int64) string {
	return fmt.Sprintf("/boards/%d", boardID)
}

func archiveURL(boardID int64) string {
	return fmt.Sprintf("/boards/%d/archive", boardID)
}

func redirectWithError(w http.ResponseWriter, r *http.Request, path string, code string) {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	http.Redirect(w, r, path+separator+"error="+code, http.StatusSeeOther)
}

func handleHTMLStoreError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	switch {
	case errors.Is(err, store.ErrInvalidInput):
		redirectWithError(w, r, fallback, "invalid_input")
	case errors.Is(err, store.ErrInvalidOrder):
		redirectWithError(w, r, fallback, "invalid_order")
	case errors.Is(err, store.ErrNotFound):
		http.NotFound(w, r)
	default:
		serverError(w, err)
	}
}

func handleJSONStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrInvalidInput), errors.Is(err, store.ErrInvalidOrder):
		writeJSONError(w, http.StatusBadRequest, "invalid request")
	case errors.Is(err, store.ErrNotFound):
		writeJSONError(w, http.StatusNotFound, "not found")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func serverError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func errorMessage(code string) string {
	switch code {
	case "board_title_required":
		return "ボード名を入力してください。"
	case "list_title_required":
		return "リスト名を入力してください。"
	case "card_title_required":
		return "カード名を入力してください。"
	case "label_required":
		return "ラベル名と色を入力してください。"
	case "comment_required":
		return "コメントを入力してください。"
	case "attachment_required":
		return "添付リンクの名前と URL を入力してください。"
	case "checklist_required":
		return "チェックリスト名を入力してください。"
	case "due_invalid":
		return "期限の日付を確認してください。"
	case "invalid_order":
		return "並び順を保存できませんでした。ページを再読み込みしてください。"
	case "form_invalid", "invalid_input":
		return "入力内容を確認してください。"
	default:
		return ""
	}
}
