# Acorn Architecture

> Status: architecture baseline for the next service-layout PR.  
> Scope: Cloud Agent Control Plane, independent Capability Services, standard service layout, Sandbox Service, Resource/Workspace boundary, and Kratos service pattern.

---

## 1. Executive Summary

Acorn is a service-oriented runtime architecture for AI agent capabilities.

The system is built around two major sides:

1. **Cloud Agent Control Plane**
2. **Independently deployable Capability Services**

A Capability Service is a real service implementation, usually a Kratos service, that exposes one capability domain such as sandbox, browser, mail, document processing, enterprise tools, or an MCP adapter.

The most important architectural decision is:

> **We do not use one generic node process to host all providers.  
> Each complex capability should be implemented as an independent Capability Service.**

A Capability Service provides a common external shape:

- **Agent Surface**: usually MCP, used by the agent/model runtime.
- **Control Surface**: HTTP/gRPC APIs for UI and control plane.
- **Observation Surface**: events, logs, progress, status, audit signals.
- **Resource Surface**: ResourceRef import/export and cross-service file/object exchange.
- **Governance Surface**: policy, approval, audit, and permission boundaries.
- **Domain Runtime**: service-owned runtime state and implementation logic.

The common shape is shared.  
The internal implementation is service-specific code.

---

## 2. Core Design Principles

### 2.1 Service boundary over plugin hosting

Acorn should not assume one generic `capability-node` process that hosts all capabilities.

Instead, complex capabilities are separate services:

```text
services/
  agent-control-plane/
  sandbox-service/
  browser-service/        # future
  mail-service/           # future
  document-service/       # future
  mcp-adapter-service/    # future
```

Each capability service can be deployed locally, in the cloud, in an enterprise network, or at the edge.

### 2.2 Unified service surfaces, not unified implementation

Acorn standardizes the **service surfaces**, not the internal runtime.

For example, both `sandbox-service` and `browser-service` may expose:

```text
MCP Agent Surface
HTTP/gRPC Control Surface
Event/Observation Surface
Resource Surface
Governance Surface
```

But their internal domain runtime is different:

```text
sandbox-service:
  workspaces, sandboxes, processes, artifacts

browser-service:
  browser contexts, tabs, pages, screenshots, downloads

mail-service:
  accounts, messages, threads, attachments, drafts
```

### 2.3 Configuration selects implementation; it does not define implementation

Implementation differences must be code-level differences, not YAML templates.

Configuration can:

- select a code implementation,
- provide parameters,
- enable or disable features,
- set resource limits,
- declare endpoints.

Configuration must not try to define how local Linux isolation, Windows sandboxing, cloud isolation, browser control, or process execution actually works.

```text
Correct:
  config selects local_linux implementation.

Incorrect:
  config describes local_linux sandbox behavior from scratch.
```

### 2.4 Workspace is not a global shared filesystem

Workspace is not an Acorn-wide top-level shared filesystem.

For sandbox:

> **Workspace is a sandbox-service domain object.**

A workspace is persistent file state owned by `sandbox-service`.  
A sandbox can mount a workspace for execution.  
The workspace can survive sandbox restart or rebuild.

Cross-service file exchange must use `ResourceRef`, not a shared workspace.

### 2.5 Cross-service files flow through ResourceRef

Files crossing service boundaries are represented as Resources.

Examples:

- user upload,
- browser download,
- mail attachment,
- document processing output,
- sandbox exported artifact.

The boundary is explicit:

```text
ResourceRef -> sandbox-service import -> workspace file
workspace file -> sandbox-service export -> ResourceRef
```

---

## 3. System-Level Architecture

```text
Cloud Agent Control Plane
  ├─ Agent Runtime
  ├─ Session Manager
  ├─ Tool Router
  ├─ Signal Router
  ├─ Capability Service Registry
  ├─ Resource Store / Object Store
  ├─ Resource Metadata
  ├─ Policy / Audit Metadata
  └─ UI / Control APIs

Capability Services
  ├─ sandbox-service
  ├─ browser-service
  ├─ mail-service
  ├─ document-service
  ├─ enterprise-tool-service
  └─ mcp-adapter-service
```

The Cloud Agent Control Plane coordinates sessions, models, routing, resources, and policy metadata.

Capability Services own their runtime state and expose capabilities through common surfaces.

---

## 4. Capability Service Pattern

