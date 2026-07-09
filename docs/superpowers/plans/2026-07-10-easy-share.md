# easy-share Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single-binary Go file-hosting app: drag & drop a file, see upload progress, get a shareable URL. Japanese UI, 30-day auto-expiry, deployable via Docker Compose or Railpack.

**Architecture:** One Go binary serves an embedded frontend (`//go:embed`), a tiny JSON API (`POST /api/upload`, `GET /d/{id}`), and a background cleanup goroutine. Files + JSON sidecar metadata are stored on local disk. Upload progress is reported client-side via `XMLHttpRequest.upload.onprogress`.

**Tech Stack:** Go 1.26 stdlib only (no third-party deps), vanilla JS/CSS frontend, Docker multi-stage build, Railpack zero-config.

**Reference spec:** `docs/superpowers/specs/2026-07-10-easy-share-design.md`

---

## File Structure

- **Create:** `go.mod` — module `github.com/rafazafar/easy-share`
- **Create:** `.gitignore` — ignore `data/`, built binaries, OS junk
- **Create:** `internal/config/config.go` — env-based configuration
- **Create:** `internal/config/config_test.go` — config tests
- **Create:** `internal/store/store.go` — disk storage: ID gen, Save, Meta, Open, Delete, Cleanup
- **Create:** `internal/store/store_test.go` — store tests
- **Create:** `internal/server/server.go` — HTTP handlers + routing + cleanup loop + server lifecycle
- **Create:** `internal/server/server_test.go` — handler tests
- **Create:** `web/index.html` — embedded single-page UI
- **Create:** `web/style.css` — embedded cute/pastel styles
- **Create:** `web/app.js` — embedded upload client (XHR progress)
- **Create:** `main.go` — embed frontend, wire config/server/cleanup, run
- **Create:** `Dockerfile` — multi-stage build → alpine runtime
- **Create:** `docker-compose.yml` — service + volume + ports + env
- **Create:** `railpack.json` — explicit Go start command
- **Create:** `README.md` — Japanese/English docs, run + deploy

---

## Task 1: Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `.gitignore`

- [ ] **Step 1: Create `go.mod`**

```
module github.com/rafazafar/easy-share

go 1.26
```

- [ ] **Step 2: Create `.gitignore`**

```
/data/
/easy-share
/out
*.exe
.DS_Store
```

- [ ] **Step 3: Verify module resolves**

Run: `go mod tidy`
Expected: no output, no error (no external deps).

- [ ] **Step 4: Commit**

```bash
git add go.mod .gitignore
git commit -m "chore: scaffold go module and gitignore"
```

---

## Task 2: Config Package (TDD)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Responsibility:** Load app config from environment variables with sensible defaults.

- [ ] **Step 1: Write the failing test**

`internal/config/config_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{"PORT", "DATA_DIR", "MAX_UPLOAD_BYTES", "RETENTION", "BASE_URL"} {
		t.Setenv(k, "")
	}
	cfg := Load()
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("DataDir = %q, want ./data", cfg.DataDir)
	}
	if cfg.MaxUploadBytes != 100*1024*1024 {
		t.Fatalf("MaxUploadBytes = %d, want %d", cfg.MaxUploadBytes, 100*1024*1024)
	}
	if cfg.Retention != 30*24*time.Hour {
		t.Fatalf("Retention = %v, want %v", cfg.Retention, 30*24*time.Hour)
	}
	if cfg.BaseURL != "" {
		t.Fatalf("BaseURL = %q, want empty", cfg.BaseURL)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATA_DIR", "/tmp/share")
	t.Setenv("MAX_UPLOAD_BYTES", "1234")
	t.Setenv("RETENTION", "48h")
	t.Setenv("BASE_URL", "https://share.example.com")
	cfg := Load()
	if cfg.Port != "9090" || cfg.DataDir != "/tmp/share" || cfg.MaxUploadBytes != 1234 ||
		cfg.Retention != 48*time.Hour || cfg.BaseURL != "https://share.example.com" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — `config.Load` undefined (package not built).

- [ ] **Step 3: Write minimal implementation**

`internal/config/config.go`:

```go
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration loaded from the environment.
type Config struct {
	Port           string
	DataDir        string
	MaxUploadBytes int64
	Retention      time.Duration
	BaseURL        string
}

