package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/a-h/templ/storybook"

	"golangkanban/internal/store"
	"golangkanban/internal/view"
)

func main() {
	sb := storybook.New(
		storybook.WithServerAddr(env("STORYBOOK_ADDR", ":60606")),
		storybook.WithHeader(storybookHeader()),
	)
	storybookStaticHandler := sb.StaticHandler
	sb.StaticHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") {
			http.StripPrefix("/static/", http.FileServer(http.Dir("static"))).ServeHTTP(w, r)
			return
		}
		storybookStaticHandler.ServeHTTP(w, r)
	})

	sb.AddComponent(
		"BoardsContent",
		boardsContentStory,
		storybook.BooleanArg("Empty", false),
		storybook.TextArg("PageError", ""),
	)
	sb.AddComponent(
		"EmptyState",
		emptyStateStory,
		storybook.TextArg("Title", "まだ項目がありません"),
		storybook.TextArg("Description", "最初の項目を追加するとここに表示されます。"),
	)
	sb.AddComponent(
		"ErrorAlert",
		errorAlertStory,
		storybook.TextArg("Message", "保存できませんでした。入力内容を確認してください。"),
	)
	sb.AddComponent("FilterBar", filterBarStory)
	sb.AddComponent("LabelManager", labelManagerStory)
	sb.AddComponent("BoardListsSection", boardListsSectionStory)
	sb.AddComponent("ListColumn", listColumnStory)

	cardTile := sb.AddComponent(
		"CardTile",
		cardTileStory,
		storybook.TextArg("Title", "リリース前の確認"),
		storybook.TextArg("Description", "チェックリスト、コメント、添付リンクがあるカードです。"),
		storybook.TextArg("CoverColor", "blue"),
		storybook.BooleanArg("Completed", false),
	)
	cardTile.AddStory(
		"Completed",
		storybook.TextArg("Title", "完了済みカード"),
		storybook.TextArg("Description", "完了バッジの表示を確認します。"),
		storybook.TextArg("CoverColor", "green"),
		storybook.BooleanArg("Completed", true),
	)
	cardTile.AddStory(
		"LongText",
		storybook.TextArg("Title", "非常に長いカードタイトルが複数行になってもカード内で破綻しないことを確認するためのサンプル"),
		storybook.TextArg("Description", "説明文も長い場合にカードの高さとメタ情報の位置が自然に保たれるかを確認します。"),
		storybook.TextArg("CoverColor", "orange"),
		storybook.BooleanArg("Completed", false),
	)

	sb.AddComponent(
		"CardBodyForm",
		cardBodyFormStory,
		storybook.TextArg("Title", "カードタイトル"),
		storybook.TextArg("Description", "フォーカスアウトで保存される本文フォームです。"),
	)
	sb.AddComponent("CardLabelsSection", cardLabelsSectionStory)
	sb.AddComponent("CardChecklistSection", cardChecklistSectionStory)
	sb.AddComponent("CardCommentsSection", cardCommentsSectionStory)
	sb.AddComponent("CardAttachmentsSection", cardAttachmentsSectionStory)
	sb.AddComponent(
		"CardMetaSection",
		cardMetaSectionStory,
		storybook.BooleanArg("Archived", false),
		storybook.BooleanArg("Completed", false),
	)

	cardDetail := sb.AddComponent(
		"CardDetailArticle",
		cardDetailStory,
		storybook.TextArg("Title", "チケットモーダル"),
		storybook.TextArg("Description", "本文、ラベル、チェックリスト、コメント、添付、アクティビティを含む詳細表示です。"),
		storybook.TextArg("CoverColor", "purple"),
		storybook.BooleanArg("Archived", false),
		storybook.BooleanArg("Completed", false),
	)
	cardDetail.AddStory(
		"Archived",
		storybook.TextArg("Title", "アーカイブ済みカード"),
		storybook.TextArg("Description", "復元ボタンを含むアーカイブ状態の表示を確認します。"),
		storybook.TextArg("CoverColor", "gray"),
		storybook.BooleanArg("Archived", true),
		storybook.BooleanArg("Completed", true),
	)

	sb.AddComponent(
		"CardActivitySection",
		cardActivitySectionStory,
		storybook.IntArg("EventCount", 12, storybook.IntArgConf{Min: intPtr(0), Max: intPtr(40), Step: intPtr(1)}),
		storybook.IntArg("Page", 1, storybook.IntArgConf{Min: intPtr(1), Max: intPtr(4), Step: intPtr(1)}),
	)
	sb.AddComponent(
		"DeleteBoardDialog",
		deleteBoardDialogStory,
		storybook.TextArg("Title", "プロダクト開発"),
	)
	sb.AddComponent(
		"DeleteListDialog",
		deleteListDialogStory,
		storybook.TextArg("Title", "未完了"),
	)
	sb.AddComponent(
		"DeleteCardDialog",
		deleteCardDialogStory,
		storybook.BooleanArg("Archived", false),
	)

	board := sb.AddComponent(
		"BoardContent",
		boardContentStory,
		storybook.TextArg("BoardTitle", "プロダクト開発"),
		storybook.BooleanArg("Empty", false),
	)
	board.AddStory(
		"Empty",
		storybook.TextArg("BoardTitle", "空のボード"),
		storybook.BooleanArg("Empty", true),
	)

	sb.AddComponent(
		"ArchiveContent",
		archiveContentStory,
		storybook.BooleanArg("Empty", false),
		storybook.TextArg("PageError", ""),
	)
	sb.AddComponent(
		"ArchiveCardsSection",
		archiveCardsSectionStory,
		storybook.BooleanArg("Empty", false),
	)
	sb.AddComponent("TimelineContent", timelineContentStory)
	sb.AddComponent("TimelineDisplay", timelineDisplayStory)
	sb.AddComponent("TimelineBoardSection", timelineBoardSectionStory)
	sb.AddComponent("TimelineListRow", timelineListRowStory)
	sb.AddComponent("TimelineCardButton", timelineCardButtonStory)
	sb.AddComponent("TimelineUnscheduled", timelineUnscheduledStory)

	log.Printf("Storybook: http://localhost%s", env("STORYBOOK_ADDR", ":60606"))
	if err := sb.ListenAndServeWithContext(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func boardsContentStory(empty bool, pageError string) templ.Component {
	boards := sampleBoards()
	if empty {
		boards = nil
	}
	return storyWithCSRF(view.BoardsContent(boards, pageError))
}

func emptyStateStory(title string, description string) templ.Component {
	return storyFrame(view.EmptyState(title, description))
}

func errorAlertStory(message string) templ.Component {
	return storyFrame(view.ErrorAlert(message))
}

func filterBarStory() templ.Component {
	return storyFrame(view.FilterBar(sampleBoardDetail("プロダクト開発")))
}

func labelManagerStory() templ.Component {
	return storyFrame(view.LabelManager(sampleBoardDetail("プロダクト開発")))
}

func boardListsSectionStory() templ.Component {
	return storyWithCSRF(view.BoardListsSection(sampleBoardDetail("プロダクト開発")))
}

func listColumnStory() templ.Component {
	detail := sampleBoardDetail("プロダクト開発")
	return storyFrame(view.ListColumn(detail.Lists[0], detail.Labels))
}

func cardTileStory(title string, description string, coverColor string, completed bool) templ.Component {
	return storyFrame(view.CardTile(sampleCard(title, description, coverColor, completed)))
}

func cardBodyFormStory(title string, description string) templ.Component {
	return storyFrame(view.CardBodyForm(sampleCard(title, description, "", false)))
}

func cardLabelsSectionStory() templ.Component {
	return storyFrame(view.CardLabelsSection(sampleCard("ラベル確認", "", "", false), sampleLabels()))
}

func cardChecklistSectionStory() templ.Component {
	return storyFrame(view.CardChecklistSection(sampleCard("チェックリスト確認", "", "", false)))
}

func cardCommentsSectionStory() templ.Component {
	return storyFrame(view.CardCommentsSection(sampleCard("コメント確認", "", "", false)))
}

func cardAttachmentsSectionStory() templ.Component {
	return storyFrame(view.CardAttachmentsSection(sampleCard("添付確認", "", "", false)))
}

func cardMetaSectionStory(archived bool, completed bool) templ.Component {
	return storyFrame(view.CardMetaSection(sampleCard("メタ情報確認", "", "yellow", completed), archived))
}

func cardDetailStory(title string, description string, coverColor string, archived bool, completed bool) templ.Component {
	card := sampleCard(title, description, coverColor, completed)
	card.Activities = sampleActivities(12)
	return storyFrame(view.CardDetailArticle(card, sampleLabels(), archived, ""))
}

func cardActivitySectionStory(eventCount int, page int) templ.Component {
	card := sampleCard("アクティビティ確認", "", "", false)
	card.Activities = sampleActivities(eventCount)
	return storyFrame(view.CardActivitySection(card, page))
}

func deleteBoardDialogStory(title string) templ.Component {
	return storyFrame(view.DeleteBoardDialog(store.Board{ID: 1, Title: title}))
}

func deleteListDialogStory(title string) templ.Component {
	return storyFrame(view.DeleteListDialog(store.List{ID: 1, BoardID: 1, Title: title}))
}

func deleteCardDialogStory(archived bool) templ.Component {
	return storyFrame(view.DeleteCardDialog(sampleCard("削除確認", "", "", false), archived))
}

func boardContentStory(boardTitle string, empty bool) templ.Component {
	detail := sampleBoardDetail(boardTitle)
	if empty {
		detail.Lists = nil
	}
	return view.BoardContent(detail, "")
}

func archiveContentStory(empty bool, pageError string) templ.Component {
	detail := sampleArchiveDetail()
	if empty {
		detail.Cards = nil
	}
	return storyWithCSRF(view.ArchiveContent(detail, pageError))
}

func archiveCardsSectionStory(empty bool) templ.Component {
	detail := sampleArchiveDetail()
	if empty {
		detail.Cards = nil
	}
	return storyWithCSRF(view.ArchiveCardsSection(detail))
}

func timelineContentStory() templ.Component {
	return view.TimelineContent(sampleTimelineDetail(), "")
}

func timelineDisplayStory() templ.Component {
	return storyWithCSRF(view.TimelineDisplay(sampleTimelineDetail()))
}

func timelineBoardSectionStory() templ.Component {
	return storyWithCSRF(view.TimelineBoardSection(sampleTimelineDetail()))
}

func timelineListRowStory() templ.Component {
	detail := sampleTimelineDetail()
	return storyWithCSRF(view.TimelineListRow(detail, detail.Lists[0]))
}

func timelineCardButtonStory() templ.Component {
	detail := sampleTimelineDetail()
	return storyFrame(view.TimelineCardButton(detail.Lists[0].Cards[0]))
}

func timelineUnscheduledStory() templ.Component {
	return storyWithCSRF(view.TimelineUnscheduled(sampleTimelineDetail()))
}

func storyFrame(component templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		ctx = view.WithCSRFToken(ctx, "storybook-csrf-token")
		_, _ = fmt.Fprint(w, `<main class="min-h-dvh bg-zinc-100 p-6 text-zinc-950"><div class="mx-auto max-w-5xl">`)
		if err := component.Render(ctx, w); err != nil {
			return err
		}
		_, _ = fmt.Fprint(w, `</div></main>`)
		return nil
	})
}

