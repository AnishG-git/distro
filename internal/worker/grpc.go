package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	pb "distro.lol/pkg/rpc/worker"
)

func (w *worker) Ping(ctx context.Context, req *pb.PingRequest) (*pb.StorageStats, error) {
	// Implement the logic to handle ping requests
	// This could involve checking the worker's status and returning its stats
	return &pb.StorageStats{
		TotalCapacity: w.capacity,
		UsedCapacity:  w.usedSpace,
	}, nil
}

func (w *worker) StoreShard(ctx context.Context, envelope *pb.ShardEnvelope) (*pb.StorageStats, error) {
	if envelope == nil || envelope.ShardId == "" || len(envelope.Shard) == 0 {
		return nil, fmt.Errorf("invalid shard envelope: missing shard ID or data")
	}

	// Check if we have enough space
	shardSize := int64(len(envelope.Shard))
	availableSpace := w.capacity - w.usedSpace
	usagePercent := float64(w.usedSpace) / float64(w.capacity) * 100

	log.Printf("Storage check for shard %s: size=%d bytes, used=%d/%d bytes (%.1f%%), available=%d bytes",
		envelope.ShardId, shardSize, w.usedSpace, w.capacity, usagePercent, availableSpace)

	if w.usedSpace+shardSize > w.capacity {
		log.Printf("Storage capacity exceeded: shard %s requires %d bytes but only %d bytes available (%.1f%% full)",
			envelope.ShardId, shardSize, availableSpace, usagePercent)
		return nil, fmt.Errorf("insufficient storage capacity: need %d bytes, available %d bytes",
			shardSize, w.capacity-w.usedSpace)
	}

	// Store shard in SQLite database
	insertSQL := `
	INSERT OR REPLACE INTO shards (shard_id, shard_data, updated_at) 
	VALUES (?, ?, CURRENT_TIMESTAMP)
	`

	result, err := w.db.ExecContext(ctx, insertSQL, envelope.ShardId, envelope.Shard)
	if err != nil {
		return nil, fmt.Errorf("failed to store shard %s: %w", envelope.ShardId, err)
	}

	// Check if this was an insert (new shard) or update (existing shard)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Warning: could not determine rows affected for shard %s: %v", envelope.ShardId, err)
	}

	oldUsedSpace := w.usedSpace

	// Only update used space if this is a new shard
	if rowsAffected == 1 {
		// Check if shard already existed to determine if we should update used space
		var existingSize int64
		checkSQL := `SELECT LENGTH(shard_data) FROM shards WHERE shard_id = ? AND updated_at < CURRENT_TIMESTAMP`
		err := w.db.QueryRowContext(ctx, checkSQL, envelope.ShardId).Scan(&existingSize)

		switch err {
		case sql.ErrNoRows:
			// New shard, update used space
			w.usedSpace += shardSize
			log.Printf("New shard stored: %s, capacity changed from %d to %d bytes (+%d bytes)",
				envelope.ShardId, oldUsedSpace, w.usedSpace, shardSize)
		case nil:
			// Existing shard, update used space difference
			sizeDiff := shardSize - existingSize
			w.usedSpace = w.usedSpace - existingSize + shardSize
			log.Printf("Existing shard updated: %s, capacity changed from %d to %d bytes (%+d bytes)",
				envelope.ShardId, oldUsedSpace, w.usedSpace, sizeDiff)
		}
		// If other error, log but continue
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Warning: could not check existing shard size for %s: %v", envelope.ShardId, err)
		}
	}

	newUsagePercent := float64(w.usedSpace) / float64(w.capacity) * 100
	newAvailableSpace := w.capacity - w.usedSpace

	log.Printf("Successfully stored shard %s (%d bytes), storage now at %.1f%% capacity (%d/%d bytes, %d bytes available)",
		envelope.ShardId, shardSize, newUsagePercent, w.usedSpace, w.capacity, newAvailableSpace)

	// Log warning if storage is getting full
	if newUsagePercent >= 90 {
		log.Printf("WARNING: Storage capacity critical - %.1f%% full (%d bytes remaining)", newUsagePercent, newAvailableSpace)
	} else if newUsagePercent >= 80 {
		log.Printf("WARNING: Storage capacity high - %.1f%% full (%d bytes remaining)", newUsagePercent, newAvailableSpace)
	}

	return &pb.StorageStats{
		TotalCapacity: w.capacity,
		UsedCapacity:  w.usedSpace,
	}, nil
}

func (w *worker) FetchShard(ctx context.Context, req *pb.ShardRequest) (*pb.ShardEnvelope, error) {
	if req == nil || req.ShardId == "" {
		return nil, fmt.Errorf("invalid shard request: missing shard ID")
	}

	// Fetch shard from SQLite database
	selectSQL := `SELECT shard_data FROM shards WHERE shard_id = ?`

	var shardData []byte
	err := w.db.QueryRowContext(ctx, selectSQL, req.ShardId).Scan(&shardData)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("shard not found: %s", req.ShardId)
		}
		return nil, fmt.Errorf("failed to fetch shard %s: %w", req.ShardId, err)
	}

	log.Printf("Successfully fetched shard %s (%d bytes)", req.ShardId, len(shardData))

	return &pb.ShardEnvelope{
		ShardId: req.ShardId,
		Shard:   shardData,
	}, nil
}
