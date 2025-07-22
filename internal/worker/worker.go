package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	pbo "distro.lol/pkg/rpc/orchestrator"
	pb "distro.lol/pkg/rpc/worker"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type worker struct {
	id                   string
	workerEndpoint       string
	orchestratorEndpoint string  // Endpoint of the orchestrator
	capacity             int64   // Total capacity of the worker
	usedSpace            int64   // Used space in the worker
	db                   *sql.DB // SQLite database for shard storage
	pb.UnimplementedWorkerServer
}

func New(workerID uuid.UUID, workerEndpoint string, capacity int64) *worker {
	// TEST - for now we will use an environment variable for the worker ID
	// In the real service, the worker ID would be the worker's email address after they sign up or some byproduct
	return &worker{
		id:                   workerID.String(),
		workerEndpoint:       workerEndpoint,
		capacity:             capacity,
		orchestratorEndpoint: "127.0.0.1:9090",
	}
}

func (w *worker) Start() error {
	if w.capacity <= w.usedSpace {
		return fmt.Errorf("worker capacity exceeded: total capacity %d, used space %d", w.capacity, w.usedSpace)
	}

	// Initialize SQLite database before starting
	if err := w.initDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer w.db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ping orchestrator to check if it's ready
	conn, err := grpc.NewClient(w.orchestratorEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to orchestrator: %v", err)
	}

	now := time.Now()

	cli := pbo.NewOrchestratorClient(conn)
	resp, err := cli.Ping(ctx, &pbo.PingRequest{
		Message: "ping",
	})

	if err != nil {
		return fmt.Errorf("failed to ping orchestrator: %v", err)
	}

	if resp.Success {
		log.Printf("Orchestrator is ready at %s", w.orchestratorEndpoint)
	}
	timeElapsed := time.Since(now)
	log.Printf("Ping to orchestrator took %s", timeElapsed)
	// end of ping orchestrator

	go func() {
		time.Sleep(1 * time.Second) // Simulate some startup delay
		for {
			if err := w.checkReadiness(); err != nil {
				log.Printf("Server is not ready: %v", err)
				continue
			}
			break
		}
		// Wait for the server to be ready
		log.Printf("Server is ready at %s", w.workerEndpoint)

		// Register the worker with the orchestrator
		if err := w.register(ctx); err != nil {
			log.Printf("Failed to register worker: %v", err)
			cancel()
		}
	}()

	if err := w.startServer(ctx); err != nil {
		return err
	}

	return nil
}

func (w *worker) startServer(ctx context.Context) error {
	listener, err := net.Listen("tcp", w.workerEndpoint)
	if err != nil {
		log.Printf("Failed to listen at %s: %v", w.workerEndpoint, err)
		return err
	}

	server := grpc.NewServer()

	// Register orchestrator grpc service
	pb.RegisterWorkerServer(server, w)

	errChan := make(chan error, 1)

	go func() {
		log.Printf("Starting gRPC server at %s", w.workerEndpoint)
		errChan <- fmt.Errorf("gRPC server failed: %v", server.Serve(listener))
	}()

	// Wait for context cancellation and gracefully stop the server
	go func() {
		<-ctx.Done()
		log.Printf("Context cancelled, stopping gRPC server")
		server.GracefulStop()
	}()

	return <-errChan
}

func (w *worker) checkReadiness() error {
	conn, err := grpc.NewClient(w.workerEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to worker server: %v", err)
	}
	defer conn.Close()

	client := pb.NewWorkerClient(conn)
	_, err = client.Ping(context.Background(), &pb.PingRequest{
		Ping: "ping",
	})
	if err != nil {
		return fmt.Errorf("ping failed: %v", err)
	}

	return nil
}

func (w *worker) register(ctx context.Context) error {
	conn, err := grpc.NewClient(w.orchestratorEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to orchestrator: %v", err)
	}
	defer conn.Close()

	client := pbo.NewOrchestratorClient(conn)

	// Register the worker with the orchestrator
	resp, err := client.Register(ctx, &pbo.RegisterRequest{
		WorkerId:       w.id,
		WorkerEndpoint: w.workerEndpoint,
		TotalCapacity:  w.capacity,
		UsedSpace:      w.usedSpace,
	})

	if err != nil {
		return err
	}

	if resp.Success {
		return nil
	}

	return fmt.Errorf("worker registration failed")
}

