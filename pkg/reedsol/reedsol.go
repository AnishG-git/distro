package reedsol

import (
	"errors"
	"fmt"

	"github.com/klauspost/reedsolomon"
)

// Encoder wraps Reed-Solomon operations
type Encoder struct {
	enc reedsolomon.Encoder
	k   int // data shards
	n   int // total shards
}

// NewEncoder creates a new Reed-Solomon encoder
func NewEncoder(k, n int) (*Encoder, error) {
	if k <= 0 || n <= 0 {
		return nil, errors.New("k and n must be positive")
	}
	if k >= n {
		return nil, errors.New("k must be less than n")
	}
	if n-k > 255 {
		return nil, errors.New("too many parity shards (max 255)")
	}

	enc, err := reedsolomon.New(k, n-k)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	return &Encoder{
		enc: enc,
		k:   k,
		n:   n,
	}, nil
}

// Encode splits data into n shards with k recovery threshold
func (e *Encoder) Encode(data []byte) ([][]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("data cannot be empty")
	}

	// Split data into k equal parts
	shards, err := e.enc.Split(data)
	if err != nil {
		return nil, fmt.Errorf("failed to split data: %w", err)
	}

	// Generate parity shards
	if err := e.enc.Encode(shards); err != nil {
		return nil, fmt.Errorf("failed to encode shards: %w", err)
	}

	return shards, nil
}

// Reconstruct rebuilds data from k or more shards
func (e *Encoder) Reconstruct(shards [][]byte) ([]byte, error) {
	if len(shards) != e.n {
		return nil, fmt.Errorf("expected %d shards, got %d", e.n, len(shards))
	}

	// Count non-nil shards
	available := 0
	for _, shard := range shards {
		if shard != nil {
			available++
		}
	}

	if available < e.k {
		return nil, fmt.Errorf("insufficient shards: need at least %d, have %d", e.k, available)
	}

	// Reconstruct missing shards
	if err := e.enc.Reconstruct(shards); err != nil {
		return nil, fmt.Errorf("failed to reconstruct: %w", err)
	}

	// Join the data shards (first k shards)
	dataShards := shards[:e.k]

	// Calculate the total size of the original data
	totalSize := 0
	for _, shard := range dataShards {
		if shard != nil {
			totalSize += len(shard)
		}
	}

	// Reconstruct the original data by concatenating data shards
	data := make([]byte, 0, totalSize)
	for _, shard := range dataShards {
		if shard != nil {
			data = append(data, shard...)
		}
	}

	return data, nil
}

// Verify checks if shards are valid
func (e *Encoder) Verify(shards [][]byte) (bool, error) {
	if len(shards) != e.n {
		return false, fmt.Errorf("expected %d shards, got %d", e.n, len(shards))
	}

	return e.enc.Verify(shards)
}

// GetDataShardCount returns the number of data shards
func (e *Encoder) GetDataShardCount() int {
	return e.k
}

// GetParityShardCount returns the number of parity shards
func (e *Encoder) GetParityShardCount() int {
	return e.n - e.k
}

// GetTotalShardCount returns the total number of shards
func (e *Encoder) GetTotalShardCount() int {
	return e.n
}

// ShardInfo contains metadata about a shard
type ShardInfo struct {
	Index    int
	IsData   bool
	IsParity bool
	Size     int
}

// GetShardInfo returns information about each shard
func (e *Encoder) GetShardInfo(shards [][]byte) []ShardInfo {
	info := make([]ShardInfo, len(shards))
	for i, shard := range shards {
		info[i] = ShardInfo{
			Index:    i,
			IsData:   i < e.k,
			IsParity: i >= e.k,
			Size:     len(shard),
		}
	}
	return info
}

// EstimateShardSize estimates the size of each shard for given data
func (e *Encoder) EstimateShardSize(dataSize int) int {
	// Reed-Solomon splits data evenly across k data shards
	// Each shard will be approximately dataSize/k bytes
	return (dataSize + e.k - 1) / e.k // Round up division
}
