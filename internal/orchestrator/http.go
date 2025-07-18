package orchestrator

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"distro.lol/internal/orchestrator/orchestrator"
)

type httpServer struct {
	// Add fields for HTTP server configuration, handlers, etc.
	orchestrator *orchestrator.Orchestrator
}

func (s *httpServer) start(ctx context.Context, errChan chan error) {
	mux := http.NewServeMux()

	// Client REST endpoints
	mux.HandleFunc("/upload", s.handleUpload)
	mux.HandleFunc("/download/", s.handleDownload) // /download/{objectID}

	addr := fmt.Sprintf(":%d", s.orchestrator.GetConfig().HTTPPort)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting HTTP server on port %d", s.orchestrator.GetConfig().HTTPPort)
		errChan <- fmt.Errorf("http server failed: %w", server.ListenAndServe()) // Start the HTTP server
	}()

	// Wait for context cancellation and gracefully stop the server
	go func() {
		<-ctx.Done()
		log.Printf("Context cancelled, stopping HTTP server")
		server.Shutdown(context.Background())
	}()
}

func (s *httpServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form data
	err := r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	// Process the file upload...
}

func (s *httpServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	objectID := strings.TrimPrefix(r.URL.Path, "/download/")
	if objectID == "" {
		http.Error(w, "Object ID is required", http.StatusBadRequest)
		return
	}

	// Retrieve the object and send it in the response...
}
