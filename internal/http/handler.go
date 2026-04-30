package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"golangkanban/internal/store"
	"golangkanban/internal/view"
)

type DataStore interface {
	ListBoards(context.Context) ([]store.Board, error)
	CreateBoard(context.Context, string) (store.Board, error)
	GetBoardDetail(context.Context, int64) (store.BoardDetail, error)
	RenameBoard(context.Context, int64, string) error
	DeleteBoard(context.Context, int64) error
	CreateList(context.Context, int64, string) (store.List, error)
	RenameList(context.Context, int64, string) (int64, error)
	DeleteList(context.Context, int64) (int64, error)
	CreateCard(context.Context, int64, string) (store.Card, error)
	UpdateCard(context.Context, int64, string, string) (int64, error)
	DeleteCard(context.Context, int64) (int64, error)
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
	mux.HandleFunc("POST /boards/{boardID}/rename", app.renameBoard)
	mux.HandleFunc("POST /boards/{boardID}/delete", app.deleteBoard)
	mux.HandleFunc("POST /boards/{boardID}/lists", app.createList)
	mux.HandleFunc("POST /lists/{listID}/rename", app.renameList)
	mux.HandleFunc("POST /lists/{listID}/delete", app.deleteList)
	mux.HandleFunc("POST /lists/{listID}/cards", app.createCard)
	mux.HandleFunc("POST /cards/{cardID}/update", app.updateCard)
	mux.HandleFunc("POST /cards/{cardID}/delete", app.deleteCard)
	mux.HandleFunc("PATCH /api/lists/reorder", app.reorderLists)
	mux.HandleFunc("PATCH /api/cards/reorder", app.reorderCards)

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
	detail, err := a.store.GetBoardDetail(r.Context(), boardID)
	if err != nil {
		handleHTMLStoreError(w, r, err, "/boards")
		return
	}
	render(w, r, view.BoardPage(detail, errorMessage(r.URL.Query().Get("error"))))
}

func (a *App) renameBoard(w http.ResponseWriter, r *http.Request) {
	boardID, ok := pathID(w, r, "boardID")
	if !ok {
		return
	}
	redirectURL := fmt.Sprintf("/boards/%d", boardID)
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
	redirectURL := fmt.Sprintf("/boards/%d", boardID)
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
	redirectURL := fmt.Sprintf("/boards/%d", boardID)
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
	http.Redirect(w, r, fmt.Sprintf("/boards/%d", boardID), http.StatusSeeOther)
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
	redirectURL := fmt.Sprintf("/boards/%d", boardID)
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
	redirectURL := fmt.Sprintf("/boards/%d", boardID)

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
	http.Redirect(w, r, fmt.Sprintf("/boards/%d", boardID), http.StatusSeeOther)
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
	case "invalid_order":
		return "並び順を保存できませんでした。ページを再読み込みしてください。"
	case "form_invalid", "invalid_input":
		return "入力内容を確認してください。"
	default:
		return ""
	}
}