A Capability Service is an independently deployable service that provides one capability domain.

```text
Capability Service
  ├─ Agent Surface
  ├─ Control Surface
  ├─ Observation Surface
  ├─ Resource Surface
  ├─ Governance Surface
  └─ Domain Runtime
```

### 4.1 Agent Surface

The Agent Surface is used by the agent/model runtime.

The preferred protocol is MCP.

Examples:

```text
sandbox tool calls
workspace file access
browser operations
mail operations
document parsing
```

The Agent Surface should hide operational routing details from the model.  
For example, the model should not need to know `service_id`, `workspace authority`, or object-store routing.

### 4.2 Control Surface

The Control Surface is used by the control plane, UI, admin tools, or local management.

The preferred protocols are HTTP and gRPC.

Examples:

```text
sandbox lifecycle control
workspace lifecycle control
resource import/export control
browser tab control
download status control
```

The Control Surface is not meant to be the primary model-facing API.

### 4.3 Observation Surface

The Observation Surface exposes events, logs, progress, runtime status, and audit-relevant facts.

Examples:

```text
sandbox.process.started
sandbox.process.exited
sandbox.artifact.discovered
browser.download.completed
browser.console.error
mail.message.received
```

Observation data may be consumed by:

- UI,
- control plane,
- audit,
- trigger engine,
- debugging tools,
- agent runtime context injection.

### 4.4 Resource Surface

The Resource Surface handles cross-service resource exchange.

Examples:

```text
import resource into service domain state
export service domain state as resource
ListResources
ReadResource
```

It uses `ResourceRef` as the cross-service resource representation.

### 4.5 Governance Surface

The Governance Surface contains policy, approval, audit, and permission boundaries.

Governance is a standard surface, but it is not a separate transport requirement. It may be reached through HTTP, gRPC, MCP tool handling, internal hooks, or control-plane integration depending on the service.

Examples:

```text
approval before destructive sandbox command
audit record for resource export
policy check before reading host file
approval before sending external message
```

### 4.6 Domain Runtime

Domain Runtime is service-specific code and state.

It is not standardized by the generic Capability Service Pattern.

Examples:

```text
sandbox-service:
  sandbox, workspace, process, artifact, import/export

browser-service:
  browser context, tab, page, download, screenshot

mail-service:
  account, mailbox, thread, message, attachment, draft
```

---

## 5. Capability Class Contract

The generic Capability Service Pattern is not enough by itself.

It standardizes service shape, but it does not define what a sandbox, browser, or mail service means.

Therefore, Acorn should also define **Capability Class Contracts**.

```text
Capability Service Pattern
  └─ Capability Class Contract
       └─ Code Implementation
```

Examples:

```text
Sandbox Capability Contract
Browser Capability Contract
Mail Capability Contract
Document Capability Contract
```

A Capability Class Contract defines shared domain semantics for a class of services.

It is not a YAML template.  
It is not a generic provider host.  
It is a domain contract implemented by code.

---

## 6. Sandbox Capability Contract

`sandbox-service` is the first Capability Service.

It implements the Sandbox Capability Contract.

### 6.1 Core sandbox concepts

```text
sandbox-service
  ├─ Workspace
  ├─ Sandbox
  ├─ Process
  ├─ Artifact
  └─ Import/Export boundary
```

### 6.2 Sandbox Service

`sandbox-service` is a long-running Kratos Capability Service.

It can manage:

- zero, one, or many sandboxes,
- zero, one, or many workspaces,
- workspace-to-sandbox bindings,
- processes,
- artifacts,
- resource import/export.

Important rule:

> `sandbox-service` is not a single sandbox instance.  
> It is a service that provides sandbox capability.

### 6.3 Workspace

A Workspace is persistent file state owned by `sandbox-service`.

It may contain conventional directories such as:

```text
/uploads
/workspace
/outputs
/.metadata
```

A Workspace:

- belongs to `sandbox-service`,
- is not a global Acorn filesystem,
- may be mounted by a sandbox,
- may survive sandbox restart/rebuild,
- may be snapshotted or migrated through explicit export/import,
- becomes visible to the model as file paths only when bound to the current sandbox/session context.

A file is a workspace file only if:

1. it was created by sandbox execution, or
2. it was explicitly imported into the workspace.

### 6.4 Sandbox

A Sandbox is an execution context managed by `sandbox-service`.

A Sandbox:

- can mount a Workspace,
- can run processes,
- can be created/destroyed/restarted,
- does not own the Workspace lifecycle,
- may be local, cloud, enterprise, or edge-backed as an implementation detail.

Relationship:

```text
sandbox-service
  ├─ workspace ws_1
  ├─ workspace ws_2
  ├─ sandbox sbox_1 -> mounts ws_1
  └─ sandbox sbox_2 -> mounts ws_2
```

### 6.5 Process

A Process is an execution unit inside a sandbox.

A process should have a common observable shape:

```text
process_id
sandbox_id
command
status
stdout/stderr
exit_code
started_at
finished_at
```

Whether the underlying implementation is a Windows process, Linux process, container exec, Kubernetes job, or cloud execution unit is implementation-specific code.

### 6.6 Artifact

An Artifact is a user-relevant output discovered in a Workspace.

Examples:

```text
/outputs/report.md
/outputs/chart.png
/outputs/result.csv
```

An Artifact can be exported into a `ResourceRef` for cross-service use.

### 6.7 Import

Import materializes a `ResourceRef` into a Workspace.

```text
ResourceRef -> import -> Workspace file
```

Example:

```text
resource://uploads/data.csv
  -> /uploads/data.csv
```

### 6.8 Export

Export turns a Workspace file into a `ResourceRef`.

```text
Workspace file -> export -> ResourceRef
```

Example:

```text
/outputs/report.md
  -> resource://sandbox_exports/report.md
```

### 6.9 Local/cloud/platform differences

Local sandbox, cloud sandbox, Windows sandbox, Linux sandbox, Docker sandbox, and enterprise sandbox are code implementations of the Sandbox Capability Contract.

They are not YAML templates.

Configuration may select the implementation and provide parameters, but the implementation difference is code.

---

## 7. Resource Model

`ResourceRef` is the cross-service resource object.

It should express:

```text
id
uri
name
type
mime_type
size
authority
service_id
provider_id
owner_type
owner_id
version
content_hash
metadata_json
```

### 7.1 Resource authority

`authority` describes where the authoritative bytes live.

Possible values include:

```text
object_store
sandbox
service_local
external
```

### 7.2 Resource ownership

`owner_type` and `owner_id` describe the logical source or ownership context.

Examples:

```text
owner_type=upload
owner_id=upload_123

owner_type=browser
owner_id=browser_session_456

owner_type=mail
owner_id=message_789

owner_type=sandbox
owner_id=workspace_or_sandbox_id
```

### 7.3 Resource is not Workspace

A Resource is not automatically a Workspace file.

A Resource becomes a Workspace file only after explicit import into a Workspace managed by `sandbox-service`.

Likewise, a Workspace file becomes a cross-service Resource only after explicit export.

---

## 8. Model-Visible File Paths

The model should see simple workspace paths:

```text
/uploads/data.csv
/workspace/main.py
/outputs/report.md
```

The model should not need to reason about:

```text
service_id
workspace authority
object store location
ResourceRef routing
copy/import/export mechanics
```

The Agent Runtime and Control Plane should resolve these details based on session context.

A typical mapping:

```text
session_id -> workspace_id -> sandbox-service service_id
session_id -> current_sandbox_id
```

When the model calls a sandbox tool, the runtime can route the call to the correct `sandbox-service` and workspace.

---

## 9. Cross-Service File Flow

### 9.1 User upload to sandbox

```text
User Upload
  -> Resource Store / Object Store
  -> ResourceRef(owner_type=upload)
  -> sandbox-service import
  -> workspace:/uploads/data.csv
```

### 9.2 Browser download to sandbox

```text
browser-service download
  -> ResourceRef(owner_type=browser)
  -> sandbox-service import
  -> workspace:/uploads/file.pdf
```

### 9.3 Sandbox output to mail service

```text
workspace:/outputs/report.md
  -> sandbox-service export
  -> ResourceRef(owner_type=sandbox)
  -> mail-service consumes ResourceRef
```

### 9.4 Workspace migration

Workspace migration is explicit.

```text
sandbox-service A workspace
  -> export snapshot/resource bundle
  -> Resource Store / Object Store
  -> sandbox-service B import
```

This changes the authoritative service for the workspace.

Acorn should not assume cross-service real-time shared workspace synchronization.

---

## 10. Kratos Service Pattern

Each Capability Service should follow a Kratos service structure.

Recommended layout:

