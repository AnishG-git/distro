package reedsol

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeBasic(t *testing.T) {
	k, n := 6, 10 // 6 data shards, 4 parity shards
	data := []byte("Hello, Reed-Solomon! This is a test message that will be split into shards.")

	// Create encoder
	enc, err := NewEncoder(k, n)
	if err != nil {
		t.Fatalf("NewEncoder failed: %v", err)
	}

	// Encode
	shards, err := enc.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(shards) != n {
		t.Errorf("Expected %d shards, got %d", n, len(shards))
	}

	// Verify we can reconstruct with all shards
	reconstructed, err := enc.Reconstruct(shards)
	if err != nil {
		t.Fatalf("Reconstruct with all shards failed: %v", err)
	}

	// The reconstructed data should match original (may have padding)
	if !bytes.HasPrefix(reconstructed, data) {
		t.Errorf("Reconstructed data doesn't start with original data")
	}
}

func TestReconstructWithMissingShards(t *testing.T) {
	k, n := 6, 10
	data := []byte("Test data for Reed-Solomon reconstruction with missing shards.")

	enc, err := NewEncoder(k, n)
	if err != nil {
		t.Fatalf("NewEncoder failed: %v", err)
	}

	shards, err := enc.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Simulate missing shards (lose 4 shards, keeping 6 - exactly the threshold)
	corruptedShards := make([][]byte, len(shards))
	copy(corruptedShards, shards)

	// Set some shards to nil to simulate loss
	corruptedShards[1] = nil
	corruptedShards[3] = nil
	corruptedShards[7] = nil
	corruptedShards[9] = nil

	// Should still be able to reconstruct
	reconstructed, err := enc.Reconstruct(corruptedShards)
	if err != nil {
		t.Fatalf("Reconstruct with missing shards failed: %v", err)
	}

	if !bytes.HasPrefix(reconstructed, data) {
		t.Errorf("Reconstructed data doesn't start with original data")
	}
}

func TestInsufficientShards(t *testing.T) {
	k, n := 6, 10
	data := []byte("Test data")

	enc, err := NewEncoder(k, n)
	if err != nil {
		t.Fatalf("NewEncoder failed: %v", err)
	}

	shards, err := enc.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Lose too many shards (keep only 5, need 6)
	corruptedShards := make([][]byte, len(shards))
	copy(corruptedShards, shards)

	for i := 5; i < len(corruptedShards); i++ {
		corruptedShards[i] = nil
	}

	// Should fail to reconstruct
	_, err = enc.Reconstruct(corruptedShards)
	if err == nil {
		t.Error("Expected reconstruction to fail with insufficient shards")
	}
}

func TestGetShardInfo(t *testing.T) {
	k, n := 6, 10
	data := []byte("Test data for shard info")

	enc, err := NewEncoder(k, n)
	if err != nil {
		t.Fatalf("NewEncoder failed: %v", err)
	}

	shards, err := enc.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	info := enc.GetShardInfo(shards)

	if len(info) != n {
		t.Errorf("Expected %d shard info entries, got %d", n, len(info))
	}

	// Check data shards
	for i := 0; i < k; i++ {
		if !info[i].IsData || info[i].IsParity {
			t.Errorf("Shard %d should be data shard", i)
		}
	}

	// Check parity shards
	for i := k; i < n; i++ {
		if info[i].IsData || !info[i].IsParity {
			t.Errorf("Shard %d should be parity shard", i)
		}
	}
}