func storyWithCSRF(component templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return component.Render(view.WithCSRFToken(ctx, "storybook-csrf-token"), w)
	})
}

func storybookHeader() string {
	css, err := os.ReadFile("static/app.css")
	if err != nil {
		log.Printf("static/app.css を読み込めませんでした: %v", err)
	}
	appCSS := strings.ReplaceAll(string(css), "</style", "<\\/style")
	return fmt.Sprintf(`
<meta name="viewport" content="width=device-width, initial-scale=1">
<script src="/static/vendor/tailwindcss-browser-4.2.4.js"></script>
<script src="/static/vendor/htmx-2.0.10.min.js"></script>
<link rel="stylesheet" href="/static/vendor/basecoat-0.3.11.min.css">
<script src="/static/vendor/basecoat-0.3.11.all.min.js" defer></script>
<script defer src="/static/vendor/alpine-sort-3.15.12.min.js"></script>
<script defer src="/static/vendor/alpine-3.15.12.min.js"></script>
<style>%s</style>`, appCSS)
}

func sampleBoards() []store.Board {
	now := sampleNow()
	return []store.Board{
		{ID: 1, Title: "プロダクト開発", CreatedAt: now.AddDate(0, 0, -10), UpdatedAt: now},
		{ID: 2, Title: "採用計画", CreatedAt: now.AddDate(0, 0, -8), UpdatedAt: now.Add(-2 * time.Hour)},
		{ID: 3, Title: "個人タスク", CreatedAt: now.AddDate(0, 0, -5), UpdatedAt: now.Add(-24 * time.Hour)},
	}
}

