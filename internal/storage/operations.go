package storage

import (
	"context"
	"encoding/json"
	"fmt"
)

// Worker operations
func (m *manager) UpsertWorker(ctx context.Context, worker WorkerRecord) error {
	_, _, err := m.client.From("workers").Upsert(worker, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to upsert worker %s: %w", worker.WorkerID, err)
	}
	return nil
}

func (m *manager) GetOnlineWorkers(ctx context.Context) ([]WorkerRecord, error) {
	var workers []WorkerRecord
	data, _, err := m.client.From("workers").
		Select("*", "", false).
		Eq("status", "online").
		Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workers: %w", err)
	}

	if err := json.Unmarshal(data, &workers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal worker data: %w", err)
	}

	return workers, nil
}

func (m *manager) UpdateWorkerStatus(ctx context.Context, workerID string, status string) error {
	update := map[string]any{
		"status":         status,
		"updated_at":     "now()",
		"last_heartbeat": "now()",
	}

	_, _, err := m.client.From("workers").
		Update(update, "", "").
		Eq("worker_id", workerID).
		Execute()
	if err != nil {
		return fmt.Errorf("failed to update worker %s status: %w", workerID, err)
	}
	return nil
}

// Object operations
func (m *manager) StoreObjectMetadata(ctx context.Context, object ObjectRecord) error {
	_, _, err := m.client.From("objects").Insert(object, false, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to store object metadata for %s: %w", object.ObjectID, err)
	}
	return nil
}

func (m *manager) GetObjectMetadata(ctx context.Context, objectID string) (*ObjectRecord, error) {
	var objects []ObjectRecord
	data, _, err := m.client.From("objects").
		Select("*", "", false).
		Eq("object_id", objectID).
		Single().
		Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata for %s: %w", objectID, err)
	}

	if err := json.Unmarshal(data, &objects); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object metadata: %w", err)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("object metadata not found for object %s", objectID)
	}

	return &objects[0], nil
}

// Shard operations
func (m *manager) StoreShard(ctx context.Context, shard ShardRecord) error {
	_, _, err := m.client.From("shards").Insert(shard, false, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to store shard %s: %w", shard.ShardID, err)
	}
	return nil
}

func (m *manager) GetShardsForObject(ctx context.Context, objectID string) ([]ShardRecord, error) {
	var shards []ShardRecord
	data, _, err := m.client.From("shards").
		Select("*", "", false).
		Eq("object_id", objectID).
		Order("shard_id", nil).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get shards for object %s: %w", objectID, err)
	}

	if err := json.Unmarshal(data, &shards); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shard data: %w", err)
	}

	return shards, nil
}