// initDatabase initializes the SQLite database for shard storage
func (w *worker) initDatabase() error {
	// Get the directory of the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	dbDir := filepath.Dir(execPath)
	dbPath := filepath.Join(dbDir, fmt.Sprintf("%s.sqlite", w.id))

	// Check if database already exists
	if _, err := os.Stat(dbPath); err == nil {
		log.Printf("Using existing SQLite database at %s", dbPath)
	} else {
		log.Printf("Creating new SQLite database at %s", dbPath)
	}

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	w.db = db

	// Create shards table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS shards (
		shard_id TEXT PRIMARY KEY,
		shard_data BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TRIGGER IF NOT EXISTS update_shards_updated_at 
	AFTER UPDATE ON shards
	FOR EACH ROW
	BEGIN
		UPDATE shards SET updated_at = CURRENT_TIMESTAMP WHERE shard_id = NEW.shard_id;
	END;
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create shards table: %w", err)
	}

	// Calculate used space from existing shards
	if err := w.calculateUsedSpace(); err != nil {
		log.Printf("Warning: failed to calculate used space: %v", err)
		// Continue without failing - we'll use the default usedSpace value
	}

	log.Printf("SQLite database initialized successfully at %s", dbPath)
	return nil
}

// calculateUsedSpace calculates the total used space from all shards in the database
func (w *worker) calculateUsedSpace() error {
	// Query to get the total size of all shards
	query := `SELECT COALESCE(SUM(LENGTH(shard_data)), 0) as total_size, COUNT(*) as shard_count FROM shards`

	var totalSize int64
	var shardCount int

	err := w.db.QueryRow(query).Scan(&totalSize, &shardCount)
	if err != nil {
		return fmt.Errorf("failed to calculate used space: %w", err)
	}

	// Update the worker's used space
	oldUsedSpace := w.usedSpace
	w.usedSpace = totalSize

	usagePercent := float64(w.usedSpace) / float64(w.capacity) * 100
	availableSpace := w.capacity - w.usedSpace

	log.Printf("Calculated used space from %d existing shards: %d bytes (%.1f%% of %d bytes capacity, %d bytes available)",
		shardCount, w.usedSpace, usagePercent, w.capacity, availableSpace)

	if oldUsedSpace != w.usedSpace {
		log.Printf("Updated used space from %d to %d bytes (difference: %+d bytes)",
			oldUsedSpace, w.usedSpace, w.usedSpace-oldUsedSpace)
	}

	// Log warnings if storage is getting full
	if usagePercent >= 90 {
		log.Printf("WARNING: Storage capacity critical at startup - %.1f%% full (%d bytes remaining)",
			usagePercent, availableSpace)
	} else if usagePercent >= 80 {
		log.Printf("WARNING: Storage capacity high at startup - %.1f%% full (%d bytes remaining)",
			usagePercent, availableSpace)
	}

	return nil
}

// getShardStatistics returns detailed statistics about stored shards
// func (w *worker) getShardStatistics() (map[string]interface{}, error) {
// 	stats := make(map[string]interface{})

// 	// Get total count and size
// 	var totalSize int64
// 	var shardCount int
// 	err := w.db.QueryRow(`SELECT COALESCE(SUM(LENGTH(shard_data)), 0), COUNT(*) FROM shards`).Scan(&totalSize, &shardCount)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get basic statistics: %w", err)
// 	}

// 	// Get size distribution
// 	var minSize, maxSize, avgSize sql.NullInt64
// 	err = w.db.QueryRow(`
// 		SELECT 
// 			MIN(LENGTH(shard_data)), 
// 			MAX(LENGTH(shard_data)), 
// 			AVG(LENGTH(shard_data)) 
// 		FROM shards
// 	`).Scan(&minSize, &maxSize, &avgSize)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get size statistics: %w", err)
// 	}

// 	stats["shard_count"] = shardCount
// 	stats["total_size_bytes"] = totalSize
// 	stats["used_capacity_percent"] = float64(totalSize) / float64(w.capacity) * 100
// 	stats["available_space_bytes"] = w.capacity - totalSize

// 	if shardCount > 0 {
// 		stats["min_shard_size_bytes"] = minSize.Int64
// 		stats["max_shard_size_bytes"] = maxSize.Int64
// 		stats["avg_shard_size_bytes"] = avgSize.Int64
// 	}

// 	return stats, nil
// }
