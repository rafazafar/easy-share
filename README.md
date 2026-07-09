# 📎 easy-share

かんたんファイル共有 — ドラッグ＆ドロップでファイルをアップロード、完了したら共有URLをコピー。

A super simple self-hosted file-sharing app: drag & drop a file, watch the progress bar, copy the share link. Japanese UI. Files auto-expire after 30 days.

- Single Go binary, stdlib only (no third-party deps)
- Files stored on local disk; metadata as JSON sidecars
- 30-day auto-cleanup background job
- Deployable via **Docker Compose** or **Railpack**

## Run locally

```bash
go run .
# open http://localhost:8080
```

## Configuration (env vars)

| Variable           | Default        | Description                                |
|--------------------|----------------|--------------------------------------------|
| `PORT`             | `8080`         | Listen port                                |
| `DATA_DIR`         | `./data`       | Storage root (`files/` + `meta/` subdirs)  |
| `MAX_UPLOAD_BYTES` | `104857600`    | 100 MiB upload cap                         |
| `RETENTION`        | `720h`         | File lifetime (30 days)                    |
| `BASE_URL`         | _(empty)_      | Absolute origin for share URLs             |

## Deploy with Docker Compose

```bash
docker compose up -d
```

Uploads are persisted in a named volume. Override any setting via environment variables or a `.env` file (see `.env.example`) — e.g. set `BASE_URL` to your domain for correct share URLs.

## Deploy with Railpack

Railpack auto-detects Go and builds the static binary. The included `railpack.json` sets the start command:

```bash
railpack build -t easy-share .
```

## Tests

```bash
go test ./...
```
