package shard

import (
	"context"
	"testing"

	"distro.lol/pkg/crypto"
)

func TestShardManager_EncryptShardAndReconstructFlow(t *testing.T) {
	masterKey, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	ctx := context.Background()
	sm := NewManager(ctx, masterKey)

	// Use a non-trivial data payload
	originalData := []byte("This is a comprehensive test for the full encrypt-shard and reconstruct flow.")
	n, k := 7, 4

	// Encrypt and shard the data
	encryptedShards, currentEpoch, err := sm.CreateEncryptedShards(originalData, n, k)
	if err != nil {
		t.Fatalf("Failed to create encrypted shards: %v", err)
	}

	// Assert correct number of shards
	if len(encryptedShards) != n {
		t.Errorf("Expected %d shards, got %d", n, len(encryptedShards))
	}

	// Assert all shards are non-nil and have expected minimum size
	for i, shard := range encryptedShards {
		if shard == nil {
			t.Errorf("Shard %d is nil", i)
		}
		if len(shard) < 32 { // Should be at least nonce+tag
			t.Errorf("Shard %d is unexpectedly small: %d bytes", i, len(shard))
		}
	}

	// Reconstruct the data from all shards
	reconstructed, err := sm.ReconstructEncryptedShards(encryptedShards, currentEpoch, n, k)
	if err != nil {
		t.Fatalf("Failed to reconstruct data from shards: %v", err)
	}

	// Assert reconstructed data matches original
	if string(reconstructed) != string(originalData) {
		t.Errorf("Reconstructed data does not match original.\nOriginal: %q\nReconstructed: %q", originalData, reconstructed)
	}

	// Remove some shards (simulate missing shards) and reconstruct with minimum required
	minimalShards := make([][]byte, n)
	copy(minimalShards[:k], encryptedShards[:k])
	// The rest remain nil

	reconstructedMinimal, err := sm.ReconstructEncryptedShards(minimalShards, currentEpoch, n, k)
	if err != nil {
		t.Fatalf("Failed to reconstruct from minimal shards: %v", err)
	}
	if string(reconstructedMinimal) != string(originalData) {
		t.Errorf("Reconstructed data from minimal shards does not match original.\nOriginal: %q\nReconstructed: %q", originalData, reconstructedMinimal)
	}
}