// Load reads configuration from environment variables, applying defaults.
func Load() Config {
	return Config{
		Port:           getenv("PORT", "8080"),
		DataDir:        getenv("DATA_DIR", "./data"),
		MaxUploadBytes: getenvInt64("MAX_UPLOAD_BYTES", 100*1024*1024),
		Retention:      getenvDuration("RETENTION", 30*24*time.Hour),
		BaseURL:        os.Getenv("BASE_URL"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS (`ok ... internal/config`).

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add env-based config package"
```

---

## Task 3: Store Package — ID, Save, Meta, Open, Delete (TDD)

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

**Responsibility:** On-disk file + metadata storage. Layout: `data/files/<id>` (bytes), `data/meta/<id>.json` (JSON sidecar).

- [ ] **Step 1: Write the failing tests**

`internal/store/store_test.go`:

```go
package store

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewID(t *testing.T) {
	seen := make(map[string]bool, 10000)
	for i := 0; i < 10000; i++ {
		id := NewID()
		if len(id) != 12 {
			t.Fatalf("id length = %d, want 12", len(id))
		}
		if seen[id] {
			t.Fatalf("collision after %d ids: %s", i, id)
		}
		seen[id] = true
	}
}

func TestNewCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"files", "meta"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil || !info.IsDir() {
			t.Fatalf("expected dir %s to exist", sub)
		}
	}
	_ = s
}

func TestSaveAndOpen(t *testing.T) {
	s, _ := New(t.TempDir())
	meta, err := s.Save("hello.txt", "text/plain", strings.NewReader("hello world"), -1)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Size != 11 {
		t.Fatalf("Size = %d, want 11", meta.Size)
	}
	if meta.ID == "" || meta.Filename != "hello.txt" || meta.ContentType != "text/plain" {
		t.Fatalf("unexpected meta: %+v", meta)
	}

	got, err := s.Meta(meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "hello.txt" {
		t.Fatalf("meta filename = %q", got.Filename)
	}

	f, err := s.Open(meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	if string(b) != "hello world" {
		t.Fatalf("content = %q", string(b))
	}
}

func TestMetaMissing(t *testing.T) {
	s, _ := New(t.TempDir())
	if _, err := s.Meta("nope"); err == nil {
		t.Fatal("expected error for missing meta")
	}
}

func TestDelete(t *testing.T) {
	s, _ := New(t.TempDir())
	meta, _ := s.Save("a.txt", "text/plain", strings.NewReader("a"), -1)
	if err := s.Delete(meta.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Meta(meta.ID); err == nil {
		t.Fatal("expected meta gone after delete")
	}
	if _, err := s.Open(meta.ID); err == nil {
		t.Fatal("expected file gone after delete")
	}
}

// writeBackdatedMeta writes a sidecar with an old CreatedAt for cleanup tests.
func writeBackdatedMeta(t *testing.T, s *Store, id string, age time.Duration) {
	t.Helper()
	m := &Meta{
		ID:          id,
		Filename:    "old.txt",
		ContentType: "text/plain",
		Size:        3,
		CreatedAt:   time.Now().UTC().Add(-age),
	}
	b, _ := os.ReadFile(s.metaPath(id)) // ensure path helper exists
	_ = b
	data, err := jsonMarshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.metaPath(id), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
```

> Note: the helper above calls unexported `s.metaPath` and a `jsonMarshal` helper, both defined in `store.go` in Step 3.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/`
Expected: FAIL — `store.New`, `store.NewID` etc. undefined.

- [ ] **Step 3: Write minimal implementation**

`internal/store/store.go`:

```go
package store

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	idLen     = 12
	idAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// ErrNotFound is returned when no metadata exists for an id.
var ErrNotFound = errors.New("not found")

// Meta is the JSON sidecar metadata for a stored file.
type Meta struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"contentType"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Store manages files + metadata on local disk under root.
type Store struct {
	root string
}

// New creates the directory structure and returns a Store.
func New(root string) (*Store, error) {
	s := &Store{root: root}
	for _, sub := range []string{"files", "meta"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// NewID returns a random 12-char base62 id (~71 bits entropy).
func NewID() string {
	b := make([]byte, idLen)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand should not fail
	}
	for i := range b {
		b[i] = idAlphabet[int(b[i])%len(idAlphabet)]
	}
	return string(b)
}

// Save streams r to disk and writes a metadata sidecar. It returns the Meta.
func (s *Store) Save(filename, contentType string, r io.Reader, _ int64) (*Meta, error) {
	id := NewID()
	filePath := s.filePath(id)
	f, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(filePath)
		return nil, err
	}
	meta := &Meta{
		ID:          id,
		Filename:    filename,
		ContentType: contentType,
		Size:        n,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.writeMeta(meta); err != nil {
		os.Remove(filePath)
		return nil, err
	}
	return meta, nil
}

// Meta reads the metadata sidecar for an id.
func (s *Store) Meta(id string) (*Meta, error) {
	data, err := os.ReadFile(s.metaPath(id))
	if err != nil {
		return nil, ErrNotFound
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Open opens the stored file bytes for reading.
func (s *Store) Open(id string) (*os.File, error) {
	return os.Open(s.filePath(id))
}

// Delete removes the file and its metadata. Missing files are not an error.
func (s *Store) Delete(id string) error {
	os.Remove(s.metaPath(id))
	if err := os.Remove(s.filePath(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) writeMeta(m *Meta) error {
	data, err := jsonMarshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(s.metaPath(m.ID), data, 0o644)
}

func jsonMarshal(m *Meta) ([]byte, error) { return json.Marshal(m) }

func (s *Store) filePath(id string) string { return filepath.Join(s.root, "files", id) }
func (s *Store) metaPath(id string) string { return filepath.Join(s.root, "meta", id+".json") }

// init is intentionally omitted; see Cleanup in Task 4.
var _ = strings.TrimSpace
```

> The trailing `var _ = strings.TrimSpace` avoids an unused-import error until Task 4 uses `strings`; remove it in Task 4 once `strings` is actually used. (Alternatively, remove the `strings` import here and re-add it in Task 4.) **Implementation note:** to keep the build green at this task, do NOT import `strings` yet — drop the `strings` import and the `var _` line; they are re-introduced in Task 4.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: add disk store with id/save/meta/open/delete"
```

---

## Task 4: Store Package — Cleanup (TDD)

**Files:**
- Modify: `internal/store/store.go` (add `Cleanup`)
- Modify: `internal/store/store_test.go` (add cleanup test)

**Responsibility:** Delete files whose `CreatedAt` is older than the retention window.

- [ ] **Step 1: Add the failing test**

Append to `internal/store/store_test.go`:

```go
func TestCleanup(t *testing.T) {
	s, _ := New(t.TempDir())
	meta, _ := s.Save("old.txt", "text/plain", strings.NewReader("old"), -1)
	writeBackdatedMeta(t, s, meta.ID, 48*time.Hour)

	// fresh file should survive
	fresh, _ := s.Save("new.txt", "text/plain", strings.NewReader("new"), -1)

	deleted := s.Cleanup(24 * time.Hour)
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := s.Meta(meta.ID); err == nil {
		t.Fatal("expected old meta deleted")
	}
	if _, err := s.Open(meta.ID); err == nil {
		t.Fatal("expected old file deleted")
	}
	if _, err := s.Meta(fresh.ID); err != nil {
		t.Fatal("expected fresh meta to remain")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/`
Expected: FAIL — `s.Cleanup` undefined.

- [ ] **Step 3: Implement Cleanup**

Add to `internal/store/store.go` (also import `"strings"` and remove the placeholder `var _` line from Task 3):

```go
// Cleanup deletes files older than maxAge. Returns the count removed.
func (s *Store) Cleanup(maxAge time.Duration) int {
	entries, err := os.ReadDir(filepath.Join(s.root, "meta"))
	if err != nil {
		return 0
	}
	now := time.Now()
	deleted := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		m, err := s.Meta(id)
		if err != nil {
			continue
		}
		if now.Sub(m.CreatedAt) > maxAge {
			if err := s.Delete(id); err == nil {
				deleted++
			}
		}
	}
	return deleted
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: add retention cleanup to store"
```

---

## Task 5: Server Package — Upload + Download Handlers (TDD)

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

**Responsibility:** HTTP routing + handlers. `POST /api/upload` streams the request body to disk with a size cap (`http.MaxBytesReader` → 413); returns JSON. `GET /d/{id}` serves the file inline with correct headers. Static files served from an injected `fs.FS`.

- [ ] **Step 1: Write the failing tests**

`internal/server/server_test.go`:

```go
package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"testing/fstest"

	"github.com/rafazafar/easy-share/internal/config"
)

func newTestServer(t *testing.T, maxBytes int64) *Server {
	t.Helper()
	cfg := config.Config{
		Port:           "0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: maxBytes,
		Retention:      720e9, // 30d in ns (only used by cleanup, not here)
	}
	srv, err := New(cfg, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestUploadAndDownload(t *testing.T) {
	srv := newTestServer(t, 1024)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload", bytes.NewReader([]byte("hello")))
	req.Header.Set("X-Filename", url.QueryEscape("テスト.txt"))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}
	var res struct {
		ID       string `json:"id"`
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if res.ID == "" || res.Size != 5 || res.Filename != "テスト.txt" {
		t.Fatalf("unexpected response: %+v", res)
	}
	if res.URL == "" {
		t.Fatal("expected non-empty url")
	}

	resp2, err := http.Get(ts.URL + "/d/" + res.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d", resp2.StatusCode)
	}
	if ct := resp2.Header.Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("content-type = %q", ct)
	}
	b, _ := io.ReadAll(resp2.Body)
	if string(b) != "hello" {
		t.Fatalf("body = %q", string(b))
	}
}

func TestUploadTooLarge(t *testing.T) {
	srv := newTestServer(t, 5)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload", bytes.NewReader([]byte("1234567890")))
	req.Header.Set("X-Filename", "big.bin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}

func TestDownloadMissing(t *testing.T) {
	srv := newTestServer(t, 1024)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/d/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/`
Expected: FAIL — `server.New`, `server.Server`, `srv.Handler()` undefined.

- [ ] **Step 3: Write minimal implementation**

`internal/server/server.go`:

```go
package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rafazafar/easy-share/internal/config"
	"github.com/rafazafar/easy-share/internal/store"
)

// Server wires the store and static frontend into HTTP handlers.
type Server struct {
	cfg     config.Config
	store   *store.Store
	static  fs.FS
	handler http.Handler
}

// New initializes the store and builds the router.
func New(cfg config.Config, static fs.FS) (*Server, error) {
	st, err := store.New(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, store: st, static: static}
	s.handler = s.routes()
	return s, nil
}

// Handler returns the configured HTTP handler (for testing + ListenAndServe).
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/upload", s.handleUpload)
	mux.HandleFunc("GET /d/{id}", s.handleDownload)
	mux.Handle("/", http.FileServer(http.FS(s.static)))
	return mux
}

// StartCleanup runs an immediate cleanup then sweeps every hour.
func (s *Server) StartCleanup(maxAge time.Duration) {
	go func() {
		s.store.Cleanup(maxAge)
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			s.store.Cleanup(maxAge)
		}
	}()
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes)

	filename, err := url.QueryUnescape(r.Header.Get("X-Filename"))
	if err != nil || filename == "" {
		filename = "file"
	}
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	meta, err := s.store.Save(filename, contentType, r.Body, -1)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "ファイルサイズが大きすぎます（最大100MB）", http.StatusRequestEntityTooLarge)
			return
		}
		log.Printf("upload failed: %v", err)
		http.Error(w, "アップロードに失敗しました", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":       meta.ID,
		"url":      s.shareURL(r, meta.ID),
		"filename": meta.Filename,
		"size":     meta.Size,
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	meta, err := s.store.Meta(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := s.store.Open(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+url.PathEscape(meta.Filename))
	w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
	_, _ = io.Copy(w, f)
}

func (s *Server) shareURL(r *http.Request, id string) string {
	if s.cfg.BaseURL != "" {
		return strings.TrimRight(s.cfg.BaseURL, "/") + "/d/" + id
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/d/" + id
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:              ":" + s.cfg.Port,
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("easy-share listening on :%s", s.cfg.Port)
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/
git commit -m "feat: add http server with upload/download handlers"
```

---

## Task 6: Frontend (HTML/CSS/JS)

**Files:**
- Create: `web/index.html`
- Create: `web/style.css`
- Create: `web/app.js`

**Responsibility:** Single responsive Japanese page with 4 states (idle/uploading/done/error). Drag & drop + click-to-select, XHR upload progress, copy-to-clipboard, mobile-friendly.

- [ ] **Step 1: Create `web/index.html`**

```html
<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>easy-share</title>
  <link rel="stylesheet" href="/style.css">
</head>
<body>
  <main class="card">
    <header class="brand">
      <h1><span class="logo">📎</span> easy-share</h1>
      <p class="subtitle">かんたんファイル共有</p>
    </header>

    <!-- IDLE -->
    <section id="idle" class="state">
      <label id="dropzone" class="dropzone" for="fileinput">
        <div class="dz-icon">📤</div>
        <div class="dz-text">ここにファイルをドロップ</div>
        <div class="dz-sub">または クリックして選択</div>
        <input id="fileinput" type="file" hidden>
      </label>
      <div class="hint">最大 100MB</div>
    </section>

    <!-- UPLOADING -->
    <section id="uploading" class="state hidden">
      <div class="filename" id="up-filename"></div>
      <div class="progress"><div id="bar" class="bar"></div></div>
      <div class="pct" id="pct">0%</div>
    </section>

    <!-- DONE -->
    <section id="done" class="state hidden">
      <div class="check">✅</div>
      <div class="done-text">アップロード完了！</div>
      <div class="url-box">
        <input id="shareurl" readonly>
        <button id="copybtn" type="button">コピー</button>
      </div>
      <button id="again" class="link-btn" type="button">別のファイルをアップロード</button>
    </section>

    <!-- ERROR -->
    <section id="error" class="state hidden">
      <div class="err-icon">⚠️</div>
      <div id="err-text" class="err-text"></div>
      <button id="retry" class="link-btn" type="button">もう一度</button>
    </section>
  </main>
  <script src="/app.js"></script>
</body>
</html>
```

- [ ] **Step 2: Create `web/style.css`**

```css
:root {
  --bg1: #fdf2f8;
  --bg2: #ecfeff;
  --accent: #a78bfa;
  --accent-dark: #7c3aed;
  --ink: #3b3650;
  --muted: #8b86a0;
  --card: #ffffff;
  --ok: #34d399;
  --err: #fb7185;
  --radius: 18px;
}

* { box-sizing: border-box; }

body {
  margin: 0;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.25rem;
  font-family: "Hiragino Kaku Gothic ProN", "Noto Sans JP", system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  color: var(--ink);
  background: linear-gradient(135deg, var(--bg1), var(--bg2));
}

.card {
  width: 100%;
  max-width: 440px;
  background: var(--card);
  border-radius: 24px;
  padding: 2rem 1.75rem;
  box-shadow: 0 20px 45px rgba(80, 70, 120, 0.12);
}

.brand { text-align: center; margin-bottom: 1.5rem; }
.brand h1 { margin: 0; font-size: 1.6rem; font-weight: 700; }
.logo { font-size: 1.8rem; vertical-align: middle; }
.subtitle { margin: .35rem 0 0; color: var(--muted); font-size: .95rem; }

.state { animation: pop .25s ease; }
.hidden { display: none; }

.dropzone {
  display: block;
  border: 2px dashed var(--accent);
  border-radius: var(--radius);
  padding: 2.5rem 1rem;
  text-align: center;
  cursor: pointer;
  transition: background .15s ease, transform .15s ease, border-color .15s ease;
  background: #faf7ff;
}
.dropzone:hover, .dropzone.drag {
  background: #f1ebff;
  border-color: var(--accent-dark);
  transform: translateY(-1px);
}
.dz-icon { font-size: 2.4rem; }
.dz-text { font-weight: 600; margin-top: .5rem; }
.dz-sub { color: var(--muted); font-size: .9rem; margin-top: .25rem; }
.hint { text-align: center; color: var(--muted); font-size: .82rem; margin-top: .9rem; }

.filename { text-align: center; font-weight: 600; margin-bottom: 1rem; word-break: break-all; }
.progress {
  height: 14px;
  background: #eeeaf7;
  border-radius: 999px;
  overflow: hidden;
}
.bar {
  height: 100%;
  width: 0%;
  background: linear-gradient(90deg, var(--accent), var(--accent-dark));
  border-radius: 999px;
  transition: width .15s ease;
}
.pct { text-align: center; color: var(--muted); margin-top: .6rem; font-variant-numeric: tabular-nums; }

.check { font-size: 2.6rem; text-align: center; }
.done-text { text-align: center; font-weight: 600; margin-bottom: 1rem; }
.url-box { display: flex; gap: .5rem; margin-bottom: 1rem; }
.url-box input {
  flex: 1;
  padding: .65rem .75rem;
  border: 1px solid #e4e0f0;
  border-radius: 12px;
  font-size: .9rem;
  color: var(--ink);
  background: #faf9ff;
}
#copybtn {
  flex: 0 0 auto;
  padding: .65rem 1rem;
  border: none;
  border-radius: 12px;
  background: var(--accent-dark);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.link-btn {
  width: 100%;
  padding: .7rem;
  border: 1px solid var(--accent);
  background: transparent;
  color: var(--accent-dark);
  border-radius: 12px;
  font-weight: 600;
  cursor: pointer;
}
.link-btn:hover { background: #f3eeff; }

.err-icon { font-size: 2.6rem; text-align: center; }
.err-text { text-align: center; color: var(--err); font-weight: 600; margin: .5rem 0 1.25rem; }

@keyframes pop {
  from { opacity: 0; transform: translateY(6px); }
  to   { opacity: 1; transform: translateY(0); }
}
```

- [ ] **Step 3: Create `web/app.js`**

```js
(function () {
  "use strict";

  var $ = function (id) { return document.getElementById(id); };
  var STATES = ["idle", "uploading", "done", "error"];
  var MAX = 100 * 1024 * 1024;

  function show(name) {
    STATES.forEach(function (s) { $(s).classList.toggle("hidden", s !== name); });
  }
  function fail(msg) { $("err-text").textContent = msg; show("error"); }
  function reset() { $("fileinput").value = ""; show("idle"); }

  function upload(file) {
    if (file.size > MAX) { fail("ファイルサイズが大きすぎます（最大100MB）"); return; }
    show("uploading");
    $("up-filename").textContent = file.name;
    $("bar").style.width = "0%";
    $("pct").textContent = "0%";

    var xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/upload");
    xhr.setRequestHeader("X-Filename", encodeURIComponent(file.name));
    xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");

    xhr.upload.onprogress = function (e) {
      if (e.lengthComputable) {
        var p = Math.round((e.loaded / e.total) * 100);
        $("bar").style.width = p + "%";
        $("pct").textContent = p + "%";
      }
    };
    xhr.onload = function () {
      if (xhr.status >= 200 && xhr.status < 300) {
        var res = JSON.parse(xhr.responseText);
        var url = res.url || (location.origin + "/d/" + res.id);
        $("shareurl").value = url;
        show("done");
      } else {
        fail(xhr.status === 413
          ? "ファイルサイズが大きすぎます（最大100MB）"
          : "アップロードに失敗しました");
      }
    };
    xhr.onerror = function () { fail("通信エラーが発生しました"); };
    xhr.send(file);
  }

  $("fileinput").addEventListener("change", function (e) {
    if (e.target.files[0]) upload(e.target.files[0]);
  });

  var dz = $("dropzone");
  ["dragenter", "dragover"].forEach(function (ev) {
    dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.add("drag"); });
  });
  ["dragleave", "drop"].forEach(function (ev) {
    dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.remove("drag"); });
  });
  dz.addEventListener("drop", function (e) {
    var f = e.dataTransfer.files[0];
    if (f) upload(f);
  });

  $("retry").addEventListener("click", reset);
  $("again").addEventListener("click", reset);

  $("copybtn").addEventListener("click", function () {
    var input = $("shareurl");
    var btn = $("copybtn");
    var done = function () { btn.textContent = "コピーしました！"; setTimeout(function () { btn.textContent = "コピー"; }, 2000); };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(input.value).then(done, function () { input.select(); document.execCommand("copy"); done(); });
    } else {
      input.select(); document.execCommand("copy"); done();
    }
  });
})();
```

- [ ] **Step 4: Commit**

```bash
git add web/
git commit -m "feat: add japanese drag-drop frontend"
```

---

## Task 7: main.go + Embed

**Files:**
- Create: `main.go`

**Responsibility:** Embed `web/`, load config, build server, start cleanup loop, run.

- [ ] **Step 1: Create `main.go`**

```go
package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/rafazafar/easy-share/internal/config"
	"github.com/rafazafar/easy-share/internal/server"
)

//go:embed web
var webFS embed.FS

func main() {
	cfg := config.Load()

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("embed: %v", err)
	}

	srv, err := server.New(cfg, static)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	srv.StartCleanup(cfg.Retention)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
```

- [ ] **Step 2: Build and run the whole suite**

Run: `go build ./...`
Expected: builds with no errors.

Run: `go test ./...`
Expected: all packages PASS.

Run a smoke test:
```bash
go run . &
sleep 1
curl -s -X POST http://localhost:8080/api/upload -H "X-Filename: $(printf '%s' 'smile.txt' | jq -sRr @uri)" -H "Content-Type: text/plain" --data-binary "hello :)"
kill %1
```
Expected: JSON response with non-empty `id` and `url`.

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: wire main with embedded frontend and cleanup"
```

---

## Task 8: Deployment Files

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `railpack.json`

- [ ] **Step 1: Create `Dockerfile`** (multi-stage → alpine)

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /easy-share .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /easy-share /app/easy-share
RUN mkdir -p /data
ENV DATA_DIR=/data PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/easy-share"]
```

- [ ] **Step 2: Create `docker-compose.yml`**

```yaml
services:
  app:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    environment:
      DATA_DIR: /data
      PORT: "8080"
      # BASE_URL: "https://your-domain.com"
      # MAX_UPLOAD_BYTES: "104857600"
      # RETENTION: "720h"
    restart: unless-stopped
```

- [ ] **Step 3: Create `railpack.json`** (explicit start command; zero-config Go build produces binary `out`)

```json
{
  "$schema": "https://schema.railpack.com",
  "deploy": {
    "startCommand": "./out"
  }
}
```

- [ ] **Step 4: Verify Docker build + run**

Run:
```bash
docker compose build
docker compose up -d
sleep 2
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/
curl -s -X POST http://localhost:8080/api/upload -H "X-Filename: hi.txt" -H "Content-Type: text/plain" --data-binary "hello"
docker compose down
```
Expected: `200` for the homepage; JSON for the upload.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile docker-compose.yml railpack.json
git commit -m "build: add docker and railpack deployment configs"
```

---

## Task 9: README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Create `README.md`**

````markdown
# 📎 easy-share

かんたんファイル共有 — ドラッグ＆ドロップでファイルをアップロード、完了したら共有URLをコピー。

A super simple self-hosted file-sharing app: drag & drop a file, watch the progress bar, copy the share link. Japanese UI. Files auto-expire after 30 days.

- Single Go binary, stdlib only (no third-party deps)
- Files stored on local disk; metadata as JSON sidecars
- 30-day auto-cleanup
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

Uploads are persisted in `./data`. Configure `BASE_URL` in `docker-compose.yml` for production share URLs.

## Deploy with Railpack

Railpack auto-detects Go and builds the static binary. The included `railpack.json` sets the start command:

```bash
railpack build -t easy-share .
```

## Tests

```bash
go test ./...
```
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README"
```

---

## Self-Review Checklist (run after writing this plan)

1. **Spec coverage:** config ✓ (T2), store id/save/meta/open/delete ✓ (T3), cleanup ✓ (T4), routes upload/download/static ✓ (T5), progress (XHR) ✓ (T6), frontend 4 states Japanese ✓ (T6), embed + wiring ✓ (T7), cleanup goroutine ✓ (T5+T7), size cap MaxBytesReader→413 ✓ (T5), docker ✓ (T8), railpack ✓ (T8), retention 30d ✓ (T2 default + T4). All spec sections covered.
2. **Placeholder scan:** none — all code blocks complete.
3. **Type consistency:** `config.Config` fields (Port, DataDir, MaxUploadBytes, Retention, BaseURL) used consistently across T2/T5/T7. `store.New/Save/Meta/Open/Delete/Cleanup/NewID` signatures consistent across T3/T4/T5. `server.New(cfg, fs.FS) (*Server, error)`, `Handler() http.Handler`, `StartCleanup(time.Duration)`, `ListenAndServe() error` consistent across T5/T7.
```
