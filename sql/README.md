# Database Schema

This directory contains the SQL schema files for the distributed storage system.

## Tables

### objects

Stores metadata about distributed files including:

- `id`: Unique identifier for the object
- `filename`: Original filename
- `size`: File size in bytes
- `checksum`: File integrity checksum
- `metadata`: Additional metadata as JSON
- `created_at`/`updated_at`: Timestamps

### shards

Stores information about Reed-Solomon encoded shards including:

- `id`: Unique shard identifier
- `object_id`: Reference to the parent object
- `worker_id`: Reference to the worker storing this shard
- `shard_index`: Index of this shard in the Reed-Solomon encoding
- `size`: Shard size in bytes
- `checksum`: Shard integrity checksum
- `metadata`: Additional metadata as JSON
- `created_at`/`updated_at`: Timestamps

## Setup

### Option 1: Full Setup

Run the complete initialization script:

```sql
\i sql/schema/init.sql
```

### Option 2: Individual Tables

Run each table creation script individually:

```sql
\i sql/schema/01_objects.sql
\i sql/schema/02_shards.sql
```

## Notes

- The shards table references both objects and workers tables with CASCADE deletes
- Automatic timestamp updates are handled by triggers
- Indexes are optimized for common query patterns:
  - Object lookups by filename
  - Shard lookups by object_id and worker_id
  - Unique constraint prevents duplicate shards per object index
