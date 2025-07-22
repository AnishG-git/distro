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

	log.Printf("SQLite database initialized successfully at %s", dbPath)
	return nil
}
