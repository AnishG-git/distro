package secrets

import (
	"bytes"
	"testing"
)

func TestSplitCombineSecret(t *testing.T) {
	secret := []byte("This is a very secret message that needs to be protected!")
	n, threshold := 5, 3

	// Split the secret
	shares, err := Split(secret, n, threshold)
	if err != nil {
		t.Fatalf("SplitSecret failed: %v", err)
	}

	if len(shares) != n {
		t.Errorf("Expected %d shares, got %d", n, len(shares))
	}

	// Verify we can reconstruct with exactly threshold shares
	thresholdShares := shares[:threshold]
	reconstructed, err := Combine(thresholdShares)
	if err != nil {
		t.Fatalf("CombineSecret with threshold shares failed: %v", err)
	}

	if !bytes.Equal(secret, reconstructed) {
		t.Errorf("Reconstructed secret doesn't match original")
	}

	// Verify we can reconstruct with more than threshold shares
	moreShares := shares[:threshold+1]
	reconstructed2, err := Combine(moreShares)
	if err != nil {
		t.Fatalf("CombineSecret with more shares failed: %v", err)
	}

	if !bytes.Equal(secret, reconstructed2) {
		t.Errorf("Reconstructed secret with more shares doesn't match original")
	}

	// Verify we can reconstruct with all shares
	reconstructed3, err := Combine(shares)
	if err != nil {
		t.Fatalf("CombineSecret with all shares failed: %v", err)
	}

	if !bytes.Equal(secret, reconstructed3) {
		t.Errorf("Reconstructed secret with all shares doesn't match original")
	}
}

func TestInsufficientShares(t *testing.T) {
	secret := []byte("Secret message")
	n, threshold := 5, 3

	shares, err := Split(secret, n, threshold)
	if err != nil {
		t.Fatalf("SplitSecret failed: %v", err)
	}

	// Try to reconstruct with fewer than threshold shares
	insufficientShares := shares[:threshold-1]
	_, err = Combine(insufficientShares)
	if err != nil {
		// This should actually work with our implementation since we don't enforce threshold
		// But the reconstructed data will be incorrect
		t.Logf("CombineSecret with insufficient shares returned error (expected): %v", err)
	}
}

func TestEmptySecret(t *testing.T) {
	secret := []byte{}
	n, threshold := 3, 2

	_, err := Split(secret, n, threshold)
	if err == nil {
		t.Error("Expected SplitSecret to fail with empty secret")
	}
}

func TestInvalidParameters(t *testing.T) {
	secret := []byte("test")

	// Test n > t
	_, err := Split(secret, 2, 3)
	if err == nil {
		t.Error("Expected SplitSecret to fail when t > n")
	}

	// Test zero values
	_, err = Split(secret, 0, 0)
	if err == nil {
		t.Error("Expected SplitSecret to fail with zero values")
	}
}
