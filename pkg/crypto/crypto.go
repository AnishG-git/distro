package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// StreamingEncryptor handles streaming encryption
type StreamingEncryptor struct {
	gcm   cipher.AEAD
	nonce []byte
}

// StreamingDecryptor handles streaming decryption
type StreamingDecryptor struct {
	gcm   cipher.AEAD
	nonce []byte
}

// NewStreamingEncryptor creates a new streaming encryptor
func NewStreamingEncryptor(key []byte) (*StreamingEncryptor, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return &StreamingEncryptor{
		gcm:   gcm,
		nonce: nonce,
	}, nil
}

// EncryptStream encrypts data from reader to writer in chunks
func (se *StreamingEncryptor) EncryptStream(reader io.Reader, writer io.Writer, chunkSize int) (int64, error) {
	// Write nonce first
	if _, err := writer.Write(se.nonce); err != nil {
		return 0, fmt.Errorf("failed to write nonce: %w", err)
	}

	buffer := make([]byte, chunkSize)
	var totalBytes int64

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			// Encrypt chunk
			encrypted := se.gcm.Seal(nil, se.nonce, buffer[:n], nil)

			// Write encrypted chunk size first (4 bytes)
			chunkSizeBytes := []byte{
				byte(len(encrypted) >> 24),
				byte(len(encrypted) >> 16),
				byte(len(encrypted) >> 8),
				byte(len(encrypted)),
			}
			if _, err := writer.Write(chunkSizeBytes); err != nil {
				return totalBytes, fmt.Errorf("failed to write chunk size: %w", err)
			}

			// Write encrypted chunk
			if _, err := writer.Write(encrypted); err != nil {
				return totalBytes, fmt.Errorf("failed to write encrypted chunk: %w", err)
			}

			totalBytes += int64(n)

			// Update nonce for next chunk (simple increment)
			incrementNonce(se.nonce)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return totalBytes, fmt.Errorf("failed to read chunk: %w", err)
		}
	}

	// Write end marker (chunk size of 0)
	endMarker := []byte{0, 0, 0, 0}
	if _, err := writer.Write(endMarker); err != nil {
		return totalBytes, fmt.Errorf("failed to write end marker: %w", err)
	}

	return totalBytes, nil
}

// NewStreamingDecryptor creates a new streaming decryptor
func NewStreamingDecryptor(key []byte) (*StreamingDecryptor, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &StreamingDecryptor{
		gcm: gcm,
	}, nil
}

// DecryptStream decrypts data from reader to writer in chunks
func (sd *StreamingDecryptor) DecryptStream(reader io.Reader, writer io.Writer) (int64, error) {
	// Read nonce first
	nonce := make([]byte, sd.gcm.NonceSize())
	if _, err := io.ReadFull(reader, nonce); err != nil {
		return 0, fmt.Errorf("failed to read nonce: %w", err)
	}
	sd.nonce = nonce

	var totalBytes int64
	chunkSizeBytes := make([]byte, 4)

	for {
		// Read chunk size
		if _, err := io.ReadFull(reader, chunkSizeBytes); err != nil {
			return totalBytes, fmt.Errorf("failed to read chunk size: %w", err)
		}

		chunkSize := int(chunkSizeBytes[0])<<24 |
			int(chunkSizeBytes[1])<<16 |
			int(chunkSizeBytes[2])<<8 |
			int(chunkSizeBytes[3])

		// Check for end marker
		if chunkSize == 0 {
			break
		}

		// Read encrypted chunk
		encryptedChunk := make([]byte, chunkSize)
		if _, err := io.ReadFull(reader, encryptedChunk); err != nil {
			return totalBytes, fmt.Errorf("failed to read encrypted chunk: %w", err)
		}

		// Decrypt chunk
		decrypted, err := sd.gcm.Open(nil, sd.nonce, encryptedChunk, nil)
		if err != nil {
			return totalBytes, fmt.Errorf("failed to decrypt chunk: %w", err)
		}

		// Write decrypted data
		if _, err := writer.Write(decrypted); err != nil {
			return totalBytes, fmt.Errorf("failed to write decrypted chunk: %w", err)
		}

		totalBytes += int64(len(decrypted))

		// Update nonce for next chunk
		incrementNonce(sd.nonce)
	}

	return totalBytes, nil
}

// incrementNonce increments the nonce by 1 (simple counter mode)
func incrementNonce(nonce []byte) {
	for i := len(nonce) - 1; i >= 0; i-- {
		nonce[i]++
		if nonce[i] != 0 {
			break
		}
	}
}

// Encrypt encrypts data using AES-256-GCM
func Encrypt(plain, key []byte) (nonce, ciphertext, tag []byte, err error) {
	if len(key) != 32 {
		return nil, nil, nil, errors.New("key must be 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, plain, nil)
	tagSize := gcm.Overhead()

	return nonce, sealed[:len(sealed)-tagSize], sealed[len(sealed)-tagSize:], nil
}

// Decrypt decrypts data using AES-256-GCM
func Decrypt(nonce, ciphertext, tag, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Reconstruct the sealed data
	sealed := append(ciphertext, tag...)

	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// DeriveEpochKey derives a key from master key and epoch
func DeriveEpochKey(masterKey []byte, epoch string) []byte {
	h := hmac.New(sha256.New, masterKey)
	h.Write([]byte(epoch))
	return h.Sum(nil)
}

// GenerateRandomKey generates a cryptographically secure random key
func GenerateRandomKey() ([]byte, error) {
	key := make([]byte, 32) // 256 bits for AES-256
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return key, nil
}

// EncryptedShard represents an encrypted shard with metadata
type EncryptedShard struct {
	Nonce      []byte
	Ciphertext []byte
	Tag        []byte
	ShardID    string
	Epoch      string
	Index      int
}

// EncryptShard encrypts a shard with epoch-based key derivation
func EncryptShard(data []byte, shardID, epoch string, index int, masterKey []byte) (*EncryptedShard, error) {
	epochKey := DeriveEpochKey(masterKey, epoch)
	nonce, ciphertext, tag, err := Encrypt(data, epochKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt shard: %w", err)
	}

	return &EncryptedShard{
		Nonce:      nonce,
		Ciphertext: ciphertext,
		Tag:        tag,
		ShardID:    shardID,
		Epoch:      epoch,
		Index:      index,
	}, nil
}

// DecryptShard decrypts a shard using epoch-based key derivation
func DecryptShard(shard *EncryptedShard, masterKey []byte) ([]byte, error) {
	epochKey := DeriveEpochKey(masterKey, shard.Epoch)
	return Decrypt(shard.Nonce, shard.Ciphertext, shard.Tag, epochKey)
}
