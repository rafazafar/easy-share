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