```text
services/<capability>-service/
  cmd/
    <capability>-service/
      main.go

  configs/
    config.yaml

  internal/
    app/
    conf/
    server/
      http.go
      grpc.go
      mcp.go
    surface/
      agent/
      control/
      observation/
      resource/
      governance/
    manifest/
    domain/
      ...
    service/
    version/
```

For `sandbox-service`:

```text
services/sandbox-service/internal/
  domain/
    sandbox/
    workspace/
    process/
    artifact/
    importexport/
```

### 10.1 `server/`

Transport-level setup:

```text
HTTP
gRPC
MCP placeholder / later MCP server
```

`server/` is responsible for protocol entrypoints. It should route into surface packages, not contain domain logic.

### 10.2 `surface/`

Surface boundary documentation and later handlers:

```text
agent       MCP-facing surface
control     HTTP/gRPC control APIs
observation events/logs/progress/status
resource    import/export/resource operations
governance  policy/approval/audit/permission boundaries
```

`surface/` expresses external capability boundaries. It is not the same layer as `server/`; one surface can be exposed through multiple transports, and one transport can route to multiple surfaces.

### 10.3 `manifest/`

Capability and provider manifest declarations.

A Capability Service manifest declares service identity and class contract:

```text
service id
service kind
contract
version
features
```

A Provider manifest declares provider-facing surfaces:

```text
agent surface protocol
tool specs
control features
observation events and metrics
resource types
security/governance policy metadata
```

These manifests describe identity, capability, and endpoint metadata. They do not implement runtime behavior.

### 10.4 `domain/`

Service-owned domain concepts and implementation boundaries.

The domain layout is service-specific:

```text
sandbox-service/internal/domain/
  sandbox/
  workspace/
  process/
  artifact/
  importexport/

browser-service/internal/domain/
  context/
  tab/
  page/
  download/
  screenshot/

mail-service/internal/domain/
  account/
  thread/
  message/
  attachment/
  draft/
```

Domain packages should not depend on transport packages.

---

## 11. `packages/servicekit`

A shared service infrastructure package may be introduced for Kratos service baseline.

It may provide:

```text
BuildInfo
standard logger helper
default server middleware
```

It may depend on Kratos.

It must not contain:

```text
sandbox logic
browser logic
provider runtime
resource store
MCP SDK
object store
agent runtime
```

Important dependency rule:

```text
packages/core must not depend on Kratos.
packages/api must not depend on servicekit.
packages/servicekit may depend on Kratos.
```

---

## 12. Manifest and Registration

Capability Services should be discoverable through manifests.

There are two related manifest levels:

```text
CapabilityService
  service identity
  deployment profile
  lifecycle status
  implemented capability class contract
  service-level features

ProviderManifest
  provider identity
  agent/control/observation/resource/governance surfaces
  tool specs
  resource types
  security policy metadata
```

`CapabilityService` is used by service registry and routing. `ProviderManifest` is used to describe provider-facing capability surfaces.

Example sandbox capability service shape:

```yaml
id: sandbox.local.dev
name: sandbox-service
kind: sandbox
contract: acorn.sandbox
version: dev
features: []
```

Example sandbox provider manifest shape:

```yaml
id: sandbox
type: sandbox
version: dev

surfaces:
  agent:
    protocol: mcp
  control:
    protocols: [grpc, http]
  observation:
    events:
      - sandbox.created
      - sandbox.process.exited
      - sandbox.artifact.discovered
  resource:
    types:
      - sandbox_export
      - workspace_snapshot
      - sandbox_artifact

features: []
```

Manifests are descriptive. They are not runtime implementation templates.

---

## 13. What Configuration Can and Cannot Do

Configuration can:

- select a code implementation,
- configure ports,
- configure data directories,
- configure resource limits,
- configure credentials,
- enable/disable features,
- choose defaults.

Configuration cannot:

- define how sandbox isolation works,
- define how browser automation works,
- define how mail synchronization works,
- replace platform-specific implementation code,
- replace service runtime logic.

Implementation differences must live in code.

---

## 14. Contract Tests Strategy

Contract tests should happen after the service pattern is stable.

There should be two categories.

### 14.1 Generic Capability Service contract tests

These verify shared Acorn contracts:

```text
Capability Service manifest
Provider manifest
Tool request/result
Signal ingest
Event publish
ResourceRef model
```

### 14.2 Capability Class contract tests

