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
  Download streams snapshot ResourceRef bytes through Control Plane
  ImportResource consumes ResourceRef bytes through Control Plane broker
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

The previous local filesystem behavior has been moved behind the
`WorkspaceStore` abstraction.

```text
internal/domain/workspacestore/
  defines the workspace backing content contract

internal/infra/localfs/
  workspace_store.go
  path_resolver.go
  mime.go
```

The service layer now depends on `domain/workspacestore.Store` rather than
directly on `os`, `filepath`, symlink resolution, or concrete workspace root
paths.

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
    resource blob store contract
    future backend, attachment, profile contracts

  infra/
    concrete adapters
    localfs workspace store
    localblob resource blob store
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

The local filesystem root path is not part of hosted workspace metadata. Only
the local filesystem adapter knows how a workspace maps to host paths.

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

ImportResource:
  ResourceRef -> WorkspacePathRef
```

ExportWorkspacePath now creates a snapshot-backed `ResourceRef`. The exported
resource record points to blob metadata, while retaining the source workspace id
and path only for provenance and debugging.

ImportResource is brokered by the Control Plane: the Control Plane opens the
ResourceRef authority stream, forwards bytes into sandbox-service, and
sandbox-service writes them through `WorkspaceStore`. sandbox-service does not
directly access another service's private storage.

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

`ResourceBlobStore` is the internal content snapshot store for exported
resources.

Reason:

```text
Workspace files are mutable live state.
ResourceRef should represent stable content.
```

Implemented interface:

```go
type Store interface {
    Kind() string

    Put(ctx context.Context, req PutRequest) (*StoredBlob, error)
    Open(ctx context.Context, resourceID string) (io.ReadCloser, *BlobInfo, error)
    Stat(ctx context.Context, resourceID string) (*BlobInfo, error)
    Delete(ctx context.Context, resourceID string) error
}
```

Implemented export flow:

```text
WorkspaceStore.ExportPath
  -> open source bytes
  -> ResourceBlobStore.Put
  -> ResourceRef
  -> Control Plane RegisterResource
```

`ExportWorkspacePath` now creates snapshot-backed `ResourceRef` metadata. The
current local implementation stores blob metadata in memory and blob bytes on
local disk.

The Resource Download Gateway streams these snapshot blobs through the Control
Plane. Download does not read `WorkspacePathRef` or workspace live files.

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

Current internal foundation:

```text
domain/attachment
  WorkspaceAttachment
  WorkspaceMounter
  local_process target

infra/localfs
  prepares local_path attachments for local_process targets
  validates the resolved workspace root stays under the resolved localfs base

service/attachment
  resolves canonical HostedWorkspace metadata
  asks the mounter for a backend-specific attachment

domain/profile
  ProfileRegistry is the internal source of truth for enabled sandbox profiles
  local-process-dev is present only when sandbox.local_process.enabled = true
  profiles declare workspace capabilities, attachment kind, backend id, and isolation class
```

CapabilityDescriptor sandbox_profiles are generated from ProfileRegistry.
CreateHostedWorkspace accepts only enabled registry profiles. WorkspaceExecService
checks that the workspace profile supports workspace exec and is backed by the
configured backend before running a command.

---

## 10. Recommended Migration Plan

### PR 1: Introduce WorkspaceStore Abstraction

Goal:

```text
Move local filesystem assumptions out of service/view.go, service/transfer.go,
and service/workspace.go.
```

Status:

```text
Implemented.
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

Status:

