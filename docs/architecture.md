# Acorn Architecture

> This document defines Acorn's current Phase 1 architecture baseline. It is intentionally narrower than a full Agent Runtime: Acorn is the capability service substrate that a future runtime can build on.

---

## 1. Architecture Positioning

Acorn is a service-oriented capability substrate for future AI agent runtimes.

The current goal is to make these boundaries reliable before building agent loops, subagents, MCP tool wrappers, or real sandbox execution:

```text
Control Plane
  -> Capability Service
  -> Workspace
  -> View / Preview
  -> ResourceRef
  -> future Agent Runtime / MCP Tool
```

The first concrete capability service is `sandbox-service`, but the same model should later support browser, mail, document, messaging, and enterprise services.

---

## 2. System Layers

```text
Acorn
├─ Control Plane
│  ├─ Session
│  ├─ WorkspaceRecord
│  ├─ CapabilityDescriptor Registry
│  ├─ ResourceRecord / Resource Gateway
│  ├─ View Gateway
│  ├─ Policy / Audit
│  └─ future Agent Runtime
│
├─ Capability Services
│  ├─ sandbox-service
│  ├─ browser-service
│  ├─ mail-service
│  ├─ document-service
│  ├─ messaging-service
│  └─ future services
│
├─ Service-owned Domain State
│  ├─ HostedWorkspace
│  ├─ browser tabs
│  ├─ mail threads
│  ├─ downloads
│  └─ attachments
│
├─ Resource Plane
│  ├─ ResourceRef
│  ├─ ResourceRecord
│  ├─ upload
│  ├─ download
│  ├─ import
│  └─ export
│
└─ Agent-facing Surface
   ├─ MCP adapter
   ├─ future native tool router
   └─ model-facing tools
```

---

## 3. Core Principles

### 3.1 Control Plane owns relationships

The Control Plane owns relationship and platform state:

```text
Session
WorkspaceRecord
CapabilityDescriptor
ResourceRecord
Policy / Audit
```

The Control Plane does not own service internals:

```text
workspace file trees
sandbox instances
browser tab internals
mail storage
service-private paths
```

### 3.2 Capability Services own domain state

Each Capability Service owns its domain state and final domain enforcement.

Examples:

```text
sandbox-service
  HostedWorkspace
  workspace files
  WorkspacePathRef
  sandbox instance lifecycle
  file preview
  resource import/export

browser-service
  tabs
  current page
  screenshots
  downloads

mail-service
  threads
  messages
  attachments
  drafts
```

### 3.3 Workspace and SandboxInstance are separate

```text
Workspace = persistent state
SandboxInstance = disposable execution environment
```

Phase 1 binding:

```text
Session
  -> WorkspaceRecord

WorkspaceRecord
  -> WorkspaceHostRef

WorkspaceHostRef
  -> service_id
  -> service_workspace_id
  -> sandbox_profile_id

sandbox-service
  -> HostedWorkspace
  -> internal SandboxInstance, rebuildable
```

The Control Plane must not bind sessions to `sandbox_instance_id`. A sandbox instance may crash, restart, move, or be recreated while the workspace identity remains stable.

### 3.4 State is not runtime context

State is a service-exposed current status view. It is not prompt context, not a model message, and not a token-budgeted runtime summary.

`sandbox-service` currently exposes:

```text
HostedWorkspaceState
  ref
  status
  summary
  facts
  generated_at
```

State must not become a workspace file index.

### 3.5 View is not Resource

A View is a temporary, user-facing, read-only browse or preview result. It lets users inspect service-owned internal objects through the Control Plane without creating a `ResourceRef`.

```text
Preview/List does not create ResourceRef.
```

### 3.6 ResourceRef is the content boundary

`ResourceRef` is the platform-level content handle for upload, download, export, import, and cross-service transfer.

```text
Upload / Download / Export / Import / cross-service transfer uses ResourceRef.
```

Workspace files, mail attachments, screenshots, and downloads are not automatically ResourceRefs while they remain inside their owning service. They become ResourceRefs only when explicitly exported, uploaded, registered, or made downloadable/cross-service transferable.

