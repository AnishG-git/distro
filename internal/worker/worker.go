package worker

import (
	pb "distro.lol/pkg/rpc/worker"
)

type worker struct {
	id       string
	target   string // The target address of the worker
	capacity int64   // Total capacity of the worker
	usedSpace int64 // Used space in the worker
	pb.UnimplementedWorkerServer
}

func New(target string, capacity int64) *worker {
	// Initialize a new worker with the given target and capacity
	return &worker{
		target:   target,
		capacity: capacity,
	}
}

func (w *worker) Start() error {
	// check if used space is less than capacity
	// Here you would implement the logic to start the worker
	// For example, connecting to a gRPC server or starting a service
	return nil
}
