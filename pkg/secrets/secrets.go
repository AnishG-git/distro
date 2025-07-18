package secrets

import (
	"crypto/rand"
	"errors"
	"fmt"

	"go.bryk.io/pkg/crypto/shamir"
)

// Package secrets provides Shamir's Secret Sharing functionality
// Uses proper cryptographically secure Shamir's Secret Sharing algorithm

// Split splits a secret into n shares with t threshold
// Uses proper Shamir's Secret Sharing algorithm
func Split(data []byte, n, t int) ([][]byte, error) {
	if t <= 0 || n <= 0 {
		return nil, errors.New("n and t must be positive")
	}
	if t > n {
		return nil, errors.New("threshold t cannot be greater than n")
	}
	if len(data) == 0 {
		return nil, errors.New("data cannot be empty")
	}
	if n > 255 {
		return nil, errors.New("n must be less than 256")
	}
	if t < 2 {
		return nil, errors.New("threshold must be at least 2")
	}

	// Use proper Shamir's Secret Sharing
	return shamir.Split(data, n, t)
}

// Combine reconstructs secret from t or more shares
// Uses proper Shamir's Secret Sharing algorithm
func Combine(shares [][]byte) ([]byte, error) {
	if len(shares) == 0 {
		return nil, errors.New("no shares provided")
	}

	// Use proper Shamir's Secret Sharing
	return shamir.Combine(shares)
}

// GenerateRandomShares creates random data for testing
func GenerateRandomShares(shareCount, shareSize int) ([][]byte, error) {
	shares := make([][]byte, shareCount)
	for i := range shares {
		shares[i] = make([]byte, shareSize)
		if _, err := rand.Read(shares[i]); err != nil {
			return nil, fmt.Errorf("failed to generate random share %d: %w", i, err)
		}
	}
	return shares, nil
}
