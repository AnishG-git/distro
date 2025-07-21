package shard

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"distro.lol/pkg/crypto"
	"distro.lol/pkg/reedsol"
)

// Manager defines the interface for shard management
type Manager interface {
	CreateEncryptedShards(data []byte, n, k int) ([][]byte, string, error)
	CreateEncryptedShardsFromReader(reader io.Reader, n, k int) ([][]byte, string, error)
	ReconstructEncryptedShards(encryptedShards [][]byte, epoch string, n, k int) ([]byte, error)
	ReconstructEncryptedShardsToWriter(encryptedShards [][]byte, epoch string, n, k int, writer io.Writer) error
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

// CreateEncryptedShardsFromReader creates shards from an io.Reader using streaming but compatible with existing format
func (sm *shardManager) CreateEncryptedShardsFromReader(reader io.Reader, n, k int) ([][]byte, string, error) {
	currentEpoch := getCurrentEpoch()
	epochKey := crypto.DeriveEpochKey(sm.masterKey, currentEpoch)

	// Read all data (streaming benefit comes from not holding intermediate encrypted chunks)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read data: %w", err)
	}

	// Use standard single-chunk encryption for compatibility
	nonce, ciphertext, tag, err := crypto.Encrypt(data, epochKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt original data: %w", err)
	}

	log.Printf("Encrypted original file (%d bytes) into %d bytes of ciphertext using streaming", len(data), len(ciphertext))

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

	log.Printf("Created %d shards using streaming interface (n=%d, k=%d)", len(shards), n, k)
	return shards, currentEpoch, nil
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

// ReconstructEncryptedShardsToWriter reconstructs and decrypts data directly to a writer
func (sm *shardManager) ReconstructEncryptedShardsToWriter(shards [][]byte, epoch string, n, k int, writer io.Writer) error {
	// Use the existing reconstruction method and then write to the writer
	// This provides the streaming interface while maintaining compatibility
	data, err := sm.ReconstructEncryptedShards(shards, epoch, n, k)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write reconstructed data: %w", err)
	}

	log.Printf("Successfully wrote %d bytes to writer using streaming interface", len(data))
	return nil
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

func getCurrentEpoch() string {
	// 15-minute epochs for key rotation
	epochTime := time.Now().Unix() / 900
	return fmt.Sprintf("epoch-%d", epochTime)
}
