package logic

import (
	"context"
)

func (o *Orchestrator) RegisterWorker(ctx context.Context, workerID, workerEndpoint string, capacity, usedSpace int64) error {
	// Implement the logic to register a worker
	// This could involve checking the worker's capacity and used space
	// and storing it in a database or in-memory structure

	return o.workerManager.RegisterWorker(ctx, workerID, workerEndpoint, capacity, usedSpace)
}