### 3.7 Services do not read each other's internals

Services must not directly access each other's private paths or storage.

Correct cross-service exchange path:

```text
service A internal content
  -> Export / Register ResourceRef
  -> Control Plane Resource Gateway
  -> service B ImportResource
  -> service B internal object
```

---

## 4. Core Objects

### 4.1 Session

A Control Plane object representing a user conversation or task context.

```text
Session
  id
  user_id
  primary_workspace_record_id
```

### 4.2 WorkspaceRecord

A Control Plane object representing the session workspace binding. It is not a filesystem.

```text
WorkspaceRecord
  id
  session_id
  owner_user_id
  status
  current_host
  created_at
  updated_at
```

### 4.3 WorkspaceHostRef

The bridge between Control Plane and `sandbox-service`.

```text
WorkspaceHostRef
  service_id
  service_workspace_id
  sandbox_profile_id
```

It says which sandbox service currently hosts a workspace record.

### 4.4 HostedWorkspace

A `sandbox-service` object representing a hosted workspace.

```text
HostedWorkspace
  ref: WorkspaceHostRef
  status
  display_name
  created_at
  updated_at
```

Files, volumes, remote storage, and sandbox instance lifecycle are sandbox-service internals.

### 4.5 WorkspacePathRef

A sandbox-owned internal path reference, planned under `acorn.sandbox.v1`.

```text
WorkspacePathRef
  workspace: WorkspaceHostRef
  path: workspace-relative POSIX path
  kind: file | directory | unknown
```

Examples:

```text
outputs/report.pdf
src/main.py
```

A `WorkspacePathRef` is not a `ResourceRef`. It is meaningful only through the owning `sandbox-service` and cannot be directly read by other services.

### 4.6 ResourceRef

A lightweight platform-level content object reference.

Examples:

```text
user uploaded PDF
mail attachment
browser download
browser screenshot
sandbox-exported report.pdf
workspace export bundle
```

`ResourceRef` is the only cross-service content boundary.

```text
ResourceRef
  id
  authority_service_id
  name
  mime_type
  size_bytes
  content_hash
  metadata_json
```

`ResourceRef` intentionally does not carry owner, session, source, visibility, or lifecycle governance fields. Those belong to `ResourceRecord`.

### 4.7 ResourceRecord

A Control Plane object storing ResourceRef metadata, provenance, permissions, and lifecycle.

```text
ResourceRecord
  ref
  owner_user_id
  session_id
  source
  status
  visibility
  created_at
  updated_at
  metadata_json
```

Provenance belongs here in Phase 1. Acorn does not need a separate Artifact concept for exported user-facing outputs.

```text
ResourceSource
  type: user_upload | sandbox_export | mail_attachment | browser_download | ...
  source_service_id
  workspace_record_id
  service_workspace_id
  source_path
  run_id
  tool_call_id
  metadata_json
```

The Phase 1 `ResourceService` is metadata-only:

```text
RegisterResource
GetResource
ListResources
```

Resource bytes are streamed through `ResourceContentService` and the Control
Plane Resource Download Gateway:

```text
OpenResource
```

Upload and import remain separate flows.

---

## 5. Deferred Concepts

### 5.1 Artifact

Artifact is deferred. In Phase 1, exported user-facing outputs are represented directly as `ResourceRef` / `ResourceRecord`.

The current model is:

```text
WorkspacePathRef --export/download--> ResourceRef
```

not:

```text
WorkspacePathRef -> Artifact -> ResourceRef
```

### 5.2 EntityRef

EntityRef is deferred until State/Event/Signal need a generic reference to non-content service-owned objects such as:

```text
mail.thread
browser.tab
sandbox.process
calendar.event
approval.request
```

Current Phase 1 concepts remain:

```text
WorkspaceRecord
HostedWorkspace
WorkspacePathRef
ResourceRef
ResourceRecord
```

---

## 6. Capability Service Surfaces

