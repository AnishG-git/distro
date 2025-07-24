package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"distro.lol/internal/orchestrator/storage"
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
	storageManager     storage.Manager
	mu                 *sync.RWMutex
	workers            map[string]*Worker // workerID -> Info
	workerSyncInterval time.Duration
	minAvailableSpace  int64
}

func NewManager(ctx context.Context, storageManager storage.Manager, workerSyncInterval time.Duration, minAvailableSpace int64) *workerManager {
	return &workerManager{
		ctx:                ctx,
		storageManager:     storageManager,
		mu:                 &sync.RWMutex{},
		workers:            make(map[string]*Worker),
		workerSyncInterval: workerSyncInterval,
		minAvailableSpace:  minAvailableSpace,
	}
}

func (wm *workerManager) Start() error {
	log.Print("Starting worker manager...")

	// Get workers from storage manager
	workerRecords, err := wm.storageManager.GetOnlineWorkers(wm.ctx)
	if err != nil {
		return fmt.Errorf("failed to get online workers from storage: %w", err)
	}

	log.Printf("Loaded %d worker(s) from database", len(workerRecords))

	wg := sync.WaitGroup{}

	// Convert storage records to internal worker objects
	for _, record := range workerRecords {
		worker := &Worker{
			WorkerID:       record.WorkerID,
			WorkerEndpoint: record.WorkerEndpoint,
			TotalCapacity:  record.TotalCapacity,
			UsedCapacity:   record.UsedCapacity,
			Status:         StatusFromString(record.Status),
			LastHeartbeat:  record.LastHeartbeat,
		}
		wm.workers[worker.WorkerID] = worker
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

	// background goroutine to sync local map with storage
	go wm.startStorageSyncLoop()

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
			grpc.WithDefaultCallOptions(
                grpc.MaxCallRecvMsgSize(128*1024*1024), // 128MB max receive message size
                grpc.MaxCallSendMsgSize(128*1024*1024), // 128MB max send message size
            ),
		}

		newConn, err := grpc.NewClient(target, opts...)
		if err != nil {
			return nil, err
		}

		conn = newConn
	}

	totalCapacity, usedCapacity, err := wm.retrieveWorkerStats(conn)
	if err != nil {
		wm.mu.Lock()
		defer wm.mu.Unlock()
		if worker, ok := wm.workers[workerID]; ok {
			conn.Close() // Close connection if ping fails
			worker.Conn = nil // Clear connection if ping fails
			worker.Status = StatusOffline // Set status to offline
		}
		log.Printf("failed to ping worker %s at %s: %v", workerID, target, err)
		return nil, err
	}

	wm.mu.Lock()
	if worker, ok := wm.workers[workerID]; ok {
		worker.Conn = conn
		worker.TotalCapacity = totalCapacity
		worker.UsedCapacity = usedCapacity
		worker.Status = StatusOnline
		worker.LastHeartbeat = time.Now()
	}
	wm.mu.Unlock()

	log.Printf("Worker %s is online with capacity %d and used space %d", workerID, totalCapacity, usedCapacity)
	return conn, nil
}

// retrieveWorkerStats pings the worker with a deadline of 3 seconds to check its status and retrieve storage stats
// also retrieves total and used capacity
func (wm *workerManager) retrieveWorkerStats(conn *grpc.ClientConn) (int64, int64, error) {
	if conn == nil {
		return 0, 0, fmt.Errorf("worker connection is nil")
	}

	ctx, cancel := context.WithTimeout(wm.ctx, 3*time.Second)
	defer cancel()

	workerCli := pbw.NewWorkerClient(conn)
	storageStats, err := workerCli.Ping(ctx, &pbw.PingRequest{Ping: "ping"})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to ping worker: %w", err)
	}

	return storageStats.TotalCapacity, storageStats.UsedCapacity, nil
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
			log.Print("Performing health checks on workers...")
			wm.performHealthChecks()
		}
	}
}

// performHealthChecks checks the health of all workers
func (wm *workerManager) performHealthChecks() {
	// Create a temporary slice to hold worker information needed for health checks
	type workerInfo struct {
		id     string
		conn   *grpc.ClientConn
		worker *Worker
	}

	var workersToCheck []workerInfo

	wm.mu.RLock()
	for workerID, worker := range wm.workers {
		workersToCheck = append(workersToCheck, workerInfo{
			id:     workerID,
			conn:   worker.Conn,
			worker: worker,
		})
	}
	wm.mu.RUnlock()

	// Now perform health checks without holding the lock
	for _, info := range workersToCheck {
		if capacity, usedSpace, err := wm.retrieveWorkerStats(info.conn); err != nil {
			log.Printf("Worker %s is offline: %v", info.id, err)
			wm.mu.Lock()
			if info.worker.Status != StatusOffline && info.worker.Conn != nil {
				info.worker.Status = StatusOffline
				info.worker.Conn.Close() // Close connection if offline
				info.worker.Conn = nil   // Clear connection
			}
			wm.mu.Unlock()
		} else {
			wm.mu.Lock()
			info.worker.Status = StatusOnline
			info.worker.TotalCapacity = capacity
			info.worker.UsedCapacity = usedSpace
			info.worker.LastHeartbeat = time.Now()
			wm.mu.Unlock()
			log.Printf("Worker %s is online with capacity %d and used space %d", info.id, capacity, usedSpace)
		}
		// log.Printf("Worker %s is %s and has a conn with status %s", info.id, info.worker.Status, info.conn.GetState().String())
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

// startStorageSyncLoop syncs worker data with storage periodically
func (wm *workerManager) startStorageSyncLoop() {
	ticker := time.NewTicker(wm.workerSyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-wm.ctx.Done():
			return // Exit if context is done
		case <-ticker.C:
			if err := wm.syncWorkersWithStorage(); err != nil {
				log.Printf("failed to sync local workers map with storage: %v", err)
			}
		}
	}
}

// syncWorkersWithStorage syncs the local worker map with storage
func (wm *workerManager) syncWorkersWithStorage() error {
	wm.mu.Lock()

	// Convert workers map to storage records
	var records []storage.WorkerRecord
	for _, worker := range wm.workers {
		record := storage.WorkerRecord{
			WorkerID:       worker.WorkerID,
			WorkerEndpoint: worker.WorkerEndpoint,
			TotalCapacity:  worker.TotalCapacity,
			UsedCapacity:   worker.UsedCapacity,
			Status:         string(worker.Status),
			LastHeartbeat:  worker.LastHeartbeat,
		}
		records = append(records, record)
	}

	wm.mu.Unlock()

	// Upsert each worker record
	for _, record := range records {
		if err := wm.storageManager.UpsertWorker(wm.ctx, record); err != nil {
			return fmt.Errorf("failed to upsert worker %s: %w", record.WorkerID, err)
		}
	}

	log.Println("Successfully synced workers with storage")
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

	// Store worker in database using storage manager
	record := storage.WorkerRecord{
		WorkerID:       worker.WorkerID,
		WorkerEndpoint: worker.WorkerEndpoint,
		TotalCapacity:  worker.TotalCapacity,
		UsedCapacity:   worker.UsedCapacity,
		Status:         string(worker.Status),
		LastHeartbeat:  worker.LastHeartbeat,
	}

	if err := wm.storageManager.UpsertWorker(ctx, record); err != nil {
		conn.Close() // Close connection if upsert fails
		return fmt.Errorf("failed to upsert worker to storage: %w", err)
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
