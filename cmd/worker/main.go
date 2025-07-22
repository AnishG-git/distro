package main

import (
	"log"
	"os"
	"strconv"

	"distro.lol/internal/worker"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	// expected usage of the Worker struct
	if err := godotenv.Load(); err != nil {
		log.Printf("Error loading .env file, using environment variables: %v", err)
	}

	workerID := os.Getenv("WORKER_ID")
	workerUUID, err := uuid.Parse(workerID)
	if err != nil {
		log.Fatalf("Invalid WORKER_ID environment variable: %v", err)
		return
	}

	workerEndpoint := os.Getenv("WORKER_ENDPOINT")
	_, err = strconv.Atoi(workerEndpoint)
	if err != nil {
		log.Fatalf("Invalid WORKER_ENDPOINT environment variable, must be a valid port number: %v", err)
	}

	orchestratorEndpoint := os.Getenv("ORCHESTRATOR_ENDPOINT")
	if orchestratorEndpoint == "" {
		log.Fatalf("ORCHESTRATOR_ENDPOINT environment variable is required")
	}

	capacityStr := os.Getenv("WORKER_CAPACITY")
	capacity, err := strconv.ParseInt(capacityStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid WORKER_CAPACITY environment variable, must be an integer: %v", err)
	}

	worker := worker.New(workerUUID, workerEndpoint, orchestratorEndpoint, capacity)

	if err := worker.Start(); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}
}