```text
Implemented.
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

Status:

```text
Implemented.
```

### PR 4: ImportResource

Goal:

```text
ResourceRef -> WorkspacePathRef
```

Control Plane flow:

```text
Get ResourceRecord
Open authority ResourceContentService
Open sandbox ImportResourceToWorkspace stream
Pipe bytes into WorkspaceStore
Return WorkspacePathRef
```

Status:

```text
Implemented.
```

### PR 5: WorkspaceAttachment Foundation

Goal:

```text
Prepare backend-specific workspace attachments without adding exec.
```

Status:

```text
Implemented internally.
```

Scope:

```text
Add domain/attachment
LocalFS prepares local_path attachments for local_process targets
WorkspaceAttachmentService resolves HostedWorkspace metadata before mounting
No external proto changes
No ExecService or SandboxBackend
```

### PR 6: SandboxBackend and local-process-dev

Goal:

```text
Execute inside a sandbox without coupling exec to local directories.
```

Status:

```text
Implemented as an experimental development backend.
```

Scope:

```text
Add internal domain/backend
Add local-process-dev backend
Add WorkspaceExecService.ExecWorkspaceCommand
Control Plane forwards POST /sessions/{session_id}/workspace/exec
Execution uses WorkspaceAttachment.LocalPath
stdout / stderr byte limits and command timeout are enforced
```

Important warning:

```text
local-process-dev runs host processes for local development.
It is not a strong multi-tenant security boundary.
OS sandboxing, Docker, VM, and remote-agent backends are future profiles.
```

### PR 7: ProfileRegistry / Sandbox Profile Selection Cleanup

Goal:

```text
Make sandbox profile availability and selection explicit.
```

Status:

```text
Implemented.
```

Rules:

```text
ProfileRegistry is the sandbox-service source of truth.
CapabilityDescriptor sandbox_profiles are generated from enabled profiles.
local-process-dev appears only when sandbox.local_process.enabled = true.
CreateHostedWorkspace rejects unknown or disabled profiles.
WorkspaceExecService rejects profiles without workspace_exec capability.
Control Plane checks its default profile against the sandbox descriptor before
creating a workspace.
```

This is not a per-user or per-tenant policy layer. It only prevents the Control
Plane and sandbox-service from selecting profiles that the sandbox-service does
not currently expose as available.

---

### PR 8: Control Plane SandboxPolicy / PlacementResolver

Goal:

```text
Select the sandbox profile for workspace creation in the Control Plane.
```

Status:

```text
Implemented.
```

Rules:

```text
sandbox-service ProfileRegistry remains service-side capability truth.
CapabilityDescriptor advertises available profiles.
Control Plane SandboxPolicy decides who can use which profile.
Selection order is requested profile, user default, tenant default, global
default, then legacy sandbox.default_profile_id.
The selected profile must be policy-allowed and advertised by the target
sandbox-service as available.
WorkspaceRecord.current_host.sandbox_profile_id records the final selected
profile.
```

This is workspace creation-time placement only. It does not choose different
profiles for individual runs inside an existing workspace, and it does not
implement migration, scheduling, quota, or a new backend.

---

### PR 9: Workspace Execution Lease / Concurrency Guard

Goal:

```text
Protect workspace consistency with read/write leases.
```

Status:

```text
Implemented as an in-process lease manager.
```

Rules:

```text
ListWorkspaceDir and PreviewWorkspaceFile acquire read leases.
ExportWorkspacePath acquires a read lease until the blob snapshot is written.
ImportResourceToWorkspace and ExecWorkspaceCommand acquire write leases.
Multiple read leases may coexist.
A write lease is exclusive and blocks reads and writes.
Busy workspaces return FailedPrecondition / HTTP 409.
```

This is a single-process guard for the current sandbox-service. It is not a
distributed lock and does not survive process restarts. Clustered deployments
will need persisted leases, service-owned workspace routing, or another
distributed coordination mechanism.

---

## 11. Non-Goals For The Next PR

The next PR can focus on execution records or run-level operational semantics.
Do not include:

```text
Docker / VM / remote agent
Run-level profile selection
WorkspaceStore redesign
Large config schema migration
```

Execution code should continue to use WorkspaceAttachment rather than reaching
into LocalFS workspace roots directly. It should not change workspace view
behavior or resource upload/download/import flows.
