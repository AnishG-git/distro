package orchestrator

import (
	"context"
	"fmt"
	"log"
	"net"

	"distro.lol/internal/orchestrator/logic"
	pb "distro.lol/pkg/rpc/orchestrator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcServer struct {
	pb.UnimplementedOrchestratorServer
	orchestrator *logic.Orchestrator
}

func (s *grpcServer) start(ctx context.Context, errChan chan error) {
	// Start the gRPC server
	config := s.orchestrator.GetConfig()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.GRPCPort))
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port %d: %v", config.GRPCPort, err)
	}

	server := grpc.NewServer()

	// Register orchestrator grpc service
	pb.RegisterOrchestratorServer(server, s)

	go func() {
		log.Printf("Starting gRPC server on port %d", config.GRPCPort)
		errChan <- fmt.Errorf("gRPC server failed: %v", server.Serve(listener))
	}()

	// Wait for context cancellation and gracefully stop the server
	go func() {
		<-ctx.Done()
		log.Printf("Context cancelled, stopping gRPC server")
		server.GracefulStop()
	}()
}

func (s *grpcServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	log.Printf("Received registration request for worker %s", req.WorkerId)
	if ctx.Err() != nil {
		return nil, status.Errorf(codes.Canceled, "request canceled: %v", ctx.Err())
	}

	// Handle worker registration logic
	if err := s.orchestrator.RegisterWorker(ctx, req.WorkerId, req.WorkerEndpoint, req.TotalCapacity, req.UsedSpace); err != nil {
		log.Printf("Failed to register worker %s: %v", req.WorkerId, err)
		return &pb.RegisterResponse{Success: false}, status.Errorf(codes.Internal, "failed to register worker: %s", err)
	}

	log.Printf("Worker %s registered", req.WorkerId)
	return &pb.RegisterResponse{Success: true}, nil
}

func (s *grpcServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	log.Printf("Received ping request: %s", req.Message)
	if ctx.Err() != nil {
		return nil, status.Errorf(codes.Canceled, "request canceled: %v", ctx.Err())
	}

	// Respond to ping request
	return &pb.PingResponse{Success: true}, nil
}