A Capability Service may expose several surfaces. Not every service implements every surface.

```text
Capability Service
  ├─ Agent Surface
  ├─ Control Surface
  ├─ State Surface
  ├─ View Surface
  ├─ Resource Surface
  ├─ Observation / Event Surface
  ├─ Governance Surface
  └─ Domain Runtime
```

### 6.1 Control Surface

Lifecycle and management APIs used by the Control Plane, UI, admin tooling, and local management.

Examples:

```text
DescribeCapabilities
CreateHostedWorkspace
GetHostedWorkspace
CreateSessionWorkspace
ConfigureAccount
BindMessagingAccount
```

Control APIs are not primary model-facing APIs.

### 6.2 State Surface

Service-pull status view exposed to the Control Plane.

Examples:

```text
HostedWorkspaceState
browser current page summary
mail unread thread summary
gateway connection status
```

State is current status. It is not runtime context and not a file index.

### 6.3 View Surface

Temporary, read-only, user-facing preview and browse APIs.

Examples:

```text
sandbox-service
  ListWorkspaceDir
  PreviewWorkspaceFile

browser-service
  PreviewCurrentPage
  PreviewScreenshot

mail-service
  PreviewMailMessage
  PreviewAttachment

document-service
  PreviewDocumentPage
  PreviewOutline
```

Rules:

```text
View is not Resource.
Preview/List does not create ResourceRef.
View results are bounded and temporary.
Viewed content is not persisted in ResourceCatalog.
```

`sandbox-service` currently implements `ListWorkspaceDir` and
`PreviewWorkspaceFile` experimentally through `WorkspaceViewService`. The
Control Plane forwards session-scoped workspace view requests to the current
`WorkspaceHostRef` and validates returned path refs against the session's
`WorkspaceRecord`.

### 6.4 Resource Surface

User-facing and cross-service content exchange APIs.

Examples:

```text
Upload
Download
ImportResource
ExportResource
ExportWorkspacePath
RegisterResource
```

ResourceRef is created or used when content crosses a boundary or becomes user-downloadable.

Examples:

```text
user upload -> ResourceRef
mail attachment -> ResourceRef
browser download -> ResourceRef
sandbox exported workspace path -> ResourceRef
workspace bundle export -> ResourceRef
```

### 6.5 Observation / Event Surface

Audit, UI timeline, debug, metrics, tracing, and replay.

Examples:

```text
workspace.created
workspace.file.previewed
resource.exported
browser.screenshot.previewed
mail.attachment.exported
tool.call.started
tool.call.completed
policy.denied
```

View is returned to the user. Event records that something happened. These surfaces must not be confused.

### 6.6 Governance Surface

Policy, approval, permissions, audit, and risk boundaries.

Principle:

```text
Central Policy, Distributed Enforcement
```

The Control Plane is the policy center. Services perform final domain enforcement.

### 6.7 Agent Surface

Agent-facing adapters, primarily MCP today and future native tool routing later.

MCP positioning:

```text
MCP = agent-facing adapter
not Acorn's only internal protocol
not the control protocol
```

MCP tools should run inside an existing execution context. They should not create workspaces, choose sandbox profiles, or manage `ResourceRecord`.

---

## 7. User File Interaction Paths

### 7.1 User uploads a file

```text
User
  -> Control Plane User Upload Gateway
  -> Control Plane LocalBlobStore
  -> ResourceRecord
  -> ResourceRef
```

Uploaded resources use `authority_service_id = agent-control-plane`.

To place it into sandbox:

```text
ResourceRef
  -> Control Plane
  -> sandbox-service ImportResource
  -> WorkspacePathRef
```

### 7.2 User browses workspace

```text
User
  -> Control Plane View Gateway
  -> sandbox-service ListWorkspaceDir
  -> temporary view result
```

No `ResourceRef` is created.

### 7.3 User previews workspace file

```text
User
  -> Control Plane View Gateway
  -> sandbox-service PreviewWorkspaceFile
  -> temporary preview
```

No `ResourceRef` is created.

### 7.4 User exports workspace file

