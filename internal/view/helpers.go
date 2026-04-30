package view

import (
	"fmt"
	"strconv"

	"github.com/a-h/templ"
)

func boardURL(id int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/boards/%d", id))
}

func boardAction(id int64, action string) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/boards/%d/%s", id, action))
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

func idString(id int64) string {
	return strconv.FormatInt(id, 10)
}

func listDialogID(id int64) string {
	return fmt.Sprintf("delete-list-%d", id)
}

func boardDialogID(id int64) string {
	return fmt.Sprintf("delete-board-%d", id)
}

func boardData(id int64) string {
	return fmt.Sprintf("kanbanBoard(%d)", id)
}
