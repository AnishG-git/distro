package logic

import (
	"context"
	"log"
	"time"

	"distro.lol/internal/orchestrator/shard"
	"distro.lol/internal/orchestrator/worker"
	"distro.lol/internal/orchestrator/storage"
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
	MinAvailableSpace  int64         // Minimum available space for workers
}

// Orchestrator coordinates all components
type Orchestrator struct {
	config         *OrchestratorConfig
	workerManager  worker.Manager
	shardManager   shard.Manager
	storageManager storage.Manager
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewOrchestrator creates a new orchestrator with the given configuration
func NewOrchestrator(config *OrchestratorConfig) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	// Create storage manager
	storageManager, err := storage.NewManager()
	if err != nil {
		log.Fatalf("Failed to create storage manager: %v", err)
	}

	// Create worker manager with storage manager
	workerManager := worker.NewManager(ctx, storageManager, config.WorkerSyncInterval, config.MinAvailableSpace)

	if err := workerManager.Start(); err != nil {
		log.Fatalf("Failed to start worker manager: %v", err)
	}

	return &Orchestrator{
		config:         config,
		workerManager:  workerManager,
		shardManager:   shard.NewManager(ctx, config.MasterKey),
		storageManager: storageManager,
		ctx:            ctx,
		cancel:         cancel,
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

// GetStorageManager returns the storage manager instance
func (o *Orchestrator) GetStorageManager() storage.Manager {
	return o.storageManager
}
