package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"distro.lol/internal/orchestrator/logic"
)

type httpServer struct {
	// Add fields for HTTP server configuration, handlers, etc.
	orchestrator *logic.Orchestrator
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
	err := r.ParseMultipartForm(100 << 20) // 32MB max memory
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	log.Printf("Received file upload: %s (%d MB)", header.Filename, header.Size/(1024*1024))

	id, err := s.orchestrator.DistributeFile(r.Context(), fileBytes, header.Filename, header.Size)
	if err != nil {
		http.Error(w, "Failed to distribute file", http.StatusInternalServerError)
		return
	}

	responseBytes, err := json.Marshal(uploadResponse{objectID: id})
	if err != nil {
		http.Error(w, "Failed to create response", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBytes)

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

type uploadRequest struct {
	filename string `json:"filename"`
	file     []byte `json:"file"`
}

type uploadResponse struct {
	objectID string `json:"object_id"`
}
