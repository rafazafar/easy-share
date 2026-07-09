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
	"time"

	"github.com/rafazafar/easy-share/internal/config"
)

func newTestServer(t *testing.T, maxBytes int64) *Server {
	t.Helper()
	cfg := config.Config{
		Port:           "0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: maxBytes,
		Retention:      30 * 24 * time.Hour,
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
