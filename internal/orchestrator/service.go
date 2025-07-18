package orchestrator

import (
	"context"

	"distro.lol/internal/orchestrator/orchestrator"
)

type Service interface {
	Start() error
	Stop() error
}

type service struct {
	grpcServer *grpcServer
	httpServer *httpServer
}

// New creates a new orchestrator service instance
func New(config *orchestrator.Config) *service {
	orchestrator := orchestrator.New(config)
	return &service{
		grpcServer: &grpcServer{orchestrator: orchestrator},
		httpServer: &httpServer{orchestrator: orchestrator},
	}
}

func (s *service) Start(ctx context.Context) error {
	errChan := make(chan error, 2)
	go s.grpcServer.start(ctx, errChan)
	go s.httpServer.start(ctx, errChan)
	err := <-errChan
	return err
}

func (s *service) Stop() error {
	return nil
}