```text
User explicitly exports a workspace file
  -> Control Plane
  -> sandbox-service ExportWorkspacePath
  -> ResourceRef
  -> ResourceRecord
```

Export creates a `ResourceRef` and `ResourceRecord`.

Actual byte download is handled by a separate Resource Download Gateway flow.

---

## 8. View / Resource / Observation Boundary

| User intent | Surface | Creates ResourceRef |
| --- | --- | ---: |
| Browse workspace directory | View | No |
| Preview workspace file | View | No |
| Preview browser screenshot | View | No |
| Preview mail message | View | No |
| Download workspace file | Resource | Yes |
| Upload user file | Resource | Yes |
| Import mail attachment into sandbox | Resource | Uses existing ResourceRef |
| Attach sandbox file to mail | Resource | Yes |
| Record who previewed what | Observation/Event | No |

Rule of thumb:

```text
Users look through View Surface.
Users take away content or move content across services through ResourceRef.
```

---

## 9. Service-to-Service File Exchange

All service-to-service file exchange uses `ResourceRef`.

Forbidden:

```text
mail-service reads sandbox workspace paths
browser-service writes sandbox workspace paths
sandbox-service reads mail-service private attachment paths
```

Correct examples:

```text
mail attachment
  -> ResourceRef
  -> sandbox-service ImportResource
  -> WorkspacePathRef
```

```text
sandbox file
  -> ExportWorkspacePath
  -> ResourceRef
  -> mail-service attach/send
```

---

## 10. Capability Descriptor

Static manifests are not the source of truth. A running service exposes a runtime `CapabilityDescriptor`.

It describes:

```text
service_id
kind
contract
version
surfaces
endpoints
MCP agent surface
sandbox profiles
implementation status
```

Implementation status is not a boolean:

```text
disabled
declared
experimental
implemented
```

Descriptor is the basis for service discovery and capability routing.

Endpoint addresses are advertised caller-facing addresses, not necessarily bind addresses such as `0.0.0.0`.

Service identity is split from display naming:

```text
service.id   = stable routing and registry identity
service.name = display/app name
```

`CapabilityDescriptor.service_id`, `WorkspaceHostRef.service_id`, and `ResourceRef.authority_service_id` use `service.id`.

---

## 11. Proto Package Ownership

Recommended API layout:

```text
api/proto/acorn/common/v1/
  common.proto

api/proto/acorn/capability/v1/
  descriptor.proto

api/proto/acorn/workspace/v1/
  workspace.proto
  # WorkspaceRecord
  # WorkspaceHostRef
  # WorkspaceStatus

api/proto/acorn/sandbox/v1/
  workspace.proto
  # HostedWorkspace
  # HostedWorkspaceState
  # WorkspaceHostService

  path.proto       # WorkspacePathRef
  view.proto       # ListWorkspaceDir / PreviewWorkspaceFile
  transfer.proto   # ExportWorkspacePath

api/proto/acorn/resource/v1/
  resource.proto
  # ResourceRef
  # ResourceRecord
  # ResourceSource
  # metadata-only ResourceService

  content.proto
  # ResourceContentService
  # OpenResource streaming
```

Ownership rule:

```text
acorn.workspace.v1 = control-plane workspace abstractions
acorn.sandbox.v1   = sandbox-service domain APIs
acorn.resource.v1  = platform content exchange APIs
```

---

## 12. Current Implementation Status

Already established:

```text
CapabilityDescriptor
WorkspaceRecord / HostedWorkspace
HostedWorkspaceState
sandbox/v1 proto package split
View Surface declared in CapabilityDescriptor
Sandbox Workspace View Surface implementation
WorkspacePathRef
ListWorkspaceDir / PreviewWorkspaceFile
Control Plane workspace view forwarding
Artifact removed from Phase 1 proto/code path
ResourceRef / ResourceRecord contract
Control Plane in-memory ResourceRecord store
Control Plane resource metadata HTTP/gRPC API
Workspace Resource Export foundation
ExportWorkspacePath
Control Plane workspace export forwarding
ResourceBlobStore / LocalBlobStore
Snapshot-backed ExportWorkspacePath
ResourceContentService
Resource Download Gateway
ImportResource to Workspace
User Upload Gateway
Control Plane local resource authority
WorkspaceAttachment Foundation
LocalFS local_path attachment for local_process
SandboxBackend Foundation
WorkspaceExecService
local-process-dev backend
Control Plane workspace exec forwarding
ProfileRegistry / Sandbox Profile Selection Cleanup
Control Plane SandboxPolicy / PlacementResolver
Workspace Execution Lease / Concurrency Guard
```

