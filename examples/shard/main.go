package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"distro.lol/internal/orchestrator/shard"
)

func main() {
	// Test parameters
	fileSize := 500 * 1024 * 1024 // 500 MB test file
	n := 20                      // Total shards
	k := 10                      // Threshold shards needed for reconstruction

	fmt.Printf("=== Shard Manager Performance Test ===\n")
	fmt.Printf("File size: %d MB\n", fileSize/(1024*1024))
	fmt.Printf("Total shards (n): %d\n", n)
	fmt.Printf("Threshold shards (k): %d\n", k)
	fmt.Println()

	// Generate a master key for encryption
	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		log.Fatal("Failed to generate master key:", err)
	}

	// Create shard manager
	ctx := context.Background()
	shardManager := shard.NewManager(ctx, masterKey)

	// Generate fake large file data
	fmt.Println("Generating fake file data...")
	start := time.Now()
	fakeData := make([]byte, fileSize)
	if _, err := rand.Read(fakeData); err != nil {
		log.Fatal("Failed to generate fake data:", err)
	}
	fmt.Printf("✓ Generated %d bytes in %v\n", len(fakeData), time.Since(start))
	fmt.Println()

	// Test shard creation and encryption
	fmt.Println("Creating and encrypting shards...")
	start = time.Now()
	encryptedShards, epoch, err := shardManager.CreateEncryptedShards(fakeData, n, k)
	if err != nil {
		log.Fatal("Failed to create encrypted shards:", err)
	}
	createDuration := time.Since(start)
	fmt.Printf("✓ Created %d encrypted shards in %v\n", len(encryptedShards), createDuration)

	// Calculate shard sizes
	totalShardSize := 0
	for _, shard := range encryptedShards {
		totalShardSize += len(shard)
	}
	fmt.Printf("✓ Total shard size: %d bytes (%.2f%% overhead)\n",
		totalShardSize, float64(totalShardSize-fileSize)/float64(fileSize)*100)
	fmt.Println()

	// Test reconstruction with all shards
	fmt.Println("Reconstructing from all shards...")
	start = time.Now()
	reconstructedData, err := shardManager.ReconstructEncryptedShards(encryptedShards, epoch, n, k)
	if err != nil {
		log.Fatal("Failed to reconstruct from all shards:", err)
	}
	reconstructAllDuration := time.Since(start)
	fmt.Printf("✓ Reconstructed %d bytes in %v\n", len(reconstructedData), reconstructAllDuration)

	// Verify data integrity
	if len(reconstructedData) != len(fakeData) {
		log.Fatal("Data length mismatch!")
	}
	dataMatch := true
	for i := 0; i < len(fakeData); i++ {
		if fakeData[i] != reconstructedData[i] {
			dataMatch = false
			break
		}
	}
	if !dataMatch {
		log.Fatal("Data integrity check failed!")
	}
	fmt.Println("✓ Data integrity verified")
	fmt.Println()

	// Test reconstruction with minimum threshold (k shards)
	fmt.Println("Reconstructing from minimum threshold shards...")
	minimalShards := make([][]byte, n)
	for i := 0; i < k; i++ {
		minimalShards[i] = encryptedShards[i]
	}

	start = time.Now()
	reconstructedMinimal, err := shardManager.ReconstructEncryptedShards(minimalShards, epoch, n, k)
	if err != nil {
		log.Fatal("Failed to reconstruct from minimal shards:", err)
	}
	reconstructMinDuration := time.Since(start)
	fmt.Printf("✓ Reconstructed %d bytes from %d shards in %v\n",
		len(reconstructedMinimal), k, reconstructMinDuration)

	// Verify minimal reconstruction
	if len(reconstructedMinimal) != len(fakeData) {
		log.Fatal("Minimal reconstruction length mismatch!")
	}
	dataMatch = true
	for i := 0; i < len(fakeData); i++ {
		if fakeData[i] != reconstructedMinimal[i] {
			dataMatch = false
			break
		}
	}
	if !dataMatch {
		log.Fatal("Minimal reconstruction integrity check failed!")
	}
	fmt.Println("✓ Minimal reconstruction integrity verified")
	fmt.Println()

	// Performance summary
	fmt.Println("=== Performance Summary ===")
	fmt.Printf("File size: %d MB\n", fileSize/(1024*1024))
	fmt.Printf("Shard creation + encryption: %v (%.2f MB/s)\n",
		createDuration, float64(fileSize)/(1024*1024)/createDuration.Seconds())
	fmt.Printf("Full reconstruction: %v (%.2f MB/s)\n",
		reconstructAllDuration, float64(fileSize)/(1024*1024)/reconstructAllDuration.Seconds())
	fmt.Printf("Minimal reconstruction: %v (%.2f MB/s)\n",
		reconstructMinDuration, float64(fileSize)/(1024*1024)/reconstructMinDuration.Seconds())
	fmt.Printf("Total time: %v\n", createDuration+reconstructAllDuration+reconstructMinDuration)
}
