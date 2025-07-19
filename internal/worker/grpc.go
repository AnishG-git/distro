package worker

import (
	"context"

	pb "distro.lol/pkg/rpc/worker"
)

func (w *worker) Ping(ctx context.Context, req *pb.PingRequest) (*pb.StorageStats, error) {
	// Implement the logic to handle ping requests
	// This could involve checking the worker's status and returning its stats
	return &pb.StorageStats{
		TotalCapacity: w.capacity,
		UsedCapacity:  w.usedSpace,
	}, nil
}

func (w *worker) StoreShard(ctx context.Context, envelope *pb.ShardEnvelope) (*pb.StorageStats, error) {
	// Implement the logic to store a shard
	return &pb.StorageStats{}, nil
}

func (w *worker) FetchShard(req *pb.ShardRequest) (*pb.ShardEnvelope, error) {
	// Implement the logic to fetch a shard
	return &pb.ShardEnvelope{}, nil
}
