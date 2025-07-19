package main

import (
	"context"
	"log"
	"time"

	"distro.lol/internal/orchestrator"
	"distro.lol/internal/orchestrator/logic"
)

func main() {
	// Initialize the orchestrator service with the configuration
	config := &logic.OrchestratorConfig{
		HTTPPort:           8080,
		GRPCPort:           9090,
		MasterKey:          []byte("supersecretkey"),
		EpochMinutes:       60,
		DefaultShardN:      5,
		DefaultShardK:      3,
		SecretThreshold:    2,
		SecretShares:       5,
		WorkerSyncInterval: 2 * time.Minute,
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