Next planned code sequence:

```text
PR 1: Minimal Run model / ExecutionRecord
  - Record who executed what and when
  - Store exit code and output summary
  - Associate execution with session, user, workspace, and future run id
```

`local-process-dev` executes host processes for development and is not a strong
multi-tenant security boundary.

Sandbox profile selection now follows one rule: sandbox-service ProfileRegistry
is the source of truth. The sandbox descriptor is generated from enabled
profiles. Control Plane SandboxPolicy chooses a workspace creation-time profile
from requested, user, tenant, global, or legacy defaults, then verifies the
selected profile is policy-allowed and advertised by the target sandbox-service.
Exec is gated by the stored workspace profile's capabilities and backend id.

sandbox-service also uses an in-process WorkspaceLeaseManager for workspace
consistency. View and export acquire read leases; import and exec acquire write
leases. Multiple reads may coexist, while a write lease is exclusive. Busy
workspaces return FailedPrecondition / HTTP 409. This is not a distributed
lock; clustered sandbox-service deployments will need a persisted or routed
lease mechanism.

---

## 13. Phase 1 Scope

Phase 1 should establish:

```text
Capability Descriptor
Session workspace binding
Hosted workspace state
View Surface
ResourceRef / ResourceRecord
Workspace browse / preview
Resource export / import
Policy / audit
Service-side authorization
```

Phase 1 explicitly does not require:

```text
Full agent loop
Full context engine
Subagent scheduler
Run overlays or filesystem snapshots
Complex long-running process orchestration
Full automation system
Gateway/channel product layer
Mature memory system
```

Execution, MCP tools, and agent runtime should come after workspace, view, and resource boundaries are stable.

---

## 14. Glossary

**Capability Descriptor**
Runtime descriptor exposed by a capability service. It reports service identity, surfaces, endpoints, status, and profiles.

**Control Plane**
Acorn service that owns relationships, registries, session/workspace bindings, resource records, policy, audit, and future runtime orchestration.

**HostedWorkspace**
Sandbox-service-owned workspace object. It owns actual workspace state and future files/storage/backends.

**ResourceRef**
Platform content handle for uploaded, downloadable, exported, imported, or cross-service transferable content.

**ResourceRecord**
Control-plane metadata record for a ResourceRef, including provenance, ownership, permissions, status, and lifecycle.

**State Surface**
Service-exposed current status view. Not runtime context.

**View Surface**
Temporary, read-only, user-facing browse/preview surface. Does not create ResourceRef.

**WorkspaceHostRef**
Bridge from WorkspaceRecord to the current sandbox-service hosted workspace.

**WorkspacePathRef**
Sandbox-internal workspace path reference. Not globally readable and not a ResourceRef.

**WorkspaceRecord**
Control-plane session workspace binding record.

---

## 15. Invariants

1. Control Plane owns relationships, not service internals.
2. Capability Services own their domain state.
3. Workspace is persistent; SandboxInstance is disposable.
4. State is service-exposed status, not runtime context.
5. View Surface is for temporary user-facing browse/preview.
6. View does not create ResourceRef.
7. Upload, download, export, import, and cross-service transfer use ResourceRef.
8. WorkspacePathRef is sandbox-internal and not globally readable.
9. Services do not directly access each other's internal paths.
10. MCP is an agent-facing adapter, not Acorn's internal protocol.
11. Artifact and EntityRef are deferred, not Phase 1 concepts.
