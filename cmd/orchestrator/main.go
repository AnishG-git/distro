package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"distro.lol/internal/orchestrator"
	"distro.lol/internal/orchestrator/logic"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Error loading .env file, using environment variables: %v", err)
	}

	httpPortStr := os.Getenv("HTTP_PORT")
	if httpPortStr == "" {
		httpPortStr = "8080" // Default HTTP port
	}

	grpcPortStr := os.Getenv("GRPC_PORT")
	if grpcPortStr == "" {
		grpcPortStr = "9090" // Default gRPC port
	}

	httpPort, err := strconv.Atoi(httpPortStr)
	if err != nil {
		log.Fatalf("Invalid HTTP_PORT environment variable, must be an integer: %v", err)
	}

	grpcPort, err := strconv.Atoi(grpcPortStr)
	if err != nil {
		log.Fatalf("Invalid GRPC_PORT environment variable, must be an integer: %v", err)
	}

	masterKey := []byte(os.Getenv("MASTER_KEY"))
	if len(masterKey) == 0 {
		log.Fatal("MASTER_KEY environment variable is required")
	}

	// Initialize the orchestrator service with the configuration
	config := &logic.OrchestratorConfig{
		HTTPPort:           httpPort,
		GRPCPort:           grpcPort,
		MasterKey:          masterKey,
		EpochMinutes:       60,
		DefaultShardN:      5,
		DefaultShardK:      3,
		SecretThreshold:    2,
		SecretShares:       5,
		WorkerSyncInterval: 1 * time.Minute,
		MinAvailableSpace:  20, // Minimum available space for workers
	}

	service := orchestrator.NewService(config)

	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the orchestrator service
	if err := service.Start(ctx); err != nil {
		log.Fatalf("Failed to start orchestrator service: %v", err)
	}
}
