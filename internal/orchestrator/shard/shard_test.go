package shard

import (
	"bytes"
	"context"
	"testing"

	"distro.lol/pkg/crypto"
)

func TestShardManager_CreateEncryptedShards(t *testing.T) {
	masterKey, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	ctx := context.Background()
	sm := NewManager(ctx, masterKey)

	originalData := []byte("This is a test for CreateEncryptedShards function.")
	n, k := 7, 4

	// Test CreateEncryptedShards
	encryptedShards, currentEpoch, err := sm.CreateEncryptedShards(originalData, n, k)
	if err != nil {
		t.Fatalf("CreateEncryptedShards failed: %v", err)
	}

	// Assert correct number of shards
	if len(encryptedShards) != n {
		t.Errorf("Expected %d shards, got %d", n, len(encryptedShards))
	}

	// Assert all shards are non-nil and have data
	for i, shard := range encryptedShards {
		if shard == nil {
			t.Errorf("Shard %d is nil", i)
		}
		if len(shard) == 0 {
			t.Errorf("Shard %d is empty", i)
		}
	}

	// Assert epoch is returned
	if currentEpoch == "" {
		t.Error("Expected non-empty epoch")
	}

	t.Logf("Successfully created %d shards with epoch %s", len(encryptedShards), currentEpoch)
}

func TestShardManager_ReconstructEncryptedShards(t *testing.T) {
	masterKey, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	ctx := context.Background()
	sm := NewManager(ctx, masterKey)

	originalData := []byte("This is a test for ReconstructEncryptedShards function.")
	n, k := 6, 4

	// First create shards
	encryptedShards, currentEpoch, err := sm.CreateEncryptedShards(originalData, n, k)
	if err != nil {
		t.Fatalf("Failed to create encrypted shards: %v", err)
	}

	// Test ReconstructEncryptedShards with all shards
	reconstructed, err := sm.ReconstructEncryptedShards(encryptedShards, currentEpoch, n, k)
	if err != nil {
		t.Fatalf("ReconstructEncryptedShards failed: %v", err)
	}

	// Assert reconstructed data matches original
	if !bytes.Equal(reconstructed, originalData) {
		t.Errorf("Reconstructed data does not match original.\nOriginal: %q\nReconstructed: %q", originalData, reconstructed)
	}

	// Test with minimum required shards (simulate missing shards)
	minimalShards := make([][]byte, n)
	copy(minimalShards[:k], encryptedShards[:k])
	// The rest remain nil

	reconstructedMinimal, err := sm.ReconstructEncryptedShards(minimalShards, currentEpoch, n, k)
	if err != nil {
		t.Fatalf("Failed to reconstruct from minimal shards: %v", err)
	}

	if !bytes.Equal(reconstructedMinimal, originalData) {
		t.Errorf("Reconstructed data from minimal shards does not match original.\nOriginal: %q\nReconstructed: %q", originalData, reconstructedMinimal)
	}

	t.Logf("Successfully reconstructed %d bytes from shards", len(reconstructed))
}

func TestShardManager_EndToEndFlow(t *testing.T) {
	masterKey, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	ctx := context.Background()
	sm := NewManager(ctx, masterKey)

	// Test with larger data
	originalData := []byte("This is a comprehensive end-to-end test for the complete encrypt-shard and reconstruct flow with a longer message to ensure proper handling of various data sizes.")
	n, k := 10, 6

	// Test the complete flow: CreateEncryptedShards -> ReconstructEncryptedShards
	encryptedShards, currentEpoch, err := sm.CreateEncryptedShards(originalData, n, k)
	if err != nil {
		t.Fatalf("Failed to create encrypted shards: %v", err)
	}

	reconstructed, err := sm.ReconstructEncryptedShards(encryptedShards, currentEpoch, n, k)
	if err != nil {
		t.Fatalf("Failed to reconstruct data: %v", err)
	}

	if !bytes.Equal(reconstructed, originalData) {
		t.Errorf("End-to-end flow failed: data mismatch")
	}

	t.Logf("End-to-end test passed for %d byte payload", len(originalData))
}

func TestShardManager_ErrorConditions(t *testing.T) {
	masterKey, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	ctx := context.Background()
	sm := NewManager(ctx, masterKey)

	originalData := []byte("Test data for error conditions")
	n, k := 5, 3

	// Create valid shards first
	encryptedShards, currentEpoch, err := sm.CreateEncryptedShards(originalData, n, k)
	if err != nil {
		t.Fatalf("Failed to create encrypted shards: %v", err)
	}

	// Test insufficient shards
	insufficientShards := encryptedShards[:k-1] // Less than k shards
	_, err = sm.ReconstructEncryptedShards(insufficientShards, currentEpoch, len(insufficientShards), k)
	if err == nil {
		t.Error("Expected error for insufficient shards, but got none")
	}

	// Test with wrong epoch
	wrongEpoch := "wrong-epoch-123"
	_, err = sm.ReconstructEncryptedShards(encryptedShards, wrongEpoch, n, k)
	if err == nil {
		t.Error("Expected error for wrong epoch, but got none")
	}

	// Test with corrupted shard
	corruptedShards := make([][]byte, len(encryptedShards))
	copy(corruptedShards, encryptedShards)
	if len(corruptedShards[0]) > 0 {
		corruptedShards[0][0] ^= 0xFF // Flip bits to corrupt the shard
	}
	_, err = sm.ReconstructEncryptedShards(corruptedShards, currentEpoch, n, k)
	if err == nil {
		t.Error("Expected error for corrupted shard, but got none")
	}

	t.Log("Error condition tests passed")
}

