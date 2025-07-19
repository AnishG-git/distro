package main

import (
	"log"

	"distro.lol/internal/worker"
)

func main() {
	// expected usage of the Worker struct
	worker := worker.New("localhost:9090", 10)

	errChan := make(chan error, 1)
	go func() {
		if err := worker.Start(); err != nil {
			errChan <- err
		}
	}()

	if err := <-errChan; err != nil {
		log.Fatalf("Worker encountered an error: %v", err)
	}
}
