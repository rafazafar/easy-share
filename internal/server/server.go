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
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("POST /api/upload", s.handleUpload)
	mux.HandleFunc("GET /d/{id}", s.handleDownload)
	mux.Handle("/", http.FileServer(http.FS(s.static)))
	return mux
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"maxUploadBytes": s.cfg.MaxUploadBytes,
		"retentionHours": int64(s.cfg.Retention.Hours()),
	})
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
