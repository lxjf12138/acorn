# Sandbox Service Code Architecture

This document describes the intended internal code architecture for
`services/sandbox-service`.

The external contract stays in `api/proto/acorn/sandbox/v1`. This document is
about how the service should be structured internally so the current local
filesystem implementation remains an adapter, not the architecture itself.

---

## 1. Positioning

`sandbox-service` is a Capability Service that implements the
`acorn.sandbox` contract.

It should not mean one specific sandbox backend, one local directory layout, or
one execution strategy. Internally it should be able to support different
workspace stores, resource blob stores, execution backends, attachment models,
and sandbox profiles while keeping the external API stable.

Current external boundaries:

```text
Workspace View Surface
  ListWorkspaceDir
  PreviewWorkspaceFile
  No ResourceRef creation

Workspace Resource Surface
  ExportWorkspacePath
  Creates ResourceRef / ResourceRecord
  No download streaming yet
  No ImportResource yet
```

Core rule:

```text
View lets users look.
Export turns service-owned workspace content into ResourceRef metadata.
Download and Import are separate Resource flows.
```

---

## 2. Current Shape

The current directory structure is already close to the desired service
layering:

```text
services/sandbox-service/internal/
  conf/
  descriptor/
  domain/
  server/
  service/
  surface/
  version/
```

The main gap is that local filesystem behavior still lives in the service
layer:

```text
internal/service/workspace.go
  creates local workspace roots

internal/service/view.go
  reads directories and files
  resolves symlinks
  applies local filesystem path containment

internal/service/transfer.go
  stats and exports local files

internal/domain/path/resolve.go
  contains filesystem-specific path resolution
```

That is acceptable for the current foundation, but it should not be extended
into download, import, or exec. The next cleanup is to move local filesystem
behavior behind a `WorkspaceStore` adapter.

---

## 3. Target Internal Layers

The target shape is:

```text
services/sandbox-service/internal/
  service/
    gRPC service implementations
    request validation
    orchestration
    proto conversion

  domain/
    pure models and interfaces
    workspace metadata
    workspace store contract
    path syntax policy
    exported resource records
    future blob store, backend, attachment, profile contracts

  infra/
    concrete adapters
    localfs workspace store
    future localblob store
    future localprocess / docker / remoteagent backends

  descriptor/
    CapabilityDescriptor generation

  server/
    HTTP / gRPC wiring

  conf/
    service configuration
```

The service layer should depend on domain interfaces, not on `os`,
`filepath`, symlink handling, or a concrete local root path.

---

## 4. Domain Concepts

### 4.1 Hosted Workspace

A hosted workspace is sandbox-service-owned persistent workspace state.

It is not the same thing as a local directory. A local directory is only one
possible backing implementation.

Current model:

```go
type Workspace struct {
    ID               string
    SandboxProfileID string
    DisplayName      string
    Status           workspacev1.WorkspaceStatus
    RootPath         string
    CreatedAt        time.Time
    UpdatedAt        time.Time
    MetadataJSON     []byte
}
```

Target model:

```go
type Workspace struct {
    ID               string
    SandboxProfileID string
    DisplayName      string
    Status           workspacev1.WorkspaceStatus

    StoreKind        string
    StoreWorkspaceID string

    CreatedAt        time.Time
    UpdatedAt        time.Time
    MetadataJSON     []byte
}
```

`RootPath` may remain during migration, but service code should stop reading it.
Only the local filesystem adapter should know how a workspace maps to host
paths.

### 4.2 WorkspacePathRef

`WorkspacePathRef` is the external sandbox path reference:

```text
workspace: WorkspaceHostRef
path: workspace-relative POSIX path
kind: file | directory | symlink | unknown
```

It is not a `ResourceRef`. It cannot be used directly across services. Only the
owning sandbox-service can interpret it.

### 4.3 ResourceRef

`ResourceRef` is the cross-service content boundary.

```text
ExportWorkspacePath:
  WorkspacePathRef -> ResourceRef

Future ImportResource:
  ResourceRef -> WorkspacePathRef
```

Export currently records a resource-to-workspace mapping. Before download
streaming becomes durable, export should evolve into a snapshot flow backed by
a `ResourceBlobStore`.

