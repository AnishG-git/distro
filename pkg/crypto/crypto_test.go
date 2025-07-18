package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := []byte("12345678901234567890123456789012") // 32 bytes for AES-256
	plaintext := []byte("Hello, World! This is a secret message.")

	// Encrypt
	nonce, ciphertext, tag, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Decrypt
	decrypted, err := Decrypt(nonce, ciphertext, tag, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	// Verify
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted text doesn't match original. Got %s, want %s", decrypted, plaintext)
	}
}

func TestDeriveEpochKey(t *testing.T) {
	masterKey := []byte("master-key-12345678901234567890")
	epoch1 := "2025-01-01"
	epoch2 := "2025-01-02"

	key1 := DeriveEpochKey(masterKey, epoch1)
	key2 := DeriveEpochKey(masterKey, epoch2)

	if len(key1) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key1))
	}

	if bytes.Equal(key1, key2) {
		t.Error("Different epochs should produce different keys")
	}

	// Same epoch should produce same key
	key1Again := DeriveEpochKey(masterKey, epoch1)
	if !bytes.Equal(key1, key1Again) {
		t.Error("Same epoch should produce same key")
	}
}

func TestEncryptDecryptShard(t *testing.T) {
	masterKey := []byte("master-key-12345678901234567890")
	data := []byte("This is shard data")
	shardID := "shard-123"
	epoch := "2025-01-01"
	index := 5

	// Encrypt shard
	encShard, err := EncryptShard(data, shardID, epoch, index, masterKey)
	if err != nil {
		t.Fatalf("EncryptShard failed: %v", err)
	}

	// Verify metadata
	if encShard.ShardID != shardID {
		t.Errorf("Expected ShardID %s, got %s", shardID, encShard.ShardID)
	}
	if encShard.Epoch != epoch {
		t.Errorf("Expected Epoch %s, got %s", epoch, encShard.Epoch)
	}
	if encShard.Index != index {
		t.Errorf("Expected Index %d, got %d", index, encShard.Index)
	}

	// Decrypt shard
	decrypted, err := DecryptShard(encShard, masterKey)
	if err != nil {
		t.Fatalf("DecryptShard failed: %v", err)
	}

	// Verify data
	if !bytes.Equal(data, decrypted) {
		t.Errorf("Decrypted shard data doesn't match original. Got %s, want %s", decrypted, data)
	}
}
