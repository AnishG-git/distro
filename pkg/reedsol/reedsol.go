package reedsol

import (
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/reedsolomon"
)

// StreamingEncoder wraps Reed-Solomon operations for streaming
type StreamingEncoder struct {
	enc       reedsolomon.Encoder
	k         int // data shards
	n         int // total shards
	chunkSize int // size of each chunk to process
}

// NewStreamingEncoder creates a new streaming Reed-Solomon encoder
func NewStreamingEncoder(k, n, chunkSize int) (*StreamingEncoder, error) {
	if k <= 0 || n <= 0 {
		return nil, errors.New("k and n must be positive")
	}
	if k >= n {
		return nil, errors.New("k must be less than n")
	}
	if n-k > 255 {
		return nil, errors.New("too many parity shards (max 255)")
	}
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024 // Default to 1MB chunks
	}

	enc, err := reedsolomon.New(k, n-k)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	return &StreamingEncoder{
		enc:       enc,
		k:         k,
		n:         n,
		chunkSize: chunkSize,
	}, nil
}

// EncodeStream encodes data from reader into shards written to multiple writers
func (se *StreamingEncoder) EncodeStream(reader io.Reader, shardWriters []io.Writer) (int64, error) {
	if len(shardWriters) != se.n {
		return 0, fmt.Errorf("expected %d shard writers, got %d", se.n, len(shardWriters))
	}

	buffer := make([]byte, se.chunkSize)
	var totalBytes int64

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			// Process this chunk
			chunk := buffer[:n]

			// Split chunk into k data shards
			shards, err := se.enc.Split(chunk)
			if err != nil {
				return totalBytes, fmt.Errorf("failed to split chunk: %w", err)
			}

			// Generate parity shards
			if err := se.enc.Encode(shards); err != nil {
				return totalBytes, fmt.Errorf("failed to encode chunk: %w", err)
			}

			// Write each shard to its corresponding writer
			for i, shard := range shards {
				// Write shard size first (4 bytes)
				shardSizeBytes := []byte{
					byte(len(shard) >> 24),
					byte(len(shard) >> 16),
					byte(len(shard) >> 8),
					byte(len(shard)),
				}
				if _, err := shardWriters[i].Write(shardSizeBytes); err != nil {
					return totalBytes, fmt.Errorf("failed to write shard %d size: %w", i, err)
				}

				// Write shard data
				if _, err := shardWriters[i].Write(shard); err != nil {
					return totalBytes, fmt.Errorf("failed to write shard %d data: %w", i, err)
				}
			}

			totalBytes += int64(n)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return totalBytes, fmt.Errorf("failed to read chunk: %w", err)
		}
	}

	// Write end markers to all shards
	endMarker := []byte{0, 0, 0, 0}
	for i, writer := range shardWriters {
		if _, err := writer.Write(endMarker); err != nil {
			return totalBytes, fmt.Errorf("failed to write end marker to shard %d: %w", i, err)
		}
	}

	return totalBytes, nil
}

// EncodeFromReader reads from reader and returns complete shards as byte slices
func (se *StreamingEncoder) EncodeFromReader(reader io.Reader) ([][]byte, error) {
	// Read all data first (for compatibility with existing interface)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// Use existing Encode method
	encoder := &Encoder{
		enc: se.enc,
		k:   se.k,
		n:   se.n,
	}

	return encoder.Encode(data)
}

// ReconstructToWriter reconstructs data from shard readers and writes to output writer
func (se *StreamingEncoder) ReconstructToWriter(shardReaders []io.Reader, writer io.Writer) (int64, error) {
	if len(shardReaders) != se.n {
		return 0, fmt.Errorf("expected %d shard readers, got %d", se.n, len(shardReaders))
	}

	var totalBytes int64
	chunkSizeBytes := make([]byte, 4)

	for {
		// Read chunk sizes from all shards
		chunkSizes := make([]int, se.n)
		allZero := true

		for i, reader := range shardReaders {
			if reader == nil {
				chunkSizes[i] = -1 // Mark as missing shard
				continue
			}

			if _, err := io.ReadFull(reader, chunkSizeBytes); err != nil {
				return totalBytes, fmt.Errorf("failed to read chunk size from shard %d: %w", i, err)
			}

			size := int(chunkSizeBytes[0])<<24 |
				int(chunkSizeBytes[1])<<16 |
				int(chunkSizeBytes[2])<<8 |
				int(chunkSizeBytes[3])

			chunkSizes[i] = size
			if size != 0 {
				allZero = false
			}
		}

		// Check for end condition (all sizes are 0)
		if allZero {
			break
		}

		// Read shard chunks
		shards := make([][]byte, se.n)
		for i, reader := range shardReaders {
			if reader == nil || chunkSizes[i] == -1 {
				shards[i] = nil // Missing shard
				continue
			}

			if chunkSizes[i] == 0 {
				shards[i] = nil // End marker
				continue
			}

			chunk := make([]byte, chunkSizes[i])
			if _, err := io.ReadFull(reader, chunk); err != nil {
				return totalBytes, fmt.Errorf("failed to read chunk from shard %d: %w", i, err)
			}
			shards[i] = chunk
		}

		// Reconstruct this chunk
		if err := se.enc.Reconstruct(shards); err != nil {
			return totalBytes, fmt.Errorf("failed to reconstruct chunk: %w", err)
		}

		// Combine data shards and write to output
		for i := 0; i < se.k; i++ {
			if shards[i] != nil {
				if _, err := writer.Write(shards[i]); err != nil {
					return totalBytes, fmt.Errorf("failed to write reconstructed chunk: %w", err)
				}
				totalBytes += int64(len(shards[i]))
			}
		}
	}

	return totalBytes, nil
}

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