These verify class-specific behavior.

For sandbox:

```text
create workspace
create sandbox
mount workspace
import ResourceRef
export file as ResourceRef
process lifecycle
artifact discovery
```

Early PRs may only add generic contract tests.  
Sandbox class contract tests should be added when `api/proto/acorn/sandbox` or equivalent internal service contracts stabilize.

---

## 15. Current Implementation Direction

Current intended repository direction:

```text
services/
  agent-control-plane/
  sandbox-service/

api/proto/acorn/
  capability/
  common/
  event/
  provider/
  resource/
  signal/
  tool/

packages/
  api/
  core/
  mcp/
  provider-sdk-go/
  servicekit/        # planned
  signal/
```

Top-level `workspace` should not exist.

Top-level `node` should not exist if the architecture uses `CapabilityService` terminology.

`sandbox-service` is the first independent Capability Service.

---

## 16. Recommended Next PRs

### PR5: Standard Capability Service Layout

Goal:

```text
Establish the standard Capability Service layout.
Use sandbox-service as the first concrete service skeleton.
```

Should include:

```text
packages/servicekit
sandbox-service standard surface packages:
  agent/control/observation/resource/governance
sandbox-service domain packages:
  sandbox/workspace/process/artifact/importexport
sandbox-service MCP placeholder
manifest skeleton:
  CapabilityService + ProviderManifest
README/docs update
```

Should not include:

```text
real MCP SDK
sandbox execution
workspace filesystem implementation
object store
import/export runtime
browser-service
agent runtime
api/proto/acorn/sandbox/v1
runner/backend implementation packages
contract tests
```

### PR6: Core Contract Tests

Goal:

```text
Lock current generic contracts:
CapabilityService, Provider, Tool, Signal, Event, Resource.
```

### PR7: Sandbox Service Domain Skeleton

Goal:

```text
Define sandbox-service internal domain managers and interfaces:
workspace manager, sandbox manager, process manager,
artifact discovery, import/export boundaries.
```

No real execution backend yet.

### PR8: Sandbox Control API Draft

Goal:

```text
Decide whether sandbox-service needs api/proto/acorn/sandbox/v1
or whether control contracts remain service-internal for now.
```

Draft wire-level operations only after domain interfaces stabilize.

### PR9+: Sandbox Execution Implementation

Later PRs can add:

```text
local execution
platform-specific isolation
cloud execution
MCP tool implementation
HTTP/gRPC control APIs
artifact scanning
resource import/export implementation
```

---

## 17. Non-Goals

The architecture explicitly does not assume:

```text
one capability-node process hosting all providers
global shared workspace filesystem
YAML-defined sandbox implementation
cross-service real-time workspace synchronization
model-visible service routing details
```

---

## 18. Short Glossary

### Cloud Agent Control Plane

Cloud-side service responsible for agent sessions, model calls, routing, service registry, resource store, policy metadata, and UI/control APIs.

### Capability Service

Independent service providing one capability domain, such as sandbox, browser, mail, document processing, or MCP adapter.

### Agent Surface

MCP-facing surface used by agent/model runtime.

### Control Surface

HTTP/gRPC surface used by UI and control plane.

### Observation Surface

Events, logs, progress, metrics, and audit-relevant runtime observations.

### Resource Surface

ResourceRef-based import/export/list/read surface for cross-service file/object exchange.

### Capability Class Contract

Domain contract for a category of capability services, such as sandbox or browser.

### Sandbox Service

Capability Service implementing sandbox capability.

### Workspace

Persistent file state owned by sandbox-service. It can be mounted by a sandbox and exchanged with other services only through ResourceRef import/export.

### Sandbox

Execution context managed by sandbox-service. It can mount a Workspace and run processes.

### ResourceRef

Cross-service resource reference used for uploads, downloads, attachments, artifacts, and exported sandbox files.

---

## 19. Core Rule Summary

```text
1. Acorn uses independent Capability Services.
2. A Capability Service exposes common surfaces.
3. Internal runtime differences are code, not templates.
4. Sandbox-service may manage many sandboxes and many workspaces.
5. Workspace belongs to sandbox-service, not global Acorn.
6. Sandbox mounts workspace.
7. Cross-service file flow uses ResourceRef.
8. Import turns ResourceRef into workspace file.
9. Export turns workspace file into ResourceRef.
10. Model sees workspace paths, not routing details.
```
