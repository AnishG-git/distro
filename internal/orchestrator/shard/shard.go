package shard

import (
	"bytes"
	"context"
	"fmt"
	"log"
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
	chunkSize int // Size of chunks to read at a time
}

// NewManager creates a new shard manager that satisfies the Manager interface
func NewManager(ctx context.Context, masterKey []byte) *shardManager {
	return &shardManager{
		masterKey: masterKey,
		ctx:       ctx,
		chunkSize: 1024 * 1024, // 1MB chunks by default
	}
}

// createEncryptedShardsInternal handles the core logic for both methods
func (sm *shardManager) createEncryptedShardsInternal(data []byte, n, k int, currentEpoch string) ([][]byte, string, error) {
	// First, encrypt the entire original file
	epochKey := crypto.DeriveEpochKey(sm.masterKey, currentEpoch)
	nonce, ciphertext, tag, err := crypto.Encrypt(data, epochKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt original data: %w", err)
	}

	log.Printf("Encrypted original file (%d bytes) into %d bytes of ciphertext", len(data), len(ciphertext))

	// Combine nonce + ciphertext + tag into single encrypted blob
	encryptedData := make([]byte, len(nonce)+len(ciphertext)+len(tag))
	copy(encryptedData, nonce)
	copy(encryptedData[len(nonce):], ciphertext)
	copy(encryptedData[len(nonce)+len(ciphertext):], tag)

	// Prepend encrypted data length (8 bytes) to preserve exact length for reconstruction
	dataWithLength := make([]byte, 8+len(encryptedData))

	// Store length as big-endian 64-bit integer
	dataWithLength[0] = byte(len(encryptedData) >> 56)
	dataWithLength[1] = byte(len(encryptedData) >> 48)
	dataWithLength[2] = byte(len(encryptedData) >> 40)
	dataWithLength[3] = byte(len(encryptedData) >> 32)
	dataWithLength[4] = byte(len(encryptedData) >> 24)
	dataWithLength[5] = byte(len(encryptedData) >> 16)
	dataWithLength[6] = byte(len(encryptedData) >> 8)
	dataWithLength[7] = byte(len(encryptedData))
	copy(dataWithLength[8:], encryptedData)

	// Now create Reed-Solomon encoder and split encrypted data into shards
	encoder, err := reedsol.NewEncoder(k, n)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	shards, err := encoder.Encode(dataWithLength)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode encrypted data into shards: %w", err)
	}

	log.Printf("Created %d shards from encrypted data (n=%d, k=%d)", len(shards), n, k)
	return shards, currentEpoch, nil
}

// CreateEncryptedShards - keep the original method for backward compatibility
func (sm *shardManager) CreateEncryptedShards(data []byte, n, k int) ([][]byte, string, error) {
	currentEpoch := getCurrentEpoch()
	return sm.createEncryptedShardsInternal(data, n, k, currentEpoch)
}

// decryptOriginalData decrypts the reconstructed encrypted data
func (sm *shardManager) decryptOriginalData(encryptedData []byte, epoch string) ([]byte, error) {
	const nonceSize = 12 // AES-GCM nonce size
	const tagSize = 16   // AES-GCM tag size

	if len(encryptedData) < nonceSize+tagSize {
		return nil, fmt.Errorf("encrypted data too small: got %d bytes, need at least %d",
			len(encryptedData), nonceSize+tagSize)
	}

	// Extract nonce, ciphertext, and tag from encrypted data
	nonce := encryptedData[:nonceSize]
	ciphertext := encryptedData[nonceSize : len(encryptedData)-tagSize]
	tag := encryptedData[len(encryptedData)-tagSize:]

	// Derive the epoch key
	epochKey := crypto.DeriveEpochKey(sm.masterKey, epoch)

	// Decrypt the data
	plaintext, err := crypto.Decrypt(nonce, ciphertext, tag, epochKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// ReconstructEncryptedShards reconstructs shards and then decrypts the original data
func (sm *shardManager) ReconstructEncryptedShards(shards [][]byte, epoch string, n, k int) ([]byte, error) {
	if len(shards) < k {
		return nil, fmt.Errorf("insufficient shards: need at least %d, got %d", k, len(shards))
	}

	log.Printf("Starting reconstruction with %d shards", len(shards))

	// Reconstruct encrypted data using Reed-Solomon decoder
	encoder, err := reedsol.NewEncoder(k, n)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	reconstructedData, err := encoder.Reconstruct(shards)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct data from shards: %w", err)
	}

	// Extract the encrypted data length from the first 8 bytes
	if len(reconstructedData) < 8 {
		return nil, fmt.Errorf("reconstructed data too short to contain length prefix")
	}

	// Decode length as big-endian 64-bit integer
	encryptedLength := int(reconstructedData[0])<<56 |
		int(reconstructedData[1])<<48 |
		int(reconstructedData[2])<<40 |
		int(reconstructedData[3])<<32 |
		int(reconstructedData[4])<<24 |
		int(reconstructedData[5])<<16 |
		int(reconstructedData[6])<<8 |
		int(reconstructedData[7])

	// Validate length
	if encryptedLength < 0 || encryptedLength > len(reconstructedData)-8 {
		return nil, fmt.Errorf("invalid encrypted data length: %d", encryptedLength)
	}

	// Extract the encrypted data (skip the 8-byte length prefix)
	encryptedData := reconstructedData[8 : 8+encryptedLength]

	log.Printf("Reconstructed %d bytes of encrypted data from shards", len(encryptedData))

	// Now decrypt the reconstructed encrypted data
	decryptedData, err := sm.decryptOriginalData(encryptedData, epoch)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt reconstructed data: %w", err)
	}

	log.Printf("Successfully decrypted reconstructed data: %d bytes", len(decryptedData))
	return decryptedData, nil
}

