# GolangKanban

Go で実装したローカル専用の Trello 風カンバンアプリです。

HTML は Go 側から `templ` で生成し、通常操作は HTML フォームと HTMX の部分更新で処理します。リストやカードの並び替え、タイムライン上の日付変更、チェック項目の即時更新では最小限の JSON API を使います。通常のアプリ実行に Node.js は不要です。

## 機能

- ボードの作成、名前変更、削除
- リストの作成、名前変更、削除、並び替え
- カードの作成、タイトル/説明の編集、削除、並び替え、リスト間移動
- カード詳細ダイアログ
- 開始日、期限、完了状態、カバー色
- ラベル作成とカードへの付け外し
- チェックリストとチェック項目
- コメント
- `http` / `https` の URL 添付
- カードごとのアクティビティ履歴
- キーワード、ラベル、期限、完了状態によるボード内フィルター
- カードのアーカイブ、復元、完全削除
- リスト単位で表示するタイムライン
- タイムライン上でのカード移動、期間変更、未スケジュールカードへの日付設定
- PostgreSQL 18 への永続化
- 認証なしのローカル実行
- CSRF トークンと同一オリジン確認による書き込みリクエスト保護

## 技術スタック

- Go
- `net/http`
- `a-h/templ`
- PostgreSQL 18
- `database/sql` + `pgx`
- `goose`
- HTMX
- Tailwind CSS ブラウザ版
- Basecoat
- Alpine.js
- Alpine Sort プラグイン

Tailwind、HTMX、Basecoat、Alpine.js、Alpine Sort は `static/vendor/` にコミットしたローカルファイルを読み込みます。通常アプリと Storybook のプレビューは CDN に依存しません。

## 必要なもの

- Go 1.25.7 以上
- Docker
- `docker-compose`
- Storybook を使う場合のみ Node.js / npm / npx

このリポジトリでは `docker compose` ではなく、動作確認済みの `docker-compose` コマンドを使います。`make storybook` は初回に `storybook-server/` へ npm パッケージをインストールするため、その時だけ npm レジストリへの接続が必要です。

## クイックスタート

```sh
make db-up
make migrate-up
make run
```

起動後、ブラウザで開きます。

```text
http://localhost:8080/boards
```

デフォルトの接続先は次の通りです。

```text
postgres://golangkanban:golangkanban@localhost:5432/golangkanban?sslmode=disable
```

別の DB やポートを使う場合は `DATABASE_URL` や `ADDR` を指定します。

```sh
DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban_verify?sslmode=disable' make migrate-up
DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban_verify?sslmode=disable' ADDR=':8081' make run
```

## コマンド

```sh
make db-up        # PostgreSQL 18 を起動
make db-down      # PostgreSQL 18 を停止
make migrate-up   # goose マイグレーションを適用
make migrate-down # goose マイグレーションを 1 つ戻す
make templ        # templ の Go コードを生成
make run          # templ のコード生成後にアプリを起動
make storybook    # templ のコード生成後にコンポーネントカタログを起動
make test         # go test ./...
```

## プロジェクト構成

```text
cmd/server/        HTTP サーバーのエントリーポイント
cmd/storybook/     templ/storybook の起動コード
internal/http/     ハンドラー、ルーティング、HTML/JSON レスポンス、CSRF 保護
internal/store/    database/sql を使った永続化層
internal/view/     templ コンポーネントと生成済み templ コード
migrations/        goose の SQL マイグレーション
static/            アプリ用 CSS/JS とローカル vendor ファイル
storybook-server/  Storybook の設定と npm 依存
docker-compose.yml ローカル開発用の PostgreSQL 18
```

## HTTP インターフェース

HTML ルート:

```text
GET  /                                      /boards へリダイレクト
GET  /static/*                             静的ファイル
GET  /boards                               ボード一覧
POST /boards                               ボード作成
GET  /boards/{boardID}                     フィルター付きボード詳細
GET  /boards/{boardID}/archive             アーカイブ済みカード
GET  /boards/{boardID}/timeline            リスト単位のタイムライン表示
POST /boards/{boardID}/rename              ボード名変更
POST /boards/{boardID}/delete              ボード削除
POST /boards/{boardID}/lists               リスト作成
POST /boards/{boardID}/labels              ラベル作成
POST /lists/{listID}/rename                リスト名変更
POST /lists/{listID}/delete                リスト削除
POST /lists/{listID}/cards                 カード作成
POST /cards/{cardID}/update                タイトル/説明の更新
POST /cards/{cardID}/dates                 開始日、期限、カバー色の更新
POST /cards/{cardID}/complete              完了状態の切り替え
POST /cards/{cardID}/archive               カードをアーカイブ
POST /cards/{cardID}/restore               アーカイブ済みカードを復元
POST /cards/{cardID}/delete                カードを完全削除
POST /cards/{cardID}/labels                カードのラベルを置き換え
POST /cards/{cardID}/comments              コメント追加
POST /cards/{cardID}/attachments           URL 添付を追加
POST /cards/{cardID}/checklists            チェックリスト追加
POST /cards/{cardID}/checklists/{checklistID}/items
                                             チェック項目追加
POST /cards/{cardID}/checklist-items/{itemID}/toggle
                                             チェック項目の切り替え
GET  /cards/{cardID}/activities            アクティビティ履歴のページング
```

POST と PATCH は CSRF トークンが必要です。HTML フォームでは `_csrf`、JSON API では `X-CSRF-Token` ヘッダーを使います。

