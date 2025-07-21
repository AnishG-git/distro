package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/supabase-community/supabase-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	pbw "distro.lol/pkg/rpc/worker"
)

type Manager interface {
	// RegisterWorker registers a new worker with the manager
	RegisterWorker(ctx context.Context, workerID, workerEndpoint string, capacity, usedSpace int64) error
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
	minAvailableSpace  int64
}

func NewManager(ctx context.Context, workerSyncInterval time.Duration, minAvailableSpace int) *workerManager {
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
	sbWorkers, err := wm.getOnlineWorkersFromSB()
	if err != nil {
		return fmt.Errorf("failed to get online workers from Supabase: %w", err)
	}

	log.Printf("Loaded %d worker(s) from database", len(sbWorkers))

	wg := sync.WaitGroup{}

	for _, sbWorker := range sbWorkers {
		wm.workers[sbWorker.WorkerID] = &sbWorker
	}

	for workerID, worker := range wm.workers {
		// Establish gRPC connection to worker (async)
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := wm.getOrCreateWorkerConn(workerID, worker.WorkerEndpoint)
			if err != nil {
				log.Printf("marking worker offline %s: %v", workerID, err)
				worker.Status = StatusOffline // Set status to offline if connection fails
				return
			}
			worker.Conn = conn
			worker.Status = StatusOnline      // Set initial status to online
			worker.LastHeartbeat = time.Now() // Set initial heartbeat
		}()
	}

	wg.Wait()

	// Start background goroutine for health checks
	go wm.startHealthCheckLoop()

	// background goroutine to sync local map with Supabase
	go wm.startSupabaseSyncLoop()

	log.Print("Worker manager started successfully")
	return nil
}

// getOrCreateWorkerConn establishes a gRPC connection to a worker
func (wm *workerManager) getOrCreateWorkerConn(workerID, target string) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn

	wm.mu.RLock()
	if worker, ok := wm.workers[workerID]; ok {
		conn = worker.Conn
	}
	wm.mu.RUnlock()

	if conn == nil {
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(
				keepalive.ClientParameters{
					Time:                10 * time.Second, // Ping every 10 seconds
					Timeout:             3 * time.Second,  // Wait 3 seconds for ping
					PermitWithoutStream: true,
				}),
		}

		newConn, err := grpc.NewClient(target, opts...)
		if err != nil {
			return nil, err
		}

		conn = newConn
	}

	// log.Printf("Worker %s is online with capacity %d and used space %d", target, capacity, usedSpace)
	return conn, nil
}


func isAlive(conn *grpc.ClientConn) bool {
	if conn == nil {
		return false
	}
	state := conn.GetState()
	if state == connectivity.Shutdown || state == connectivity.TransientFailure {
		return false
	}
	return true
}

// startHealthCheckLoop runs periodic health checks on all workers
func (wm *workerManager) startHealthCheckLoop() {
	ticker := time.NewTicker(10 * time.Second) // Health check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			log.Print("Health check loop stopped")
			return
		case <-ticker.C:
			log.Print("Performing health checks on workers...")
			wm.performHealthChecks()
		}
	}
}

// performHealthChecks checks the health of all workers
func (wm *workerManager) performHealthChecks() {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	for workerID, worker := range wm.workers {
		// healthy := worker.LastHeartbeat.Add(45 * time.Second).After(now)
		// newStatus := StatusOffline
		// if healthy {
		// 	newStatus = StatusOnline
		// }
		// if worker.Status != newStatus {
		// 	worker.Status = newStatus
		// 	log.Printf("Health Check: Worker %s status changed to %v", workerID, newStatus)
		// }
		if !isAlive(worker.Conn) {
			log.Printf("Worker %s is offline, changing status", workerID)
			worker.Status = StatusOffline
		}
		log.Printf("Worker %s is %s and has a conn with status %s", workerID, worker.Status, worker.Conn.GetState().String())
	}
}

// gets worker's stats from worker's connection
// also can be used to ping worker
func (wm *workerManager) retrieveWorkerStats(conn *grpc.ClientConn) (int64, int64, error) {
	if conn == nil {
		return 0, 0, fmt.Errorf("worker has no gRPC connection")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stats, err := pbw.NewWorkerClient(conn).Ping(ctx, &pbw.PingRequest{
		Ping: "ping",
	})

	if err != nil {
		return 0, 0, fmt.Errorf("ping failed for worker: %w", err)
	}

	return stats.TotalCapacity, stats.UsedCapacity, nil
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
func (wm *workerManager) RegisterWorker(ctx context.Context, workerID, workerEndpoint string, capacity, usedSpace int64) error {
	// TODO: Implement worker registration
	// This would insert a new worker into the Supabase database
	if capacity <= 0 {
		return fmt.Errorf("invalid capacity: %d", capacity)
	}

	if usedSpace < 0 || usedSpace > capacity {
		return fmt.Errorf("invalid used space: %d", usedSpace)
	}

	if capacity-usedSpace < wm.minAvailableSpace {
		return fmt.Errorf("worker has %d space < minimum required %d", capacity-usedSpace, wm.minAvailableSpace)
	}

	worker := &Worker{
		WorkerID:       workerID,
		WorkerEndpoint: workerEndpoint,
		TotalCapacity:  capacity,
		UsedCapacity:   usedSpace,
		Status:         StatusOnline,
		LastHeartbeat:  time.Now(),
	}


	conn, err := wm.getOrCreateWorkerConn(worker.WorkerID, worker.WorkerEndpoint)
	if err != nil {
		return err
	}

	// Store worker in database
	if err := wm.upsertWorkerToSB(*worker); err != nil {
		conn.Close() // Close connection if upsert fails
		return fmt.Errorf("failed to upsert worker to Supabase: %w", err)
	}

	worker.Conn = conn

	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.workers[workerID] = worker


	return nil
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