func sampleBoardDetail(title string) store.BoardDetail {
	labels := sampleLabels()
	now := sampleNow()
	return store.BoardDetail{
		Board: store.Board{ID: 1, Title: title, CreatedAt: now, UpdatedAt: now},
		Labels: labels,
		Lists: []store.List{
			{
				ID: 1, BoardID: 1, Title: "未完了", Position: 1, CreatedAt: now, UpdatedAt: now,
				Cards: []store.Card{
					sampleCard("仕様を整理する", "受け入れ条件と画面遷移を確認する。", "blue", false),
					sampleCard("UI の余白確認", "長いタイトルや説明の折り返しを見る。", "", false),
				},
			},
			{
				ID: 2, BoardID: 1, Title: "進行中", Position: 2, CreatedAt: now, UpdatedAt: now,
				Cards: []store.Card{
					sampleCard("Storybook を導入する", "代表コンポーネントを単体で確認できるようにする。", "purple", false),
				},
			},
			{
				ID: 3, BoardID: 1, Title: "完了", Position: 3, CreatedAt: now, UpdatedAt: now,
				Cards: []store.Card{
					sampleCard("テストを追加する", "templ コンポーネントを goquery で検査する。", "green", true),
				},
			},
		},
	}
}

func sampleArchiveDetail() store.ArchiveDetail {
	now := sampleNow()
	cardA := sampleCard("アーカイブ済みカード", "復元や完全削除の確認用です。", "gray", false)
	cardA.ArchivedAt = sql.NullTime{Time: now.Add(-24 * time.Hour), Valid: true}
	cardB := sampleCard("完了後に保管したカード", "完了済みのアーカイブ表示を確認します。", "green", true)
	cardB.ID = 10
	cardB.ArchivedAt = sql.NullTime{Time: now.Add(-12 * time.Hour), Valid: true}
	return store.ArchiveDetail{
		Board:  store.Board{ID: 1, Title: "プロダクト開発", CreatedAt: now.AddDate(0, 0, -10), UpdatedAt: now},
		Labels: sampleLabels(),
		Cards:  []store.Card{cardA, cardB},
	}
}

