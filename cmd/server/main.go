package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"charmtool/internal/handlers"
	"charmtool/internal/storage"
)

const (
	defaultPort    = "8080"
	sessionMaxAge  = 10 * 24 * time.Hour // 10 days
	cleanupInterval = 1 * time.Hour
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// Initialize file storage
	dataDir := filepath.Join(".", "data", "sessions")
	store, err := storage.NewFileStore(dataDir, sessionMaxAge)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Start cleanup goroutine
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := store.Cleanup(); err != nil {
				log.Printf("Cleanup error: %v", err)
			}
		}
	}()

	// Create handler with storage
	h := handlers.New(store)

	// Setup routes
	mux := http.NewServeMux()

	// API routes (session middleware applied)
	mux.Handle("/api/upload/pos", h.SessionMiddleware(http.HandlerFunc(h.UploadPOS)))
	mux.Handle("/api/upload/stack", h.SessionMiddleware(http.HandlerFunc(h.UploadStack)))
	mux.Handle("/api/xfile", h.SessionMiddleware(http.HandlerFunc(h.GetXFile)))
	mux.Handle("/api/xfile/update", h.SessionMiddleware(http.HandlerFunc(h.UpdateXFile)))
	mux.Handle("/api/export", h.SessionMiddleware(http.HandlerFunc(h.Export)))
	mux.Handle("/api/validate", h.SessionMiddleware(http.HandlerFunc(h.Validate)))
	mux.Handle("/api/stacks/export", h.SessionMiddleware(http.HandlerFunc(h.StacksExport)))
	mux.Handle("/api/stacks/import", h.SessionMiddleware(http.HandlerFunc(h.StacksImport)))

	// Static files
	staticDir := filepath.Join(".", "web", "static")
	mux.Handle("/", http.FileServer(http.Dir(staticDir)))

	log.Printf("CharmTool server starting on port %s", port)
	log.Printf("Open http://localhost:%s in your browser", port)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
