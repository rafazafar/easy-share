package store

import (
	"encoding/json"
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

// writeBackdatedMeta overwrites a sidecar with an old CreatedAt for cleanup tests.
func writeBackdatedMeta(t *testing.T, s *Store, id string, age time.Duration) {
	t.Helper()
	m := &Meta{
		ID:          id,
		Filename:    "old.txt",
		ContentType: "text/plain",
		Size:        3,
		CreatedAt:   time.Now().UTC().Add(-age),
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.metaPath(id), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