func VerifyShards(shards [][]byte, n, k int) error {
	if len(shards) != n {
		return fmt.Errorf("invalid number of shards: expected %d, got %d", n, len(shards))
	}

	// Create Reed-Solomon encoder for verification
	encoder, err := reedsol.NewEncoder(k, n)
	if err != nil {
		return fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	// Check for nil shards and log their indices
	nilShards := make([]int, 0)
	validShards := make([][]byte, len(shards))
	copy(validShards, shards)

	for i, shard := range shards {
		if shard == nil {
			nilShards = append(nilShards, i)
		}
	}

	if len(nilShards) > 0 {
		log.Printf("Found nil shards at indices: %v", nilShards)
	}

	// If we have too many nil shards, we can't verify
	if len(nilShards) > n-k {
		return fmt.Errorf("too many missing shards: %d missing, can only tolerate %d", len(nilShards), n-k)
	}

	// For Reed-Solomon verification, we need to handle nil shards properly
	// The library expects consistent shard sizes, so we can't directly verify with nils
	// Instead, we'll use a more sophisticated approach

	validShardCount := 0
	for _, shard := range shards {
		if shard != nil {
			validShardCount++
		}
	}

	// Check if we have enough valid shards for verification
	if validShardCount < k {
		return fmt.Errorf("insufficient shards for verification: have %d valid shards, need at least %d", validShardCount, k)
	}

	// For shards without nils, we can use direct verification
	if len(nilShards) == 0 {
		isValid, err := encoder.Verify(validShards)
		if err != nil {
			return fmt.Errorf("shard verification failed: %w", err)
		}
		if !isValid {
			return fmt.Errorf("shard verification failed: shards are invalid")
		}
	} else {
		// For shards with nils, we need to verify by attempting reconstruction
		// and checking if it produces consistent results
		_, err := encoder.Reconstruct(validShards)
		if err != nil {
			// Try to identify which specific shards are corrupted
			corruptedShards := make([]int, 0)

			// Test each non-nil shard individually by setting it to nil and seeing if reconstruction improves
			for i := 0; i < len(shards); i++ {
				if shards[i] == nil {
					continue // Skip already nil shards
				}

				// Create a copy with this shard set to nil
				testShards := make([][]byte, len(shards))
				copy(testShards, shards)
				testShards[i] = nil

				// Count remaining valid shards
				remainingValid := 0
				for _, testShard := range testShards {
					if testShard != nil {
						remainingValid++
					}
				}

				// If we still have enough shards and reconstruction works without this shard,
				// then this shard might be corrupted
				if remainingValid >= k {
					if _, reconstructErr := encoder.Reconstruct(testShards); reconstructErr == nil {
						corruptedShards = append(corruptedShards, i)
					}
				}
			}

			if len(corruptedShards) > 0 {
				log.Printf("Detected corrupted shards at indices: %v", corruptedShards)
				return fmt.Errorf("shard verification failed: corrupted shards detected at indices %v", corruptedShards)
			}

			return fmt.Errorf("shard verification failed: %w", err)
		}

		// Additional verification: try to verify individual groups of shards
		// If we have corrupted shards, different combinations should give different results
		if validShardCount > k {
			// Try reconstruction with different combinations to detect inconsistencies
			firstReconstructed, err := encoder.Reconstruct(validShards)
			if err != nil {
				return fmt.Errorf("shard verification failed during reconstruction: %w", err)
			}

			// Create alternative shard combinations and see if they produce the same result
			for i := 0; i < len(shards) && validShardCount > k; i++ {
				if shards[i] == nil {
					continue
				}

				// Create alternative combination by removing one valid shard
				altShards := make([][]byte, len(shards))
				copy(altShards, validShards)
				altShards[i] = nil

				// Count remaining shards
				altValidCount := 0
				for _, altShard := range altShards {
					if altShard != nil {
						altValidCount++
					}
				}

				if altValidCount >= k {
					altReconstructed, altErr := encoder.Reconstruct(altShards)
					if altErr == nil {
						// Compare lengths first (faster)
						if len(firstReconstructed) != len(altReconstructed) {
							log.Printf("Detected inconsistent reconstruction - shard %d may be corrupted", i)
							return fmt.Errorf("shard verification failed: inconsistent reconstruction detected, shard %d may be corrupted", i)
						}

						// Compare content if lengths match
						if !bytes.Equal(firstReconstructed, altReconstructed) {
							log.Printf("Detected inconsistent reconstruction - shard %d may be corrupted", i)
							return fmt.Errorf("shard verification failed: inconsistent reconstruction detected, shard %d may be corrupted", i)
						}
					}
				}
			}
		}
	}

	log.Printf("Successfully verified %d shards (%d data + %d parity)", n, k, n-k)
	return nil
}

func getCurrentEpoch() string {
	// 15-minute epochs for key rotation
	epochTime := time.Now().Unix() / 900
	return fmt.Sprintf("epoch-%d", epochTime)
}
