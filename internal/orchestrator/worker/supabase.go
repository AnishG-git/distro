package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/supabase-community/supabase-go"
)

// function to sync worker data with Supabase periodically
func (wm *workerManager) startSupabaseSyncLoop() {
	ticker := time.NewTicker(wm.workerSyncInterval) // Adjust the interval as needed
	defer ticker.Stop()
	for {
		select {
		case <-wm.ctx.Done():
			return // Exit if context is done
		case <-ticker.C:
			if err := wm.syncWorkersWithSupabase(); err != nil {
				log.Printf("failed to sync local workers map with Supabase: %v", err)
			}
		}
	}
}

func (wm *workerManager) syncWorkersWithSupabase() error {
	wm.mu.Lock()

	// Convert workers map to slice for Supabase
	var workers []Worker
	for _, worker := range wm.workers {
		workers = append(workers, *worker)
	}

	wm.mu.Unlock()

	// Upsert workers into Supabase
	if _, _, err := wm.sbClient.From("workers").Upsert(workers, "", "", "").Execute(); err != nil {
		return fmt.Errorf("failed to upsert workers: %w", err)
	}

	log.Println("Successfully synced workers with Supabase")
	return nil
}

func newSupabaseClient() (*supabase.Client, error) {
	// Initialize Supabase client
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_KEY")

	if url == "" || key == "" {
		return nil, fmt.Errorf("supabase URL or key not set in environment variables")
	}

	client, err := supabase.NewClient(url, key, &supabase.ClientOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Supabase client: %w", err)
	}
	return client, nil
}

func (wm *workerManager) getOnlineWorkersFromSB() ([]Worker, error) {
	if wm.sbClient == nil {
		return nil, fmt.Errorf("supabase client is not initialized")
	}

	var workers []Worker
	data, _, err := wm.sbClient.From("workers").
		Select("worker_id,address,grpc_port,total_capacity,used_capacity,status,last_heartbeat", "", false).
		Eq("status", string(StatusOnline)).
		Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workers from database: %w", err)
	}

	// Unmarshal the data into our workers slice
	if err := json.Unmarshal(data, &workers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal worker data: %w", err)
	}

	return workers, nil
}

func (wm *workerManager) upsertWorkerToSB(worker Worker) error {
	if wm.sbClient == nil {
		return fmt.Errorf("supabase client is not initialized")
	}

	// Upsert the worker into Supabase
	if _, _, err := wm.sbClient.From("workers").Upsert(worker, "", "", "").Execute(); err != nil {
		return fmt.Errorf("failed to upsert worker %s: %w", worker.WorkerID, err)
	}

	log.Printf("Successfully upserted worker %s to Supabase", worker.WorkerID)
	return nil
}
