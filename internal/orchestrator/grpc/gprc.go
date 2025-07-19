package grpc

import (
	pb "distro.lol/pkg/rpc/orchestrator"
)

type OrchestratorGrpcServer struct {
	pb.UnimplementedOrchestratorServer
}

