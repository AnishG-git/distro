package orchestrator

import (
	"context"
	"fmt"
	"log"
	"net"

	"distro.lol/internal/orchestrator/orchestrator"
	pb "distro.lol/pkg/rpc/orchestrator"
	"google.golang.org/grpc"
)

type grpcServer struct {
	pb.UnimplementedOrchestratorServiceServer
	orchestrator *orchestrator.Orchestrator
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
	pb.RegisterOrchestratorServiceServer(server, s)

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
