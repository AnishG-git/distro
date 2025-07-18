package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

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