---

## 5. WorkspaceStore

The next internal abstraction should be `domain/workspacestore`.

Its job is to hide how workspace backing content is created, stored, listed,
previewed, and exported. It is not the canonical HostedWorkspace metadata
store. Unless a later migration explicitly changes that ownership,
`domain/workspace.Store` remains the canonical metadata store for
sandbox-service-owned HostedWorkspace records.

In other words:

```text
domain/workspace.Store
  owns HostedWorkspace metadata
  stores service workspace id, profile id, status, display name, timestamps

domain/workspacestore.Store
  owns backing content operations
  creates backing storage
  lists / previews / stats / exports workspace paths
```

The `WorkspaceHostService.CreateHostedWorkspace` flow should stay explicit:

```text
1. Resolve sandbox profile.
2. Generate service_workspace_id.
3. workspacestore.CreateBackingWorkspace creates backing storage.
4. domain/workspace.Store saves canonical HostedWorkspace metadata.
5. Return HostedWorkspace proto.
```

Draft interface:

```go
type Store interface {
    CreateBackingWorkspace(ctx context.Context, req CreateBackingWorkspaceRequest) (*BackingWorkspace, error)
    DeleteBackingWorkspace(ctx context.Context, workspaceID string) error

    ListDir(ctx context.Context, req ListDirRequest) (*DirListing, error)
    PreviewFile(ctx context.Context, req PreviewFileRequest) (*FilePreview, error)
    StatPath(ctx context.Context, req StatPathRequest) (*PathInfo, error)
    ExportPath(ctx context.Context, req ExportPathRequest) (*ExportedPath, error)
}
```

Important types:

```go
type CreateBackingWorkspaceRequest struct {
    WorkspaceID      string
    SandboxProfileID string
    DisplayName      string
    MetadataJSON     []byte
}

type BackingWorkspace struct {
    WorkspaceID      string
    StoreKind        string
    StoreWorkspaceID string
    Metadata         map[string]string
}

type PathInfo struct {
    Path       string
    Name       string
    Kind       sandboxv1.WorkspacePathKind
    SizeBytes  int64
    ModifiedAt time.Time
}

type ExportedPath struct {
    Source    PathInfo
    MimeType  string
    SizeBytes int64
    Open      func(ctx context.Context) (io.ReadCloser, error)
}
```

`ExportedPath` should not return `ResourceRef`. The workspace store owns
workspace content. The transfer service owns ResourceRef creation and resource
registration orchestration.

The first PR should not add a second `Get` path for HostedWorkspace metadata.
Service code should read canonical workspace records from `domain/workspace`
and pass the workspace id to the backing content store.

---

## 6. LocalFS WorkspaceStore

The first adapter should be:

```text
internal/infra/localfs/
  workspace_store.go
  path_resolver.go
  workspace_store_test.go
```

Responsibilities:

```text
Create workspace root
Normalize workspace-relative POSIX paths
Resolve local filesystem paths
Prevent traversal
Prevent parent symlink escape
Apply symlink policy
List directories
Preview files
Stat paths
Open export sources
```

LocalFS-specific logic belongs here:

```text
filepath.Join
filepath.EvalSymlinks
os.ReadDir
os.Open
os.Lstat
root containment checks
leaf symlink rejection
```

The service layer should not contain these details.

---

## 7. Path Policy

Path handling has two layers.

### Syntax Policy

This is shared and belongs in `internal/domain/path`.

Rules:

```text
workspace-relative POSIX paths
no absolute paths
no ..
no NUL
no backslash
no Windows drive paths
empty path only when allowRoot=true
```

### Storage Resolver

This is adapter-specific.

For LocalFS, resolver rules include:

```text
EvalSymlinks
resolved path must remain under resolved workspace root
directory view does not enter symlink directories
preview/export do not follow symlinks
```

Object-backed and remote-agent stores may not have host symlink semantics, so
that logic should not live in service code.

---

## 8. ResourceBlobStore

This is not part of the WorkspaceStore abstraction PR, but it is the next
important boundary.

Reason:

```text
Workspace files are mutable live state.
ResourceRef should represent stable content.
```

Future interface:

