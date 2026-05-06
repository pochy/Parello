package view

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/a-h/templ"

	"golangkanban/internal/store"
)

type csrfContextKey struct{}

func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfContextKey{}, token)
}

func CSRFToken(ctx context.Context) string {
	token, _ := ctx.Value(csrfContextKey{}).(string)
	return token
}

func csrfToken(ctx context.Context) string {
	return CSRFToken(ctx)
}

func boardURL(id int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/boards/%d", id))
}

func boardAction(id int64, action string) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/boards/%d/%s", id, action))
}

func boardArchiveURL(id int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/boards/%d/archive", id))
}

func boardTimelineURL(id int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/boards/%d/timeline", id))
}

func timelineURL(id int64, from time.Time, span string, filter store.BoardFilter) templ.SafeURL {
	values := url.Values{}
	values.Set("from", from.Format("2006-01-02"))
	if span != "" {
		values.Set("span", span)
	}
	if filter.Query != "" {
		values.Set("q", filter.Query)
	}
	if filter.Label > 0 {
		values.Set("label", idString(filter.Label))
	}
	if filter.Due != "" {
		values.Set("due", filter.Due)
	}
	if filter.Status != "" {
		values.Set("status", filter.Status)
	}
	return templ.SafeURL(fmt.Sprintf("/boards/%d/timeline?%s", id, values.Encode()))
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

func cardActivitiesURL(id int64, page int) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/cards/%d/activities?page=%d", id, normalizedActivityPage(page)))
}

func attachmentURL(rawURL string) (templ.SafeURL, bool) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false
	}
	return templ.SafeURL(rawURL), true
}

func checklistItemsAction(cardID int64, checklistID int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/cards/%d/checklists/%d/items", cardID, checklistID))
}

func checklistToggleAction(cardID int64, itemID int64) templ.SafeURL {
	return templ.SafeURL(fmt.Sprintf("/cards/%d/checklist-items/%d/toggle", cardID, itemID))
}

func idString(id int64) string {
	return strconv.FormatInt(id, 10)
}

