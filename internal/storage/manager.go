package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/supabase-community/supabase-go"
)

type Manager interface {
	// Worker operations
	UpsertWorker(ctx context.Context, worker WorkerRecord) error
	GetOnlineWorkers(ctx context.Context) ([]WorkerRecord, error)
	UpdateWorkerStatus(ctx context.Context, workerID string, status string) error

	// Object operations
	StoreObjectMetadata(ctx context.Context, object ObjectRecord) error
	GetObjectMetadata(ctx context.Context, objectID string) (*ObjectRecord, error)

	// Shard operations
	StoreShard(ctx context.Context, shard ShardRecord) error
	GetShardsForObject(ctx context.Context, objectID string) ([]ShardRecord, error)

	// Health check
	HealthCheck(ctx context.Context) error
}

type manager struct {
	client *supabase.Client
}

// Database record types
type WorkerRecord struct {
	WorkerID       string    `json:"worker_id" db:"worker_id"`
	WorkerEndpoint string    `json:"worker_endpoint" db:"worker_endpoint"`
	TotalCapacity  int64     `json:"total_capacity" db:"total_capacity"`
	UsedCapacity   int64     `json:"used_capacity" db:"used_capacity"`
	Status         string    `json:"status" db:"status"`
	LastHeartbeat  time.Time `json:"last_heartbeat" db:"last_heartbeat"`
}

type ObjectRecord struct {
	ObjectID  string    `json:"object_id" db:"object_id"`
	Filename  string    `json:"filename" db:"filename"`
	FileSize  int64     `json:"file_size" db:"file_size"`
	Epoch     string    `json:"epoch" db:"epoch"`
	ShardN    int       `json:"shard_n" db:"shard_n"`
	ShardK    int       `json:"shard_k" db:"shard_k"`
	Status    string    `json:"status" db:"status"` // uploading, completed, failed
}

type ShardRecord struct {
	ShardID   string    `json:"shard_id" db:"shard_id"`
	ObjectID  string    `json:"object_id" db:"object_id"`
	WorkerID  string    `json:"worker_id" db:"worker_id"`
	ShardSize int64     `json:"shard_size" db:"shard_size"`
	Status    string    `json:"status" db:"status"` // stored, lost, corrupted
}

func NewManager() (Manager, error) {
	client, err := newSupabaseClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Supabase client: %w", err)
	}

	return &manager{
		client: client,
	}, nil
}

func newSupabaseClient() (*supabase.Client, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_KEY")

	if url == "" || key == "" {
		return nil, fmt.Errorf("supabase URL or key not set in environment variables")
	}

	client, err := supabase.NewClient(url, key, &supabase.ClientOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Supabase client: %w", err)
	}

	return client, nil
}

func (m *manager) HealthCheck(ctx context.Context) error {
	// Simple health check by querying workers table
	_, _, err := m.client.From("workers").Select("worker_id", "", false).Limit(1, "").Execute()
	if err != nil {
		return fmt.Errorf("storage health check failed: %w", err)
	}
	return nil
}
