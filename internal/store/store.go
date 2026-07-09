package store

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	idLen      = 12
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
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(s.metaPath(m.ID), data, 0o644)
}

func (s *Store) filePath(id string) string { return filepath.Join(s.root, "files", id) }
func (s *Store) metaPath(id string) string { return filepath.Join(s.root, "meta", id+".json") }