func intString(value int) string {
	return strconv.Itoa(value)
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

func cardDetailID(id int64) string {
	return fmt.Sprintf("card-%d-detail", id)
}

func boardData(id int64) string {
	return fmt.Sprintf("kanbanBoard(%d)", id)
}

func timelineData(detail store.TimelineDetail) string {
	return fmt.Sprintf("timelineBoard(%d, %q, %d, %d)", detail.Board.ID, detail.From.Format("2006-01-02"), len(detail.Days), timelineCellWidth())
}

func timelineOpenCardAction(id int64) string {
	return fmt.Sprintf("openCard($event, 'card-%d')", id)
}

func appScripts() []string {
	return []string{"/static/shared.js", "/static/app.js"}
}

func timelineScripts() []string {
	return []string{"/static/shared.js", "/static/timeline.js"}
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

func startDateInput(card store.Card) string {
	if !card.StartAt.Valid {
		return ""
	}
	return card.StartAt.Time.Format("2006-01-02")
}

func cardDateText(card store.Card) string {
	if card.StartAt.Valid && card.DueAt.Valid {
		start := card.StartAt.Time.Format("2006-01-02")
		due := card.DueAt.Time.Format("2006-01-02")
		if start == due {
			return due
		}
		return start + " - " + due
	}
	if card.StartAt.Valid {
		return card.StartAt.Time.Format("2006-01-02")
	}
	if card.DueAt.Valid {
		return card.DueAt.Time.Format("2006-01-02")
	}
	return ""
}

func dateTimeText(value time.Time) string {
	return value.Format("2006-01-02 15:04")
}

func dueBadgeClass(card store.Card) string {
	base := "badge-outline tabular-nums "
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

const cardActivityPageSize = 10

func normalizedActivityPage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func activityPageCount(card store.Card) int {
	if len(card.Activities) == 0 {
		return 1
	}
	return (len(card.Activities) + cardActivityPageSize - 1) / cardActivityPageSize
}

func activityPageNumber(card store.Card, page int) int {
	page = normalizedActivityPage(page)
	total := activityPageCount(card)
	if page > total {
		return total
	}
	return page
}

func visibleActivities(card store.Card, page int) []store.ActivityEvent {
	page = activityPageNumber(card, page)
	start := (page - 1) * cardActivityPageSize
	if start >= len(card.Activities) {
		return nil
	}
	end := start + cardActivityPageSize
	if end > len(card.Activities) {
		end = len(card.Activities)
	}
	return card.Activities[start:end]
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

func timelineCellWidth() int {
	return 96
}

func timelineRowHeight() int {
	return 44
}

func timelineGridStyle(dayCount int) templ.SafeCSS {
	return templ.SafeCSS(fmt.Sprintf("grid-template-columns: repeat(%d, %dpx);", dayCount, timelineCellWidth()))
}

func timelineBoardStyle(dayCount int) templ.SafeCSS {
	return templ.SafeCSS(fmt.Sprintf("grid-template-columns: 256px %dpx;", dayCount*timelineCellWidth()))
}

func timelineLaneStyle(list store.TimelineList) templ.SafeCSS {
	height := maxViewInt(1, list.LaneCount)*timelineRowHeight() + 16
	return templ.SafeCSS(fmt.Sprintf("height: %dpx;", height))
}

func timelineCardStyle(card store.TimelineCard) templ.SafeCSS {
	left := card.OffsetDays*timelineCellWidth() + 8
	width := card.DurationDays*timelineCellWidth() - 16
	top := card.Lane*timelineRowHeight() + 8
	return templ.SafeCSS(fmt.Sprintf("left: %dpx; top: %dpx; width: %dpx;", left, top, maxViewInt(56, width)))
}

func timelineTodayStyle(detail store.TimelineDetail) templ.SafeCSS {
	offset := daysBetweenView(detail.From, truncateViewDate(time.Now()))
	return templ.SafeCSS(fmt.Sprintf("left: %dpx;", offset*timelineCellWidth()))
}

func timelineHasToday(detail store.TimelineDetail) bool {
	today := truncateViewDate(time.Now())
	return !today.Before(detail.From) && !today.After(detail.Through)
}

func timelineDayLabel(day time.Time) string {
	return day.Format("1/2")
}

func timelineWeekdayLabel(day time.Time) string {
	labels := []string{"日", "月", "火", "水", "木", "金", "土"}
	return labels[int(day.Weekday())]
}

func timelineDayClass(day time.Time) string {
	base := "flex h-14 flex-col justify-center border-r border-zinc-200 px-2 text-xs tabular-nums "
	if sameViewDate(day, time.Now()) {
		return base + "bg-blue-50 text-blue-700"
	}
	if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		return base + "bg-zinc-50 text-zinc-500"
	}
	return base + "bg-white text-zinc-600"
}

func timelineCellClass(day time.Time) string {
	base := "border-r border-zinc-200 "
	if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		return base + "bg-zinc-50/80"
	}
	return base + "bg-white"
}

func timelineCardClasses(card store.TimelineCard) string {
	base := "timeline-card absolute flex h-9 items-center gap-2 overflow-hidden rounded-md border px-2 text-left text-xs shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 "
	if card.Card.CompletedAt.Valid {
		return base + "border-green-300 bg-green-100 text-green-900"
	}
	if card.Card.DueAt.Valid && card.Card.DueAt.Time.Before(time.Now()) {
		return base + "border-red-300 bg-red-100 text-red-900"
	}
	return base + "border-blue-300 bg-blue-100 text-blue-950"
}

func timelineRangeText(card store.TimelineCard) string {
	start := card.StartDate.Format("2006-01-02")
	due := card.DueDate.Format("2006-01-02")
	if start == due {
		return start
	}
	return start + " - " + due
}

func timelineUnscheduledCount(detail store.TimelineDetail) int {
	return len(detail.Unscheduled)
}

func timelineSpanClass(current string, target string) string {
	if current == target {
		return "!border-zinc-950 !bg-zinc-950 !text-white"
	}
	return ""
}

func timelineTodayFrom() time.Time {
	today := truncateViewDate(time.Now())
	offset := (int(today.Weekday()) + 6) % 7
	return today.AddDate(0, 0, -offset)
}

func maxViewInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncateViewDate(value time.Time) time.Time {
	year, month, day := value.In(time.Local).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}

func daysBetweenView(from time.Time, to time.Time) int {
	return int(truncateViewDate(to).Sub(truncateViewDate(from)).Hours() / 24)
}

func sameViewDate(a time.Time, b time.Time) bool {
	return truncateViewDate(a).Equal(truncateViewDate(b))
}
