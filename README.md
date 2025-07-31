# Distro

A distributed storage system that uses Reed-Solomon encoding for data redundancy and fault tolerance. Distro encrypts, shards, and distributes files across multiple worker nodes, ensuring data availability even when some nodes are unavailable.

## Purpose

Distro is designed to provide:

- **Fault-tolerant storage**: Files are encoded using Reed-Solomon erasure coding, allowing recovery even when multiple storage nodes fail
- **Security**: All data is encrypted before sharding and distribution
- **Scalability**: Distributed architecture allows horizontal scaling of storage capacity
- **Efficiency**: Streaming operations minimize memory usage for large files

## Current Architecture

<img width="1286" height="1254" alt="image" src="https://github.com/user-attachments/assets/f668b050-cb7e-414e-b9be-f5de2784b279" />


## Requirements

- **Docker**: For containerized deployment
- **Make**: For build automation

## Building and Running

### Build

```bash
make build
```

### Run

```bash
make run
```

## Configuration

Create a `.env` file in the root directory with the following variables:

```env
SUPABASE_URL=your_supabase_url
SUPABASE_KEY=your_supabase_key
MASTER_KEY=your_32_byte_master_key_in_hex
GRPC_PORT=9090
```

## API Usage

### Upload a File

```bash
curl -X POST http://localhost:8080/upload -F "file=@./upload/payload.txt"
```

### Download a File

```bash
curl -X GET http://localhost:8080/download/<object_id> -o ./download/output.txt
```

Replace `<object_id>` with the ID returned from the upload operation.

## Roadmap

### Near-term Goals

- **Client-side encryption and sharding**: Move encryption and sharding operations to the client to reduce orchestrator load and improve security
- **Streaming operations**: Implement streaming encryption and sharding to handle large files without loading them entirely into memory
- **Streaming gRPC connections**: Use streaming gRPC connections between client and orchestrator for efficient shard transfer
- **Worker streaming**: Implement streaming gRPC connections from orchestrator to worker nodes for real-time shard distribution
- **Connection pooling**: Add connection pooling for worker connections in the orchestrator to improve performance
- **Dynamic sharding parameters**: Automatically calculate optimal Reed-Solomon parameters (n, k) based on file size and network conditions

### Longer-term Goals

- **Direct client-worker connections**: Enable workers to stream shards directly to clients over mTLS gRPC connections, bypassing the orchestrator during downloads for improved performance
- **Proximity-based worker selection**: Implement intelligent worker selection based on geographic proximity to clients
- **Rewards system**: Develop an incentive mechanism to reward worker nodes for storage and bandwidth contribution

## Contributing

[TBD]

## License

[TBD]
