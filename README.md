# GolangKanban

Go で実装したローカル専用の Trello 風カンバンアプリです。

HTML は Go 側から `templ` で生成し、通常操作は HTML フォームで処理します。ドラッグアンドドロップ後の並び順保存だけ、最小限の JSON API を使います。Node.js は使いません。

## Features

- ボードの作成、名前変更、削除
- リストの作成、名前変更、削除、並び替え
- カードの作成、タイトル/説明の編集、削除、並び替え
- カードのリスト間移動
- PostgreSQL 18 への永続化
- 認証なしのローカル実行

## Stack

- Go
- `net/http`
- `a-h/templ`
- PostgreSQL 18
- `database/sql` + `pgx`
- `goose`
- Tailwind CSS Play CDN
- Basecoat CDN
- Alpine.js
- Alpine Sort plugin

## Requirements

- Go 1.25.7 以上
- Docker
- `docker-compose`

このリポジトリでは `docker compose` ではなく、動作確認済みの `docker-compose` コマンドを使います。

## Quick Start

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

別の DB を使う場合は `DATABASE_URL` を指定します。

```sh
DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban_verify?sslmode=disable' make migrate-up
DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban_verify?sslmode=disable' make run
```

## Commands

```sh
make db-up        # PostgreSQL 18 を起動
make migrate-up   # goose migration を適用
make migrate-down # goose migration を 1 つ戻す
make templ        # templ の Go コードを生成
make run          # templ generate 後にアプリを起動
make test         # go test ./...
```

## Project Layout

```text
cmd/server/        HTTP server entrypoint
internal/http/     handlers, routing, HTML/JSON responses
internal/store/    database/sql based persistence layer
internal/view/     templ components and generated templ code
migrations/        goose SQL migrations
docker-compose.yml PostgreSQL 18 for local development
```

## HTTP Interface

HTML routes:

```text
GET  /                         -> /boards
GET  /boards                   board list
POST /boards                   create board
GET  /boards/{boardID}         board detail
POST /boards/{boardID}/rename  rename board
POST /boards/{boardID}/delete  delete board
POST /boards/{boardID}/lists   create list
POST /lists/{listID}/rename    rename list
POST /lists/{listID}/delete    delete list
POST /lists/{listID}/cards     create card
POST /cards/{cardID}/update    update card
POST /cards/{cardID}/delete    delete card
```

JSON routes for DnD persistence:

```text
PATCH /api/lists/reorder
PATCH /api/cards/reorder
```

Example payloads:

```json
{ "boardId": 1, "listIds": [3, 1, 2] }
```

```json
{ "toListId": 2, "cardIds": [8, 10, 9] }
```

## Database

The first migration creates:

- `boards`
- `lists`
- `cards`

Each list and card has an integer `position`. After DnD, the server rewrites positions as `1..n` inside a transaction.

## Development Notes

- Edit `.templ` files, then run `make templ`.
- Generated `*_templ.go` files are committed so the app can build without running Node.js.
- Tailwind, Basecoat, Alpine.js, and Alpine Sort are loaded from CDNs in the shared layout.
- Destructive actions use dialog confirmation in the UI.
- Store integration tests are skipped by default. To run them against PostgreSQL:

```sh
TEST_DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban?sslmode=disable' go test ./...
```

## Troubleshooting

If `make migrate-up` fails with `relation "boards" already exists`, the Docker volume likely contains an older incompatible schema.

For a disposable local DB, reset the volume:

```sh
docker-compose down -v
make db-up
make migrate-up
```

If you do not want to delete the existing volume, create and use another database:

```sh
docker-compose exec postgres createdb -U postgres -O golangkanban golangkanban_verify
DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban_verify?sslmode=disable' make migrate-up
DATABASE_URL='postgres://golangkanban:golangkanban@localhost:5432/golangkanban_verify?sslmode=disable' make run
```

If DnD shows `並び順を保存できませんでした。ページを再読み込みしてください。`, reload the page and try again. The current implementation sends only direct sortable items to the JSON API, so repeated failures usually mean the page is stale or the target DB is not using the expected schema.