func sampleTimelineDetail() store.TimelineDetail {
	now := sampleNow()
	from := time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local)
	days := make([]time.Time, 14)
	for i := range days {
		days[i] = from.AddDate(0, 0, i)
	}

	cardA := sampleCard("Storybook を導入する", "タイムライン上のバー表示を確認します。", "blue", false)
	cardA.StartAt = sql.NullTime{Time: from.AddDate(0, 0, 1), Valid: true}
	cardA.DueAt = sql.NullTime{Time: from.AddDate(0, 0, 5), Valid: true}
	cardB := sampleCard("完了済みの予定", "完了バッジ付きのタイムラインカードです。", "green", true)
	cardB.ID = 10
	cardB.StartAt = sql.NullTime{Time: from.AddDate(0, 0, 3), Valid: true}
	cardB.DueAt = sql.NullTime{Time: from.AddDate(0, 0, 8), Valid: true}
	unscheduled := sampleCard("日付未設定のカード", "開始日と期限を入れるとタイムラインへ移動します。", "", false)
	unscheduled.ID = 11
	unscheduled.StartAt = sql.NullTime{}
	unscheduled.DueAt = sql.NullTime{}

	return store.TimelineDetail{
		Board:    store.Board{ID: 1, Title: "プロダクト開発", CreatedAt: now.AddDate(0, 0, -10), UpdatedAt: now},
		Labels:   sampleLabels(),
		From:     from,
		Through:  from.AddDate(0, 0, len(days)-1),
		Days:     days,
		Span:     "6w",
		PrevFrom: from.AddDate(0, 0, -14),
		NextFrom: from.AddDate(0, 0, 14),
		Lists: []store.TimelineList{
			{
				List:      store.List{ID: 1, BoardID: 1, Title: "進行中", Position: 1, CreatedAt: now, UpdatedAt: now},
				LaneCount: 2,
				Cards: []store.TimelineCard{
					{Card: cardA, StartDate: cardA.StartAt.Time, DueDate: cardA.DueAt.Time, StartOffset: 1, DueOffset: 5, OffsetDays: 1, DurationDays: 5, Lane: 0},
					{Card: cardB, StartDate: cardB.StartAt.Time, DueDate: cardB.DueAt.Time, StartOffset: 3, DueOffset: 8, OffsetDays: 3, DurationDays: 6, Lane: 1},
				},
				Unscheduled: []store.Card{unscheduled},
			},
			{
				List:      store.List{ID: 2, BoardID: 1, Title: "完了", Position: 2, CreatedAt: now, UpdatedAt: now},
				LaneCount: 1,
			},
		},
	}
}

