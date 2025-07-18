package orchestrator

import (
	"context"
	"time"

	"distro.lol/internal/orchestrator/shard"
	"distro.lol/internal/orchestrator/worker"
)

// Config holds orchestrator configuration
type Config struct {
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
	config        *Config
	workerManager worker.Manager
	shardManager  shard.Manager
	// storageManager *storageManager
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new orchestrator with the given configuration
func New(config *Config) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	return &Orchestrator{
		config:        config,
		workerManager: worker.NewManager(ctx, config.WorkerSyncInterval),
		shardManager:  shard.NewManager(ctx, config.MasterKey),
		// storageManager: newStorageManager(ctx, config.SecretThreshold, config.SecretShares),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (o *Orchestrator) GetConfig() Config {
    orig := o.config
    copied := *orig
    if orig.MasterKey != nil {
        copied.MasterKey = make([]byte, len(orig.MasterKey))
        copy(copied.MasterKey, orig.MasterKey)
    }
    return copied
}