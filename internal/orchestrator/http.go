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

	// Health check endpoint
	mux.HandleFunc("/health", s.handleHealth)

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
		log.Printf("Failed to distribute file: %v", err)
		http.Error(w, "Failed to distribute file", http.StatusInternalServerError)
		return
	}

	responseBytes, err := json.Marshal(uploadResponse{ObjectID: id})
	if err != nil {
		http.Error(w, "Failed to create response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBytes)

	// Process the file upload...
}

func (s *httpServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	objectID := strings.TrimPrefix(r.URL.Path, "/download/")
	if objectID == "" {
		http.Error(w, "Object ID is required", http.StatusBadRequest)
		return
	}

	log.Printf("Received download request for object: %s", objectID)

	// Get object metadata first to determine filename
	objectMetadata, err := s.orchestrator.GetStorageManager().GetObjectMetadata(r.Context(), objectID)
	if err != nil {
		log.Printf("Failed to get object metadata for %s: %v", objectID, err)
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}

	// Retrieve and reconstruct the object
	fileData, err := s.orchestrator.GetObject(r.Context(), objectID)
	if err != nil {
		log.Printf("Failed to get object %s: %v", objectID, err)
		http.Error(w, "Failed to retrieve object", http.StatusInternalServerError)
		return
	}

	// Set appropriate headers for file download
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, objectMetadata.Filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileData)))

	// Write the file data to response
	bytesWritten, err := w.Write(fileData)
	if err != nil {
		log.Printf("Failed to write file data for object %s: %v", objectID, err)
		return
	}

	log.Printf("Successfully served object %s (%s) - %d bytes", objectID, objectMetadata.Filename, bytesWritten)
}

type uploadRequest struct {
	Filename string `json:"filename"`
	File     []byte `json:"file"`
}

type uploadResponse struct {
	ObjectID string `json:"object_id"`
}

func (s *httpServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simple health check - verify orchestrator is running
	status := map[string]any{
		"status":  "healthy",
		"service": "orchestrator",
	}

	// Optional: Add basic component health checks
	// Check if storage manager is accessible
	if s.orchestrator.GetStorageManager() == nil {
		status["status"] = "unhealthy"
		status["error"] = "storage manager not available"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