ボードとタイムラインのフィルターは URL クエリパラメーターで指定します。

```text
q       タイトル/説明の部分一致検索
label   ラベル ID
due     overdue | today | week | none
status  complete | incomplete
```

例:

```text
GET /boards/{boardID}?q=release&label=1&due=week&status=incomplete
GET /boards/{boardID}/timeline?from=2026-05-04&span=6w&q=release
```

タイムラインの `from` は `YYYY-MM-DD` 形式です。未指定または不正な場合は現在週の開始日に戻ります。`span` は `6w` または `quarter` を指定できます。`quarter` は 91 日表示、それ以外は 6 週間表示です。

JSON ルート:

```text
PATCH /api/lists/reorder
PATCH /api/cards/reorder
PATCH /api/cards/{cardID}/timeline
PATCH /api/checklist-items/{itemID}
```

ペイロード例:

```json
{ "boardId": 1, "listIds": [3, 1, 2] }
```

```json
{ "toListId": 2, "cardIds": [8, 10, 9] }
```

```json
{ "startAt": "2026-05-04", "dueAt": "2026-05-08" }
```

```json
{ "checked": true }
```

リストとカードの並び替え API は成功時に `204 No Content` を返します。タイムライン更新 API は保存後の `cardId`、`startAt`、`dueAt` を JSON で返します。チェック項目 API は保存後の `checked` を JSON で返します。

## データベース

マイグレーションでは次のテーブルを作成、拡張します。

- `boards`
- `lists`
- `cards`
- `labels`
- `card_labels`
- `checklists`
- `checklist_items`
- `comments`
- `attachments`
- `activity_events`

各リストとカードは整数の `position` を持ちます。ドラッグアンドドロップ後、サーバーはトランザクション内で並び順を `1..n` に振り直します。カードにはタイムライン表示用の `start_at` と `due_at` も設定できます。アーカイブ済みカードは `archived_at` 付きでデータベースに残り、通常のボード表示からは除外されます。

カードの作成、編集、移動、完了状態の変更、アーカイブ、復元、削除、コメント追加、添付追加などは `activity_events` に履歴として記録されます。

## 使い方のメモ

- ボード上で `/` を押すと、カードフィルターにフォーカスします。
- ボードのツールバーからラベル作成とカードの絞り込みができます。
- カードは Alpine Sort でドラッグして並び替えたり、別リストへ移動したりできます。
- タイムライン表示では、リストごとにカードを確認できます。バーをドラッグまたはリサイズすると開始日と期限を保存できます。
- 開始日と期限がないカードは、タイムラインの未スケジュール欄から日付を設定できます。
- カードを開くと、詳細、ラベル、チェックリスト、コメント、添付、開始日、期限、完了状態、カバー色、アーカイブ状態を編集できます。
- URL 添付は `http` と `https` のみ許可します。
- 破壊的な操作では確認ダイアログを表示します。

## 開発メモ

- `.templ` ファイルを編集した後は `make templ` を実行します。
- 生成済みの `*_templ.go` ファイルはコミットします。これにより、templ CLI を実行しなくてもアプリをビルドできます。
- 通常アプリの JavaScript は `static/shared.js`、`static/app.js`、`static/timeline.js` にあります。
- vendor ファイルは `static/vendor/` に配置します。CDN URL をテンプレートへ直接追加しないでください。
- 現在コミットしている vendor は Tailwind CSS ブラウザ版 `4.2.4`、HTMX `2.0.10`、Basecoat `0.3.11`、Alpine.js `3.15.12`、Alpine Sort `3.15.12` です。
- `make storybook` は `templ/storybook` のコンポーネントカタログを `http://localhost:60606` で起動します。別ポートを使う場合は `STORYBOOK_ADDR=':60607' make storybook` のように指定します。
- Storybook は `storybook-server/` にコミットされた設定を使い、初回実行時に必要なフロントエンド依存をそこへインストールします。`node_modules/`、生成されたストーリー、静的ビルド出力は無視されます。
- セキュリティヘッダーは `internal/http/security.go` で設定します。CSP は現在 `Content-Security-Policy-Report-Only` です。
- ストアの結合テストはデフォルトではスキップされます。PostgreSQL に接続して実行する場合は次のようにします。

```sh
TEST_DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban?sslmode=disable' go test ./...
```

## トラブルシューティング

`make migrate-up` が `relation "boards" already exists` で失敗する場合、Docker ボリュームに古い互換性のないスキーマが残っている可能性があります。

使い捨てのローカル DB であれば、ボリュームをリセットします。

```sh
docker-compose down -v
make db-up
make migrate-up
```

既存のボリュームを削除したくない場合は、別のデータベースを作成して使います。

```sh
docker-compose exec postgres createdb -U postgres -O golangkanban golangkanban_verify
DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban_verify?sslmode=disable' make migrate-up
DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban_verify?sslmode=disable' make run
```

ドラッグアンドドロップ時に `並び順を保存できませんでした。ページを再読み込みしてください。` と表示された場合は、ページを再読み込みして再度試してください。繰り返し失敗する場合は、ページの状態が古いか、接続先 DB が想定したスキーマを使っていない可能性があります。

タイムライン上で `タイムラインの日付を保存できませんでした。ページを再読み込みしてください。` と表示された場合は、開始日と期限の順序、接続先 DB のスキーマ、ページの鮮度を確認してください。
