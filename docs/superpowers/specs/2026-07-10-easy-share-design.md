# easy-share — Design Spec

**Date:** 2026-07-10
**Status:** Approved
**Approach:** A — Pure Go stdlib + embedded single-page frontend

## Goal

A super simple file-hosting/upload site. Drag & drop a file → see upload progress → get a shareable URL. Japanese-language UI, clean and cute. Deployable via Railpack or docker-compose.

## Non-goals (YAGNI)

- Authentication / accounts
- Chunked/resumable uploads
- Database
- Object storage (S3)
- Rate limiting / admin UI
- File listings / browsing

## Architecture

A single Go binary that does three jobs in one process:

1. Serves an **embedded frontend** (HTML/CSS/JS compiled in via `//go:embed`) at `/`.
2. Exposes a tiny **JSON API** for upload/download.
3. Runs a **background goroutine** that deletes files older than the retention window.

No database — metadata is stored as JSON sidecar files on disk.

## Storage layout

```
data/
  files/<id>        # raw bytes
  meta/<id>.json    # {id, filename, contentType, size, createdAt}
```

- **`<id>`**: 12-character random base62 string (~71 bits entropy). Unguessable, so the link itself is the access token.
- On upload: generate id, stream request body to `data/files/<id>`, write sidecar JSON to `data/meta/<id>.json`.
- On download: read sidecar for filename/contentType, stream `data/files/<id>` with correct headers.

## HTTP routes

| Method | Path          | Purpose                                                                |
|--------|---------------|------------------------------------------------------------------------|
| GET    | `/`           | embedded `index.html`                                                  |
| GET    | `/static/*`   | embedded CSS/JS (served via embedded FS)                               |
| POST   | `/api/upload` | stream file to disk; JSON `{id, url, filename, size}`; 413 if too big |
| GET    | `/d/<id>`     | serve file inline (images/PDFs render; others download) = share URL   |

## Upload progress

Frontend uses `XMLHttpRequest` with `xhr.upload.onprogress` for native byte-level progress. (`fetch()` cannot report upload progress; XHR can — no WebSockets/SSE needed.)

Max upload size is enforced on two layers:
- **Client-side:** check `file.size <= MAX_UPLOAD_BYTES` before uploading; show a friendly Japanese error.
- **Server-side:** wrap the request body with `http.MaxBytesReader`; oversized uploads return `413`.

## Cleanup (retention)

A goroutine with a 1-hour `time.Ticker` walks `data/meta/`, reads each sidecar's `createdAt`, and deletes the entry (file + sidecar) when older than the retention window. The cleanup also runs once at startup.

## Configuration (env vars)

| Env               | Default        | Meaning                                                        |
|-------------------|----------------|----------------------------------------------------------------|
| `PORT`            | `8080`         | listen port                                                    |
| `DATA_DIR`        | `./data`       | storage root (creates `files/` and `meta/` subdirs)            |
| `MAX_UPLOAD_BYTES`| `104857600`    | 100 MiB hard cap                                               |
| `RETENTION`       | `720h`         | 30 days                                                        |
| `BASE_URL`        | _(empty)_      | optional origin for absolute share URLs (e.g. `https://share.example.com`); if unset, derived from request `Host` |

## Frontend UX (Japanese, clean/cute)

Single responsive, mobile-friendly page with four states:

1. **待機 (Idle):** large dashed drop-zone — "ここにファイルをドロップ" + "または クリックして選択"; hint "最大100MB".
2. **アップロード中:** filename + animated progress bar with percentage.
3. **完了:** green check; share URL in a copyable box with a "コピー" button (feedback "コピーしました！"); plus "別のファイルをアップロード" to reset.
4. **エラー:** friendly Japanese message + retry.

**Cute styling:** soft pastel palette, rounded centered card with subtle shadow, gentle transitions, Japanese font stack (`Hiragino Kaku Gothic ProN`, `Noto Sans JP`, system fallback). Mobile uses a hidden `<input type="file">` since drag-drop is unavailable there.

## Project structure

```
easy-share/
  main.go
  internal/
    config.go     # env config
    server.go     # HTTP handlers + routing (stdlib net/http)
    store.go      # disk storage ops: save, get, delete, cleanup, id gen
  web/
    index.html    # embedded
    style.css     # embedded
    app.js        # embedded
  Dockerfile            # multi-stage -> alpine single binary
  docker-compose.yml    # service + volume + ports + env
  railpack.json         # Railpack Go build config
  .gitignore
  README.md
  go.mod
```

Embedding: `//go:embed web/*` exposes the frontend files via `http.FileServer(http.FS(...))`.

## Deployment

- **Docker:** multi-stage build (`golang` build stage → `alpine` runtime). Single static binary, expose `8080`, mount `./data:/data`.
- **docker-compose:** one `app` service, `ports: 8080:8080`, `volumes: ./data:/data`, env passthrough.
- **Railpack:** native Go support; `railpack.json` declares the build (Go install) + start command + `PORT` exposure.

## Testing

- `go test ./...`
- `internal/store` tests: save/get/delete/cleanup using temp dirs.
- Handler tests: upload success (200 + JSON), oversize → 413, download 200 + correct headers, download missing → 404.
- Frontend: manual verification (no JS framework/tests for this size).

## Security

- Unguessable random IDs (link = access token).
- `http.MaxBytesReader` prevents memory/disk exhaustion from oversized uploads.
- Reasonable `http.Server` timeouts (read/write/idle).
- RFC 5987 encoding for non-ASCII filenames in `Content-Disposition`.
- The `data/` directory is never served statically; files are only reachable via the `/d/<id>` handler.
