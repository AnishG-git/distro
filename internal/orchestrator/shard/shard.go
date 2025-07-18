package shard

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"distro.lol/pkg/crypto"
	"distro.lol/pkg/reedsol"
)

// Manager defines the interface for shard management
type Manager interface {
	CreateEncryptedShards(data []byte, n, k int) ([][]byte, string, error)
	ReconstructEncryptedShards(encryptedShards [][]byte, epoch string, n, k int) ([]byte, error)
}

// shardManager handles shard creation, encryption, and decryption
type shardManager struct {
	masterKey []byte
	ctx       context.Context
}

// NewManager creates a new shard manager that satisfies the Manager interface
func NewManager(ctx context.Context, masterKey []byte) *shardManager {
	return &shardManager{
		masterKey: masterKey,
		ctx:       ctx,
	}
}

// CreateShards splits data into Reed-Solomon encoded shards
func (sm *shardManager) CreateEncryptedShards(data []byte, n, k int) ([][]byte, string, error) {
	encoder, err := reedsol.NewEncoder(k, n)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	// Prepend original data length (8 bytes) to preserve exact length
	dataWithLength := make([]byte, 8+len(data))

	// Store length as big-endian 64-bit integer
	dataWithLength[0] = byte(len(data) >> 56)
	dataWithLength[1] = byte(len(data) >> 48)
	dataWithLength[2] = byte(len(data) >> 40)
	dataWithLength[3] = byte(len(data) >> 32)
	dataWithLength[4] = byte(len(data) >> 24)
	dataWithLength[5] = byte(len(data) >> 16)
	dataWithLength[6] = byte(len(data) >> 8)
	dataWithLength[7] = byte(len(data))
	copy(dataWithLength[8:], data)

	shards, err := encoder.Encode(dataWithLength)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode data into shards: %w", err)
	}

	log.Printf("Created %d shards from %d bytes of data (n=%d, k=%d)",
		len(shards), len(data), n, k)

	currentEpoch := getCurrentEpoch()

	// Encrypt shards concurrently
	encryptedShards := make([][]byte, len(shards))
	var wg sync.WaitGroup
	errChan := make(chan error, len(shards))

	for i, shard := range shards {
		wg.Add(1)
		go func(index int, shardData []byte) {
			defer wg.Done()

			nonce, ciphertext, tag, err := sm.encryptShard(shardData, currentEpoch)
			if err != nil {
				errChan <- fmt.Errorf("failed to encrypt shard %d: %w", index, err)
				return
			}

			// Combine nonce + ciphertext + tag into single encrypted shard
			encryptedShard := make([]byte, len(nonce)+len(ciphertext)+len(tag))
			copy(encryptedShard, nonce)
			copy(encryptedShard[len(nonce):], ciphertext)
			copy(encryptedShard[len(nonce)+len(ciphertext):], tag)

			encryptedShards[index] = encryptedShard
		}(i, shard)
	}

	wg.Wait()
	close(errChan)

	// Check for any encryption errors
	if len(errChan) > 0 {
		return nil, "", <-errChan
	}

	log.Printf("Successfully encrypted %d shards", len(encryptedShards))
	return encryptedShards, currentEpoch, nil
}

// EncryptShard encrypts a shard using epoch-derived keys
func (sm *shardManager) encryptShard(data []byte, epoch string) (nonce, ciphertext, tag []byte, err error) {
	epochKey := crypto.DeriveEpochKey(sm.masterKey, epoch)

	nonce, ciphertext, tag, err = crypto.Encrypt(data, epochKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to encrypt shard: %w", err)
	}

	return nonce, ciphertext, tag, nil
}

// ReconstructEncryptedShards decrypts shards concurrently and reconstructs the original data
func (sm *shardManager) ReconstructEncryptedShards(encryptedShards [][]byte, epoch string, n, k int) ([]byte, error) {
	if len(encryptedShards) < k {
		return nil, fmt.Errorf("insufficient shards: need at least %d, got %d", k, len(encryptedShards))
	}

	// Decrypt shards concurrently
	decryptedShards := make([][]byte, len(encryptedShards))
	var wg sync.WaitGroup
	errChan := make(chan error, len(encryptedShards))

	for i, encryptedShard := range encryptedShards {
		if encryptedShard == nil {
			continue // Skip missing shards
		}

		wg.Add(1)
		go func(index int, encShard []byte) {
			defer wg.Done()

			decrypted, err := sm.decryptShard(encShard, epoch)
			if err != nil {
				errChan <- fmt.Errorf("failed to decrypt shard %d: %w", index, err)
				return
			}

			decryptedShards[index] = decrypted
		}(i, encryptedShard)
	}

	wg.Wait()
	close(errChan)

	// Check for any decryption errors
	if len(errChan) > 0 {
		return nil, <-errChan
	}

	log.Printf("Successfully decrypted %d shards", len(decryptedShards))

	// Reconstruct original data using Reed-Solomon decoder
	encoder, err := reedsol.NewEncoder(k, n)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	originalData, err := encoder.Reconstruct(decryptedShards)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct data from shards: %w", err)
	}

	// Extract the original data length from the first 8 bytes
	if len(originalData) < 8 {
		return nil, fmt.Errorf("reconstructed data too short to contain length prefix")
	}

	// Decode length as big-endian 64-bit integer
	originalLength := int(originalData[0])<<56 |
		int(originalData[1])<<48 |
		int(originalData[2])<<40 |
		int(originalData[3])<<32 |
		int(originalData[4])<<24 |
		int(originalData[5])<<16 |
		int(originalData[6])<<8 |
		int(originalData[7])

	// Validate length
	if originalLength < 0 || originalLength > len(originalData)-8 {
		return nil, fmt.Errorf("invalid original data length: %d", originalLength)
	}

	// Extract the original data (skip the 8-byte length prefix)
	actualData := originalData[8 : 8+originalLength]

	log.Printf("Successfully reconstructed %d bytes from %d shards", len(actualData), len(decryptedShards))
	return actualData, nil
}

// decryptShard decrypts a single encrypted shard
func (sm *shardManager) decryptShard(encryptedShard []byte, epoch string) ([]byte, error) {
	const nonceSize = 12 // AES-GCM nonce size
	const tagSize = 16   // AES-GCM tag size

	if len(encryptedShard) < nonceSize+tagSize {
		return nil, fmt.Errorf("encrypted shard too small: got %d bytes, need at least %d",
			len(encryptedShard), nonceSize+tagSize)
	}

	// Extract nonce, ciphertext, and tag from encrypted shard
	nonce := encryptedShard[:nonceSize]
	ciphertext := encryptedShard[nonceSize : len(encryptedShard)-tagSize]
	tag := encryptedShard[len(encryptedShard)-tagSize:]

	// Derive the epoch key
	epochKey := crypto.DeriveEpochKey(sm.masterKey, epoch)

	// Decrypt the shard
	plaintext, err := crypto.Decrypt(nonce, ciphertext, tag, epochKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt shard: %w", err)
	}

	return plaintext, nil
}

func getCurrentEpoch() string {
	// 15-minute epochs for key rotation
	epochTime := time.Now().Unix() / 900
	return fmt.Sprintf("epoch-%d", epochTime)
}
