# Persistence Boundaries

Acorn separates metadata records, byte blobs, live workspaces, and coordination
state.

Object storage is appropriate for blobs. It is not the default metadata or
runtime-state store.

## Store Categories

### RecordStore

RecordStore stores metadata and Control Plane runtime state.

Examples:

```text
WorkspaceRecordStore
ResourceRecordStore
ExecutionRecordStore
future RunRecordStore
future SandboxPolicyStore
```

Good backend families:

```text
in-memory
SQLite
Postgres
KV store
DynamoDB-like store
```

RecordStore usually needs:

```text
create
get
update
list by filter
pagination
status transition
uniqueness
timestamps
optional compare-and-set
```

Object storage is not the preferred primary backend for RecordStore because
RecordStore requires query, indexing, pagination, update, and state transition
semantics.

### BlobStore

BlobStore stores byte objects.

Examples:

```text
ResourceBlobStore
future ExecutionOutputBlobStore
future WorkspaceBundleStore
```

Good backend families:

```text
local filesystem
S3
GCS
Azure Blob
MinIO
```

BlobStore operations:

```text
put
open
stat
delete
optional content hash
optional immutable object id
```

Object storage is a good BlobStore backend.

### WorkspaceStore

WorkspaceStore stores live workspace contents owned by a capability service.

Examples:

```text
LocalFSWorkspaceStore
future GitWorktreeWorkspaceStore
future RemoteWorkspaceStore
future VM/container-backed workspace store
```

WorkspaceStore is not ResourceBlobStore. Live workspace state is mutable
directory/tree state. Object storage may hold snapshots or bundles, but it is
not necessarily the live workspace.

### LeaseManager

LeaseManager coordinates concurrent workspace operations.

Current implementation:

```text
in-process read/write workspace lease
```

Future implementations:

```text
database-backed lease
Redis/etcd lease
workspace-owner routing
```

LeaseManager is coordination state. It is not runtime history. It is not a
RecordStore.

## Boundary Table

| Concept | Owns | Example | Good backend | Not for |
| --- | --- | --- | --- | --- |
| RecordStore | metadata/state | ExecutionRecord | SQLite/Postgres/KV | large bytes |
| BlobStore | byte objects | Resource content | S3/GCS/MinIO/local | querying runtime state |
| WorkspaceStore | live workspace tree | sandbox workspace | localfs/git worktree/remote | global shared FS |
| LeaseManager | concurrency coordination | workspace read/write lease | memory/DB/Redis | audit/history |

## Practical Rules

Use RecordStore for objects that need filtering, pagination, update semantics,
and state transitions.

Use BlobStore for bytes that can be addressed, opened, hashed, and deleted
without needing relational queries.

Use WorkspaceStore for mutable workspace trees that capability services own and
operate on directly.

Use LeaseManager for short-lived coordination around concurrent operations. Do
not use it as audit history or runtime state.

