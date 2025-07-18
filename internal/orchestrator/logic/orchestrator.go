package logic

import (
	"context"
	"log"
	"time"

	"distro.lol/internal/orchestrator/shard"
	"distro.lol/internal/orchestrator/worker"
)

// OrchestratorConfig holds orchestrator configuration
type OrchestratorConfig struct {
	HTTPPort           int           // REST API port for clients
	GRPCPort           int           // gRPC port for worker communication
	MasterKey          []byte        // Master encryption key
	EpochMinutes       int           // Key rotation interval
	DefaultShardN      int           // Default number of shards
	DefaultShardK      int           // Default threshold for reconstruction
	SecretThreshold    int           // Shamir's secret sharing threshold
	SecretShares       int           // Shamir's secret sharing total shares
	WorkerSyncInterval time.Duration // Interval for worker data sync with Supabase
}

// Orchestrator coordinates all components
type Orchestrator struct {
	config        *OrchestratorConfig
	workerManager worker.Manager
	shardManager  shard.Manager
	// storageManager *storageManager
	ctx    context.Context
	cancel context.CancelFunc
}

// NewOrchestrator creates a new orchestrator with the given configuration
func NewOrchestrator(config *OrchestratorConfig) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	workerManager := worker.NewManager(ctx, config.WorkerSyncInterval)

	if err := workerManager.Start(); err != nil {
		log.Fatalf("Failed to start worker manager: %v", err)
	}

	return &Orchestrator{
		config:        config,
		workerManager: workerManager,
		shardManager:  shard.NewManager(ctx, config.MasterKey),
		// storageManager: newStorageManager(ctx, config.SecretThreshold, config.SecretShares),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (o *Orchestrator) GetConfig() OrchestratorConfig {
	orig := o.config
	copied := *orig
	if orig.MasterKey != nil {
		copied.MasterKey = make([]byte, len(orig.MasterKey))
		copy(copied.MasterKey, orig.MasterKey)
	}
	return copied
}
