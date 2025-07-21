package worker

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	pbo "distro.lol/pkg/rpc/orchestrator"
	pb "distro.lol/pkg/rpc/worker"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type worker struct {
	id                   string
	workerEndpoint       string
	orchestratorEndpoint string // Endpoint of the orchestrator
	capacity             int64  // Total capacity of the worker
	usedSpace            int64  // Used space in the worker
	pb.UnimplementedWorkerServer
}

func New(workerEndpoint string, capacity int64) *worker {
	// Initialize a new worker with the given target and capacity
	// TEST - for now we will use an environment variable for the worker ID
	// In the real service, the worker ID would be the worker's email address after they sign up or some byproduct
	// id := os.Getenv("WORKER_ID")
	// orchestratorEndpoint := os.Getenv("ORCHESTRATOR_ENDPOINT")
	// if id == "" {
	// 	log.Fatal("WORKER_ID environment variable is not set")
	// }
	return &worker{
		id:                   uuid.NewString(),
		workerEndpoint:       workerEndpoint,
		capacity:             capacity,
		orchestratorEndpoint: "127.0.0.1:9090",
	}
}

func (w *worker) Start() error {
	if w.capacity <= w.usedSpace {
		return fmt.Errorf("worker capacity exceeded: total capacity %d, used space %d", w.capacity, w.usedSpace)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ping orchestrator to check if it's ready
	conn, err := grpc.NewClient(w.orchestratorEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to orchestrator: %v", err)
	}

	now := time.Now()

	cli := pbo.NewOrchestratorClient(conn)
	resp, err := cli.Ping(ctx, &pbo.PingRequest{
		Message: "ping",
	})

	if err != nil {
		return fmt.Errorf("failed to ping orchestrator: %v", err)
	}

	if resp.Success {
		log.Printf("Orchestrator is ready at %s", w.orchestratorEndpoint)
	}
	timeElapsed := time.Since(now)
	log.Printf("Ping to orchestrator took %s", timeElapsed)
	// end of ping orchestrator

	go func() {
		time.Sleep(1 * time.Second) // Simulate some startup delay
		for {
			if err := w.checkReadiness(); err != nil {
				log.Printf("Server is not ready: %v", err)
				continue
			}
			break
		}
		// Wait for the server to be ready
		log.Printf("Server is ready at %s", w.workerEndpoint)

		// Register the worker with the orchestrator
		if err := w.register(ctx); err != nil {
			log.Printf("Failed to register worker: %v", err)
			cancel()
		}
	}()

	if err := w.startServer(ctx); err != nil {
		return err
	}

	return nil
}

func (w *worker) startServer(ctx context.Context) error {
	listener, err := net.Listen("tcp", w.workerEndpoint)
	if err != nil {
		log.Printf("Failed to listen at %s: %v", w.workerEndpoint, err)
		return err
	}

	server := grpc.NewServer()

	// Register orchestrator grpc service
	pb.RegisterWorkerServer(server, w)

	errChan := make(chan error, 1)

	go func() {
		log.Printf("Starting gRPC server at %s", w.workerEndpoint)
		errChan <- fmt.Errorf("gRPC server failed: %v", server.Serve(listener))
	}()

	// Wait for context cancellation and gracefully stop the server
	go func() {
		<-ctx.Done()
		log.Printf("Context cancelled, stopping gRPC server")
		server.GracefulStop()
	}()

	return <-errChan
}

func (w *worker) checkReadiness() error {
	conn, err := grpc.NewClient(w.workerEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to worker server: %v", err)
	}
	defer conn.Close()

	client := pb.NewWorkerClient(conn)
	_, err = client.Ping(context.Background(), &pb.PingRequest{
		Ping: "ping",
	})
	if err != nil {
		return fmt.Errorf("ping failed: %v", err)
	}

	return nil
}

func (w *worker) register(ctx context.Context) error {
	conn, err := grpc.NewClient(w.orchestratorEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to orchestrator: %v", err)
	}
	defer conn.Close()

	client := pbo.NewOrchestratorClient(conn)

	// Register the worker with the orchestrator
	resp, err := client.Register(ctx, &pbo.RegisterRequest{
		WorkerId:       w.id,
		WorkerEndpoint: w.workerEndpoint,
		TotalCapacity:  w.capacity,
		UsedSpace:      w.usedSpace,
	})

	if err != nil {
		return err
	}

	if resp.Success {
		return nil
	}

	return fmt.Errorf("worker registration failed")
}
