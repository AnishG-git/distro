package logic

import (
	"context"
	"fmt"
	"log"
	"time"

	"distro.lol/internal/orchestrator/worker"
	"distro.lol/internal/storage"
	pbw "distro.lol/pkg/rpc/worker"
)

func (o *Orchestrator) RegisterWorker(ctx context.Context, workerID, workerEndpoint string, capacity, usedSpace int64) error {
	// Implement the logic to register a worker
	// This could involve checking the worker's capacity and used space
	// and storing it in a database or in-memory structure

	return o.workerManager.RegisterWorker(ctx, workerID, workerEndpoint, capacity, usedSpace)
}

func (o *Orchestrator) DistributeFile(ctx context.Context, filebytes []byte, filename string, filesize int64) (string, error) {
	// Generate unique object ID for tracking
	objectID := fmt.Sprintf("%s-%d", filename, time.Now().UnixNano())

	// Use configured shard parameters
	n := o.config.DefaultShardN
	k := o.config.DefaultShardK

	log.Printf("Starting file distribution for %s (%d bytes) with %d/%d sharding", filename, filesize, k, n)

	// Create encrypted shards
	shards, epoch, err := o.shardManager.CreateEncryptedShards(filebytes, n, k)
	if err != nil {
		return "", fmt.Errorf("failed to create encrypted shards: %w", err)
	}

	log.Printf("Created %d encrypted shards for file %s with epoch %s", len(shards), filename, epoch)

	// Get available workers for shard distribution
	workers, err := o.workerManager.ListWorkers(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list workers: %w", err)
	}

	// Filter online workers with sufficient capacity
	availableWorkers := make([]*worker.Worker, 0)
	for _, w := range workers {
		if w.Status == worker.StatusOnline && w.TotalCapacity-w.UsedCapacity > int64(len(shards[0])) {
			availableWorkers = append(availableWorkers, w)
		}
	}

	if len(availableWorkers) < n {
		return "", fmt.Errorf("insufficient workers available: need %d, have %d", n, len(availableWorkers))
	}

	log.Printf("Found %d available workers for shard distribution", len(availableWorkers))

	// Distribute shards concurrently to workers
	type shardResult struct {
		shardID  int
		workerID string
		success  bool
		err      error
	}

	results := make(chan shardResult, n)

	for i, shard := range shards {
		go func(shardIndex int, shardData []byte, w *worker.Worker) {
			// Create worker client
			client := pbw.NewWorkerClient(w.Conn)

			// Create shard envelope with metadata
			envelope := &pbw.ShardEnvelope{
				ShardId: fmt.Sprintf("%s-shard-%d", objectID, shardIndex),
				Shard:   shardData,
			}

			// Store shard on worker
			_, err := client.StoreShard(ctx, envelope)
			if err != nil {
				results <- shardResult{shardIndex, w.WorkerID, false, err}
				return
			}

			results <- shardResult{shardIndex, w.WorkerID, true, nil}
		}(i, shard, availableWorkers[i])
	}

	// Collect results
	successCount := 0
	shardPlacements := make(map[int]string) // shard index -> worker ID

	for range n {
		result := <-results
		if result.success {
			successCount++
			shardPlacements[result.shardID] = result.workerID
			log.Printf("Successfully stored shard %d on worker %s", result.shardID, result.workerID)
		} else {
			log.Printf("Failed to store shard %d on worker %s: %v", result.shardID, result.workerID, result.err)
		}
	}

	// Check if we have enough successful placements for reconstruction
	if successCount < k {
		return "", fmt.Errorf("insufficient shard placements: need %d, got %d", k, successCount)
	}

	// Store object metadata in database
	objectRecord := storage.ObjectRecord{
		ObjectID:  objectID,
		Filename:  filename,
		FileSize:  filesize,
		Epoch:     epoch,
		ShardN:    n,
		ShardK:    k,
		Status:    "completed",
	}

	if err := o.storageManager.StoreObjectMetadata(ctx, objectRecord); err != nil {
		log.Printf("Warning: failed to store object metadata: %v", err)
	}

	// Store shard metadata in database
	for shardIndex, workerID := range shardPlacements {
		shardRecord := storage.ShardRecord{
			ShardID:   fmt.Sprintf("%s-shard-%d", objectID, shardIndex),
			ObjectID:  objectID,
			ShardSize: int64(len(shards[shardIndex])),
			WorkerID:  workerID,
			Status:    "stored",
		}

		if err := o.storageManager.StoreShard(ctx, shardRecord); err != nil {
			log.Printf("Warning: failed to store shard metadata for shard %d: %v", shardIndex, err)
		}
	}

	log.Printf("Successfully distributed file %s as object %s (%d/%d shards placed)", filename, objectID, successCount, n)

	return objectID, nil
}
