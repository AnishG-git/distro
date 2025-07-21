package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"distro.lol/internal/orchestrator/shard"
)

func main() {
	fmt.Println("=== Shard Manager Performance Test ===")
	fmt.Println()

	// Get user preferences
	fileSize := getUserInput("Enter file size in MB (default: 100): ", 100) * 1024 * 1024
	n := getUserInput("Enter total shards (n) (default: 10): ", 10)
	k := getUserInput("Enter threshold shards (k) (default: 6): ", 6)

	// Validate k < n
	if k >= n {
		log.Fatal("Threshold shards (k) must be less than total shards (n)")
	}

	fmt.Printf("\nTest Configuration:\n")
	fmt.Printf("File size: %d MB\n", fileSize/(1024*1024))
	fmt.Printf("Total shards (n): %d\n", n)
	fmt.Printf("Threshold shards (k): %d\n", k)
	fmt.Printf("Method: In-Memory (Byte Slices)\n")
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
	printMemUsage("Before generating data")
	fakeData := make([]byte, fileSize)
	if _, err := rand.Read(fakeData); err != nil {
		log.Fatal("Failed to generate fake data:", err)
	}
	fmt.Printf("✓ Generated %d bytes in %v\n", len(fakeData), time.Since(start))
	printMemUsage("After generating data")
	fmt.Println()

	// Test shard creation (in-memory only)
	fmt.Println("Creating and encrypting shards (in-memory)...")
	printMemUsage("Before in-memory shard creation")
	start = time.Now()
	encryptedShards, epoch, err := shardManager.CreateEncryptedShards(fakeData, n, k)
	if err != nil {
		log.Fatal("Failed to create encrypted shards:", err)
	}
	createDuration := time.Since(start)
	fmt.Printf("✓ Created %d encrypted shards using in-memory method in %v\n", len(encryptedShards), createDuration)
	printMemUsage("After in-memory shard creation")

	// Calculate shard sizes
	totalShardSize := 0
	for _, shard := range encryptedShards {
		totalShardSize += len(shard)
	}
	fmt.Printf("✓ Total shard size: %d bytes (%.2f%% overhead)\n",
		totalShardSize, float64(totalShardSize-fileSize)/float64(fileSize)*100)
	fmt.Println()

	// Test reconstruction with all shards (in-memory only)
	fmt.Println("Reconstructing from all shards (in-memory)...")
	start = time.Now()
	reconstructedData, err := shardManager.ReconstructEncryptedShards(encryptedShards, epoch, n, k)
	if err != nil {
		log.Fatal("Failed to reconstruct from all shards:", err)
	}
	reconstructAllDuration := time.Since(start)
	fmt.Printf("✓ Reconstructed %d bytes using in-memory method in %v\n", len(reconstructedData), reconstructAllDuration)

	// Verify data integrity
	if !verifyDataIntegrity(fakeData, reconstructedData) {
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
	fmt.Printf("✓ Reconstructed %d bytes from %d shards using in-memory method in %v\n",
		len(reconstructedMinimal), k, reconstructMinDuration)

	// Verify minimal reconstruction
	if !verifyDataIntegrity(fakeData, reconstructedMinimal) {
		log.Fatal("Minimal reconstruction integrity check failed!")
	}
	fmt.Println("✓ Minimal reconstruction integrity verified")
	fmt.Println()

	// Performance summary
	fmt.Println("=== Performance Summary ===")
	fmt.Printf("Method: In-Memory (Byte Slices)\n")
	fmt.Printf("File size: %d MB\n", fileSize/(1024*1024))
	fmt.Printf("Shard creation + encryption: %v (%.2f MB/s)\n",
		createDuration, float64(fileSize)/(1024*1024)/createDuration.Seconds())
	fmt.Printf("Full reconstruction: %v (%.2f MB/s)\n",
		reconstructAllDuration, float64(fileSize)/(1024*1024)/reconstructAllDuration.Seconds())
	fmt.Printf("Minimal reconstruction: %v (%.2f MB/s)\n",
		reconstructMinDuration, float64(fileSize)/(1024*1024)/reconstructMinDuration.Seconds())

	totalTime := createDuration + reconstructAllDuration + reconstructMinDuration
	fmt.Printf("Total time: %v\n", totalTime)
	fmt.Printf("Overall throughput: %.2f MB/s\n",
		float64(fileSize*3)/(1024*1024)/totalTime.Seconds()) // *3 because we process the data 3 times
}

// getUserInput prompts the user for an integer input with a default value
func getUserInput(prompt string, defaultValue int) int {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("Error reading input:", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(input)
	if err != nil {
		fmt.Printf("Invalid input, using default value: %d\n", defaultValue)
		return defaultValue
	}

	return value
}

// verifyDataIntegrity checks if two byte slices are identical
func verifyDataIntegrity(original, reconstructed []byte) bool {
	if len(original) != len(reconstructed) {
		return false
	}

	for i := 0; i < len(original); i++ {
		if original[i] != reconstructed[i] {
			return false
		}
	}

	return true
}

// printMemUsage prints current memory usage
func printMemUsage(stage string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("[%s] Memory: %.2f MB allocated, %.2f MB total allocated, %.2f MB from system\n",
		stage,
		float64(m.Alloc)/(1024*1024),
		float64(m.TotalAlloc)/(1024*1024),
		float64(m.Sys)/(1024*1024))
}