func sampleCard(title string, description string, coverColor string, completed bool) store.Card {
	now := sampleNow()
	card := store.Card{
		ID:              9,
		ListID:          1,
		Title:           title,
		Description:     description,
		Position:        1,
		CoverColor:      coverColor,
		StartAt:         sql.NullTime{Time: now.AddDate(0, 0, -1), Valid: true},
		DueAt:           sql.NullTime{Time: now.AddDate(0, 0, 5), Valid: true},
		Labels:          sampleLabels()[:2],
		ChecklistTotal:  3,
		CommentCount:    2,
		AttachmentCount: 1,
		CreatedAt:       now.AddDate(0, 0, -3),
		UpdatedAt:       now,
		Checklists: []store.Checklist{
			{
				ID: 1, CardID: 9, Title: "確認項目", Position: 1, CompletedCount: 1, TotalCount: 3,
				Items: []store.ChecklistItem{
					{ID: 1, ChecklistID: 1, Title: "仕様を確認", CheckedAt: sql.NullTime{Time: now, Valid: true}},
					{ID: 2, ChecklistID: 1, Title: "表示を確認"},
					{ID: 3, ChecklistID: 1, Title: "テストを確認"},
				},
			},
		},
		Comments: []store.Comment{
			{ID: 1, CardID: 9, Body: "コメントの表示例です。", CreatedAt: now.Add(-2 * time.Hour)},
			{ID: 2, CardID: 9, Body: "複数件ある状態を確認します。", CreatedAt: now.Add(-1 * time.Hour)},
		},
		Attachments: []store.Attachment{
			{ID: 1, CardID: 9, Title: "関連ドキュメント", URL: "https://example.com", CreatedAt: now},
		},
		Activities: sampleActivities(4),
	}
	if completed {
		card.CompletedAt = sql.NullTime{Time: now.Add(-30 * time.Minute), Valid: true}
	}
	return card
}

func sampleLabels() []store.Label {
	now := sampleNow()
	return []store.Label{
		{ID: 1, BoardID: 1, Name: "Frontend", Color: "green", Position: 1, CreatedAt: now, UpdatedAt: now},
		{ID: 2, BoardID: 1, Name: "Backend", Color: "blue", Position: 2, CreatedAt: now, UpdatedAt: now},
		{ID: 3, BoardID: 1, Name: "Urgent", Color: "red", Position: 3, CreatedAt: now, UpdatedAt: now},
	}
}

func sampleActivities(count int) []store.ActivityEvent {
	if count < 0 {
		count = 0
	}
	now := sampleNow()
	events := make([]store.ActivityEvent, 0, count)
	for i := 1; i <= count; i++ {
		events = append(events, store.ActivityEvent{
			ID:        int64(i),
			BoardID:   1,
			CardID:    sql.NullInt64{Int64: 9, Valid: true},
			EventType: "storybook",
			Message:   fmt.Sprintf("Storybook 用アクティビティ %02d", i),
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		})
	}
	return events
}

func sampleNow() time.Time {
	return time.Date(2026, 5, 6, 10, 0, 0, 0, time.Local)
}

func intPtr(value int) *int {
	return &value
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
