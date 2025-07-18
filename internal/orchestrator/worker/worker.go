package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/supabase-community/supabase-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Manager interface {
	// RegisterWorker registers a new worker with the manager
	RegisterWorker(ctx context.Context, workerID, address string, capacity int) error
	// GetWorker retrieves a worker by ID
	GetWorker(ctx context.Context, workerID string) (*Worker, error)
	// ListWorkers lists all registered workers
	ListWorkers(ctx context.Context) ([]*Worker, error)
	// UpdateWorker updates the worker's status or capacity
	UpdateWorker(ctx context.Context, workerID string, capacity int, status Status) error
	// RemoveWorker removes a worker from the manager
	RemoveWorker(ctx context.Context, workerID string) error
	// Start starts the worker manager
	Start() error
	// Stop stops the worker manager
	Stop() error
}

type workerManager struct {
	ctx                context.Context
	sbClient           *supabase.Client
	mu                 *sync.RWMutex
	workers            map[string]*Worker // workerID -> Info
	workerSyncInterval time.Duration
}

func NewManager(ctx context.Context, workerSyncInterval time.Duration) *workerManager {
	sbClient, err := newSupabaseClient()
	if err != nil {
		log.Fatalf("Failed to create Supabase client: %v", err)
	}
	return &workerManager{
		ctx:                ctx,
		sbClient:           sbClient,
		mu:                 &sync.RWMutex{},
		workers:            make(map[string]*Worker),
		workerSyncInterval: workerSyncInterval,
	}
}

func (wm *workerManager) Start() error {
	log.Print("Starting worker manager...")

	// Query sbWorkers from Supabase with only the columns we need
	sbWorkers, err := wm.getAllWorkersFromSB()
	if err != nil {
		log.Fatalf("Failed to get online workers from Supabase: %v", err)
	}

	log.Printf("Loaded %d workers from database", len(sbWorkers))

	// Populate the local worker map and establish gRPC connections
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for _, sbWorker := range sbWorkers {
		// Store worker info in local map
		wm.workers[sbWorker.WorkerID] = &sbWorker

		// Establish gRPC connection to worker (async)
		go wm.connectToWorker(sbWorker.WorkerID, sbWorker.Address, sbWorker.GRPCPort)
	}

	// Start background goroutine for health checks
	go wm.startHealthCheckLoop()

	// background goroutine to sync local map with Supabase
	go wm.startSupabaseSyncLoop()

	log.Print("Worker manager started successfully")
	return nil
}

// connectToWorker establishes a gRPC connection to a worker
func (wm *workerManager) connectToWorker(workerID, address string, grpcPort int) {
	target := fmt.Sprintf("%s:%d", address, grpcPort)

	// Create gRPC connection with insecure credentials for now
	// TODO: Add TLS in production
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to connect to worker %s at %s: %v", workerID, target, err)

		// Update worker status to offline on connection failure
		wm.markWorkerOffline(workerID)
		return
	}

	// Store the connection
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if worker, exists := wm.workers[workerID]; exists {
		// Close any existing connection
		if worker.Conn != nil {
			worker.Conn.Close()
		}
		worker.Conn = conn
		worker.Status = StatusOnline
		log.Printf("Successfully connected to worker %s at %s", workerID, target)
	} else {
		// Worker was removed while we were connecting
		conn.Close()
	}
}

// startHealthCheckLoop runs periodic health checks on all workers
func (wm *workerManager) startHealthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second) // Health check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			log.Print("Health check loop stopped")
			return
		case <-ticker.C:
			wm.performHealthChecks()
		}
	}
}

// performHealthChecks checks the health of all workers
func (wm *workerManager) performHealthChecks() {
	wm.mu.RLock()
	workers := make([]*Worker, 0, len(wm.workers))
	for _, worker := range wm.workers {
		workers = append(workers, worker)
	}
	wm.mu.RUnlock()

	for _, worker := range workers {
		go wm.checkWorkerHealth(worker.WorkerID)
	}
}

// checkWorkerHealth performs a health check on a specific worker
func (wm *workerManager) checkWorkerHealth(workerID string) {
	wm.mu.RLock()
	worker, exists := wm.workers[workerID]
	if !exists {
		wm.mu.RUnlock()
		return
	}

	// For now, just check if the gRPC connection is still valid
	// TODO: Implement actual health check RPC call
	if worker.Conn == nil {
		wm.mu.RUnlock()
		wm.markWorkerOffline(workerID)
		return
	}

	wm.mu.RUnlock()

	// Check connection state
	state := worker.Conn.GetState()
	if state.String() != "READY" && state.String() != "IDLE" {
		log.Printf("Worker %s connection unhealthy: %s", workerID, state.String())
		wm.markWorkerOffline(workerID)

		// Try to reconnect
		go wm.connectToWorker(worker.WorkerID, worker.Address, worker.GRPCPort)
	}
}

// markWorkerOffline marks a worker as offline
func (wm *workerManager) markWorkerOffline(workerID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if worker, exists := wm.workers[workerID]; exists {
		worker.Status = StatusOffline
		if worker.Conn != nil {
			worker.Conn.Close()
			worker.Conn = nil
		}
		log.Printf("Marked worker %s as offline", workerID)
	}
}

// Stop stops the worker manager and closes all connections
func (wm *workerManager) Stop() error {
	log.Print("Stopping worker manager...")

	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Close all gRPC connections
	for workerID, worker := range wm.workers {
		if worker.Conn != nil {
			worker.Conn.Close()
			worker.Conn = nil
		}
		worker.Status = StatusOffline
		log.Printf("Disconnected from worker %s", workerID)
	}

	log.Print("Worker manager stopped successfully")
	return nil
}

// RegisterWorker registers a new worker with the manager
func (wm *workerManager) RegisterWorker(ctx context.Context, workerID, address string, capacity int) error {
	// TODO: Implement worker registration
	// This would insert a new worker into the Supabase database
	return fmt.Errorf("RegisterWorker not yet implemented")
}

// GetWorker retrieves a worker by ID
func (wm *workerManager) GetWorker(ctx context.Context, workerID string) (*Worker, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	worker, exists := wm.workers[workerID]
	if !exists {
		return nil, fmt.Errorf("worker %s not found", workerID)
	}

	return worker, nil
}

// ListWorkers lists all registered workers
func (wm *workerManager) ListWorkers(ctx context.Context) ([]*Worker, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workers := make([]*Worker, 0, len(wm.workers))
	for _, worker := range wm.workers {
		workers = append(workers, worker)
	}

	return workers, nil
}

// UpdateWorker updates the worker's status or capacity
func (wm *workerManager) UpdateWorker(ctx context.Context, workerID string, capacity int, status Status) error {
	// TODO: Implement worker update
	// This would update the worker in both local map and Supabase database
	return fmt.Errorf("UpdateWorker not yet implemented")
}

// RemoveWorker removes a worker from the manager
func (wm *workerManager) RemoveWorker(ctx context.Context, workerID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	worker, exists := wm.workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	// Close connection if exists
	if worker.Conn != nil {
		worker.Conn.Close()
	}

	// Remove from local map
	delete(wm.workers, workerID)

	log.Printf("Removed worker %s", workerID)
	return nil
}