```go
type Store interface {
    Put(ctx context.Context, req PutRequest) (*StoredBlob, error)
    Open(ctx context.Context, resourceID string) (io.ReadCloser, *BlobInfo, error)
    Stat(ctx context.Context, resourceID string) (*BlobInfo, error)
    Delete(ctx context.Context, resourceID string) error
}
```

Future export flow:

```text
WorkspaceStore.ExportPath
  -> open source bytes
  -> ResourceBlobStore.Put
  -> ResourceRef
  -> Control Plane RegisterResource
```

Until this exists, exported resources are metadata foundations, not durable
downloadable snapshots.

The Resource Download Gateway must not treat the current
`resource_id -> workspace path` mapping as the durable behavior. Before public
download is enabled, export should become snapshot-backed through
`ResourceBlobStore`.

---

## 9. Future Execution Boundaries

Exec should not directly assume local filesystem paths either.

Future concepts:

```text
WorkspaceAttachment
WorkspaceMounter
SandboxBackend
SandboxLease
ProfileRegistry
```

The important relationship:

```text
Profile = backend + workspace store + policy/capabilities
Backend = execution environment
WorkspaceStore = workspace content authority
Attachment = how a workspace is made available to a backend
```

Examples:

```text
local-process:
  WorkspaceStore = localfs
  Backend = localprocess
  Attachment = local path

docker:
  WorkspaceStore = localfs or docker volume
  Backend = docker
  Attachment = bind mount or volume

cloud-vm:
  WorkspaceStore = remote agent or cloud disk
  Backend = cloud VM
  Attachment = remote workspace id / guest path
```

Do not introduce these packages before there is code that uses them. Keep the
next PR focused on WorkspaceStore.

---

## 10. Recommended Migration Plan

### PR 1: Introduce WorkspaceStore Abstraction

Goal:

```text
Move local filesystem assumptions out of service/view.go, service/transfer.go,
and service/workspace.go.
```

Scope:

```text
Add domain/workspacestore
Add infra/localfs
WorkspaceHostService.CreateHostedWorkspace uses WorkspaceStore.CreateBackingWorkspace
WorkspaceViewService uses WorkspaceStore.ListDir / PreviewFile
WorkspaceTransferService uses WorkspaceStore.ExportPath
Keep external proto unchanged
Keep behavior unchanged
```

Acceptance:

```text
service/view.go does not call os.ReadDir or os.Open
service/transfer.go does not call os.Lstat or filepath helpers
service/workspace.go does not create local directories itself
LocalFS tests cover traversal, symlink escape, symlink rejection, pagination,
preview limits, and empty root
View Surface behavior unchanged
ExportWorkspacePath behavior unchanged
No new external proto fields or service methods
ListWorkspaceDir / PreviewWorkspaceFile still do not create ResourceRef
ExportWorkspacePath only returns ResourceRef from sandbox-service
Only the Control Plane registers ResourceRecord
```

### PR 2: ResourceBlobStore and Snapshot Export

Goal:

```text
Make exported ResourceRefs stable content snapshots.
```

Scope:

```text
Add domain/resourceblob
Add infra/localblob
ExportWorkspacePath copies bytes into ResourceBlobStore
exportedresource record points to blob metadata, not live workspace path
```

### PR 3: Resource Download Gateway

Goal:

```text
GET /resources/{resource_id}/download
```

Control Plane flow:

```text
Get ResourceRecord
Find authority_service_id
Call authority service content API
Stream bytes to user
```

### PR 4: ImportResource

Goal:

```text
ResourceRef -> WorkspacePathRef
```

This should wait until ResourceRef bytes can be read through a content API or
download gateway.

### PR 5: SandboxBackend and WorkspaceAttachment

Goal:

```text
Execute inside a sandbox without coupling exec to local directories.
```

---

## 11. Non-Goals For The Next PR

Do not include these in the WorkspaceStore abstraction PR:

```text
Download streaming
ImportResource
ResourceBlobStore
Snapshot export
SandboxBackend
Docker / VM / remote agent
Profile registry config rewrite
Large config schema migration
```

The next PR is only an internal boundary cleanup. It should not change the
external sandbox API or user-visible behavior.
