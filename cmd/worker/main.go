package main

import (
	"log"

	"distro.lol/internal/worker"
)

func main() {
	// expected usage of the Worker struct
	worker := worker.New("127.0.0.1:8081", 10)

	if err := worker.Start(); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}
}
