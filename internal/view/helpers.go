package view

import (
	"fmt"
	"strconv"
	"time"

	"github.com/a-h/templ"

	"golangkanban/internal/store"
)

func boardURL(id int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/boards/%d", id))
}

func boardAction(id int64, action string) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/boards/%d/%s", id, action))
}

func boardArchiveURL(id int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/boards/%d/archive", id))
}

func listAction(id int64, action string) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/lists/%d/%s", id, action))
}

func listCardsAction(id int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/lists/%d/cards", id))
}

func cardAction(id int64, action string) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/cards/%d/%s", id, action))
}

func checklistItemsAction(id int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/checklists/%d/items", id))
}

func checklistToggleAction(id int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/checklist-items/%d/toggle", id))
}

func idString(id int64) string {
	return strconv.FormatInt(id, 10)
}

func listDialogID(id int64) string {
	return fmt.Sprintf("delete-list-%d", id)
}

func boardDialogID(id int64) string {
	return fmt.Sprintf("delete-board-%d", id)
}

func cardDialogID(id int64) string {
	return fmt.Sprintf("card-%d", id)
}

func cardDeleteDialogID(id int64) string {
	return fmt.Sprintf("delete-card-%d", id)
}

func boardData(id int64) string {
	return fmt.Sprintf("kanbanBoard(%d)", id)
}

func filterLabelValue(filter store.BoardFilter) string {
	if filter.Label == 0 {
		return ""
	}
	return idString(filter.Label)
}

func labelChecked(card store.Card, labelID int64) bool {
	for _, label := range card.Labels {
		if label.ID == labelID {
			return true
		}
	}
	return false
}

func dueDateInput(card store.Card) string {
	if !card.DueAt.Valid {
		return ""
	}
	return card.DueAt.Time.Format("2006-01-02")
}

func dueDateText(card store.Card) string {
	if !card.DueAt.Valid {
		return ""
	}
	return card.DueAt.Time.Format("2006-01-02")
}

func dateTimeText(value time.Time) string {
	return value.Format("2006-01-02 15:04")
}

func dueBadgeClass(card store.Card) string {
	base := "rounded px-2 py-0.5 text-xs tabular-nums "
	if !card.DueAt.Valid {
		return base + "bg-zinc-100 text-zinc-600"
	}
	if card.CompletedAt.Valid {
		return base + "bg-green-100 text-green-700"
	}
	if card.DueAt.Time.Before(time.Now()) {
		return base + "bg-red-100 text-red-700"
	}
	return base + "bg-yellow-100 text-yellow-700"
}

func completionText(card store.Card) string {
	if card.CompletedAt.Valid {
		return "完了"
	}
	return "未完了"
}

func checklistProgress(card store.Card) string {
	if card.ChecklistTotal == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", card.ChecklistCompleted, card.ChecklistTotal)
}

func checklistPercent(checklist store.Checklist) int {
	if checklist.TotalCount == 0 {
		return 0
	}
	return checklist.CompletedCount * 100 / checklist.TotalCount
}

func labelClass(color string) string {
	switch color {
	case "green":
		return "bg-green-500 text-white"
	case "yellow":
		return "bg-yellow-400 text-zinc-950"
	case "orange":
		return "bg-orange-500 text-white"
	case "red":
		return "bg-red-500 text-white"
	case "blue":
		return "bg-blue-500 text-white"
	case "purple":
		return "bg-purple-500 text-white"
	case "pink":
		return "bg-pink-500 text-white"
	default:
		return "bg-zinc-500 text-white"
	}
}

func labelDotClass(color string) string {
	switch color {
	case "green":
		return "bg-green-500"
	case "yellow":
		return "bg-yellow-400"
	case "orange":
		return "bg-orange-500"
	case "red":
		return "bg-red-500"
	case "blue":
		return "bg-blue-500"
	case "purple":
		return "bg-purple-500"
	case "pink":
		return "bg-pink-500"
	default:
		return "bg-zinc-500"
	}
}

func coverClass(color string) string {
	switch color {
	case "green":
		return "bg-green-500"
	case "yellow":
		return "bg-yellow-400"
	case "orange":
		return "bg-orange-500"
	case "red":
		return "bg-red-500"
	case "blue":
		return "bg-blue-500"
	case "purple":
		return "bg-purple-500"
	case "pink":
		return "bg-pink-500"
	case "gray":
		return "bg-zinc-500"
	default:
		return ""
	}
}
