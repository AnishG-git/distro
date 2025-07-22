package worker

import (
	"time"

	"google.golang.org/grpc"
)

type Worker struct {
	WorkerID       string           `json:"worker_id"`
	WorkerEndpoint string           `json:"worker_endpoint"`
	TotalCapacity  int64            `json:"total_capacity"`
	UsedCapacity   int64            `json:"used_capacity"`
	Status         Status           `json:"status"`
	LastHeartbeat  time.Time        `json:"last_heartbeat"`
	Conn           *grpc.ClientConn `json:"-"` // gRPC connection to the worker (not stored in DB)
}

type Status string

const (
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
)

// StatusFromString converts a string to Status type
func StatusFromString(s string) Status {
	switch s {
	case "online":
		return StatusOnline
	case "offline":
		return StatusOffline
	default:
		return StatusOffline // Default to offline for unknown statuses
	}
}