func TestVerifyShards(t *testing.T) {
	masterKey, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	ctx := context.Background()
	sm := NewManager(ctx, masterKey)

	originalData := []byte("Test data for shard verification functionality")
	n, k := 8, 5

	// Create valid shards
	encryptedShards, _, err := sm.CreateEncryptedShards(originalData, n, k)
	if err != nil {
		t.Fatalf("Failed to create encrypted shards: %v", err)
	}

	// Test 1: Verify valid shards - should pass
	err = VerifyShards(encryptedShards, n, k)
	if err != nil {
		t.Errorf("Expected verification to pass for valid shards, got error: %v", err)
	}
	t.Log("✓ Valid shards verification passed")

	// Test 2: Test with wrong number of shards
	wrongShards := encryptedShards[:n-1] // One less shard
	err = VerifyShards(wrongShards, n, k)
	if err == nil {
		t.Error("Expected error for wrong number of shards, but got none")
	}
	t.Log("✓ Wrong shard count detection passed")

	// Test 3: Test with some nil shards (within tolerance)
	shardsWithNils := make([][]byte, len(encryptedShards))
	copy(shardsWithNils, encryptedShards)
	shardsWithNils[1] = nil // Set one shard to nil
	shardsWithNils[3] = nil // Set another shard to nil
	shardsWithNils[7] = nil // Set third shard to nil (3 nil shards, within n-k=3 tolerance)

	err = VerifyShards(shardsWithNils, n, k)
	if err != nil {
		t.Errorf("Expected verification to pass with %d nil shards (within tolerance), got error: %v", 3, err)
	}
	t.Log("✓ Nil shards within tolerance passed")

	// Test 4: Test with too many nil shards (exceeds tolerance)
	tooManyNils := make([][]byte, len(encryptedShards))
	copy(tooManyNils, encryptedShards)
	for i := 0; i < n-k+1; i++ { // Set n-k+1 shards to nil (exceeds tolerance)
		tooManyNils[i] = nil
	}

	err = VerifyShards(tooManyNils, n, k)
	if err == nil {
		t.Error("Expected error for too many nil shards, but got none")
	}
	t.Log("✓ Too many nil shards detection passed")

	// Test 5: Test with corrupted shard
	corruptedShards := make([][]byte, len(encryptedShards))
	copy(corruptedShards, encryptedShards)
	if len(corruptedShards[0]) > 0 {
		corruptedShards[0][0] ^= 0xFF // Flip bits to corrupt the first shard
	}

	err = VerifyShards(corruptedShards, n, k)
	if err == nil {
		t.Error("Expected error for corrupted shard, but got none")
	}
	t.Log("✓ Corrupted shard detection passed")

	// Test 6: Test with multiple corrupted shards
	multiCorrupted := make([][]byte, len(encryptedShards))
	copy(multiCorrupted, encryptedShards)
	if len(multiCorrupted[0]) > 0 {
		multiCorrupted[0][0] ^= 0xFF // Corrupt first shard
	}
	if len(multiCorrupted[2]) > 5 {
		multiCorrupted[2][5] ^= 0x55 // Corrupt third shard
	}

	err = VerifyShards(multiCorrupted, n, k)
	if err == nil {
		t.Error("Expected error for multiple corrupted shards, but got none")
	}
	t.Log("✓ Multiple corrupted shards detection passed")

	// Test 7: Test edge case with minimal valid shards
	minimalValid := make([][]byte, n)
	copy(minimalValid[:k], encryptedShards[:k]) // Only k valid shards, rest nil

	err = VerifyShards(minimalValid, n, k)
	if err != nil {
		t.Errorf("Expected verification to pass with exactly k valid shards, got error: %v", err)
	}
	t.Log("✓ Minimal valid shards verification passed")

	// Test 8: Test with invalid k/n parameters
	err = VerifyShards(encryptedShards, 0, k)
	if err == nil {
		t.Error("Expected error for invalid n=0 parameter, but got none")
	}

	err = VerifyShards(encryptedShards, n, 0)
	if err == nil {
		t.Error("Expected error for invalid k=0 parameter, but got none")
	}
	t.Log("✓ Invalid parameter detection passed")

	t.Log("All VerifyShards tests passed successfully")
}
