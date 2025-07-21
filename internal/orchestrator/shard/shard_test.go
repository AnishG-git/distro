package shard

import (
	"bytes"
	"context"
	"strings"
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

func TestShardManager_CreateEncryptedShardsFromReader(t *testing.T) {
	masterKey, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	ctx := context.Background()
	sm := NewManager(ctx, masterKey)

	originalData := []byte("This is a test for CreateEncryptedShardsFromReader function.")
	reader := bytes.NewReader(originalData)
	n, k := 5, 3

	// Test CreateEncryptedShardsFromReader
	encryptedShards, currentEpoch, err := sm.CreateEncryptedShardsFromReader(reader, n, k)
	if err != nil {
		t.Fatalf("CreateEncryptedShardsFromReader failed: %v", err)
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

	t.Logf("Successfully created %d shards from reader with epoch %s", len(encryptedShards), currentEpoch)
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

func TestShardManager_ReconstructEncryptedShardsToWriter(t *testing.T) {
	masterKey, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	ctx := context.Background()
	sm := NewManager(ctx, masterKey)

	originalData := []byte("This is a test for ReconstructEncryptedShardsToWriter function.")
	n, k := 8, 5

	// First create shards
	encryptedShards, currentEpoch, err := sm.CreateEncryptedShards(originalData, n, k)
	if err != nil {
		t.Fatalf("Failed to create encrypted shards: %v", err)
	}

	// Test ReconstructEncryptedShardsToWriter
	var buf bytes.Buffer
	err = sm.ReconstructEncryptedShardsToWriter(encryptedShards, currentEpoch, n, k, &buf)
	if err != nil {
		t.Fatalf("ReconstructEncryptedShardsToWriter failed: %v", err)
	}

	// Assert reconstructed data matches original
	reconstructed := buf.Bytes()
	if !bytes.Equal(reconstructed, originalData) {
		t.Errorf("Reconstructed data does not match original.\nOriginal: %q\nReconstructed: %q", originalData, reconstructed)
	}

	// Test with a string writer (different io.Writer implementation)
	var strBuilder strings.Builder
	err = sm.ReconstructEncryptedShardsToWriter(encryptedShards, currentEpoch, n, k, &strBuilder)
	if err != nil {
		t.Fatalf("ReconstructEncryptedShardsToWriter with strings.Builder failed: %v", err)
	}

	if strBuilder.String() != string(originalData) {
		t.Errorf("Reconstructed string does not match original.\nOriginal: %q\nReconstructed: %q", originalData, strBuilder.String())
	}

	t.Logf("Successfully wrote %d bytes to writer", len(reconstructed))
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

	// Test the reader -> writer flow
	reader := bytes.NewReader(originalData)
	encryptedShards2, currentEpoch2, err := sm.CreateEncryptedShardsFromReader(reader, n, k)
	if err != nil {
		t.Fatalf("Failed to create encrypted shards from reader: %v", err)
	}

	var buf bytes.Buffer
	err = sm.ReconstructEncryptedShardsToWriter(encryptedShards2, currentEpoch2, n, k, &buf)
	if err != nil {
		t.Fatalf("Failed to reconstruct to writer: %v", err)
	}

	if !bytes.Equal(buf.Bytes(), originalData) {
		t.Errorf("Reader->Writer flow failed: data mismatch")
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
