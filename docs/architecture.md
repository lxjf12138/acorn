# Acorn Architecture

> This document defines Acorn's architecture boundaries, invariants, and Phase 1 design scope.
> It replaces earlier architecture notes that treated static manifests and generic `ResourceRef` usage too broadly.

---

## 1. Executive Summary

Acorn is a service-oriented capability substrate for future AI agent runtimes.

The first phase of Acorn is **not** a full agent runtime. It is the capability, control, resource, policy, and service integration layer that future agent runtimes can rely on.

Acorn is organized around:

1. **Cloud Agent Control Plane**
2. **Independent Capability Services**
3. **Runtime-native capabilities**
4. **MCP agent-surface adapters**
5. **Acorn-native control/state/resource/signal/event/governance APIs**

The core idea is:

> Acorn standardizes how services expose capabilities to an agent platform.
> It does not force all capabilities to share one implementation model.

Complex capabilities such as sandbox, browser, mail, document processing, messaging, device gateways, and enterprise tools should usually be implemented as independent Capability Services.

Lightweight capabilities such as todos, planning notes, summaries, memory, heartbeats, and context bookmarks can remain runtime-native.

MCP is the preferred model-facing protocol for agent tools. It is **not** Acorn's only internal protocol and should not be used as the control protocol.

---

## 2. Phase 1 Scope

Acorn Phase 1 focuses on building a reliable capability substrate, especially around `sandbox-service` and the control plane.

Phase 1 should establish:

```text
Capability registration
Capability Descriptor
Session workspace binding
Sandbox backend/profile selection
Workspace creation
Tool invocation boundary
State pull
Event observation
Signal ingestion
Artifact creation
Resource promotion
Policy configuration
Service-side authorization
```

Phase 1 explicitly does **not** require:

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

However, Phase 1 APIs must carry runtime-compatible identifiers such as `session_id`, `run_id`, `tool_call_id`, and `trace_id` so that a future Agent Runtime can attach without rewriting capability services.

---

## 3. Core Architecture Principles

### 3.1 Capability Services over generic plugin hosting

Acorn should not assume a single generic process that hosts all capability providers.

Complex capabilities should be independently deployable services:

```text
services/
  agent-control-plane/
  sandbox-service/
  browser-service/        # future
  mail-service/           # future
  document-service/       # future
  messaging-service/      # future
  gateway-service/        # future
  mcp-adapter-service/    # future
```

Each service may run locally, in the cloud, in an enterprise network, or at the edge.

### 3.2 Unified surfaces, not unified internals

Acorn standardizes service surfaces. It does not standardize service internals.

For example:

```text
sandbox-service:
  workspace, sandbox backend, process, artifact, import/export

browser-service:
  browser context, page, tab, download, screenshot, console event

mail-service:
  account, thread, message, attachment, draft
```

The same external surface model can connect very different internal runtimes.

### 3.3 Code implements behavior; configuration selects behavior

Implementation differences must be code-level differences, not YAML templates.

Configuration may:

```text
select an implementation
provide parameters
enable or disable features
set limits
declare endpoints
configure delivery and retry policies
```

Configuration must not define how sandbox isolation, browser runtime behavior, or messaging protocol semantics work internally.

### 3.4 Workspace is not a global filesystem

Workspace is not a top-level Acorn shared filesystem.

For sandbox:

> A workspace is a `sandbox-service` domain object.

A workspace is persistent file state managed by `sandbox-service`. A sandbox instance may mount the workspace for execution. The workspace may survive sandbox restart, rebuild, or backend replacement.

Cross-service file exchange must happen through explicit import/export or artifact/resource promotion. Services must not share raw workspace paths.

### 3.5 Session owns workspace

In Phase 1:

```text
One conversation session owns one primary workspace.
Runs within the same session share that workspace.
```

Run overlays, snapshots, forks, copy-on-write layers, and rollback are future extensions. They should not be required for Phase 1.

### 3.6 Services expose state; runtime builds context

Capability Services should not write model messages.

Services provide:

```text
State      # service-exposed current status
Signals    # service-pushed facts requiring attention
Events     # observation/audit/debug timeline
Resources  # promoted content objects
Tool results
```

The Agent Runtime and its middleware decide how to filter, summarize, rank, compress, and inject these inputs into model context.

State is not runtime context. State is what a service chooses to expose to the control plane and runtime.

### 3.7 Resources are promoted content, not every file

A workspace may contain millions of files. Acorn must not register every workspace file as a global resource.

ResourceRef means:

> a promoted, content-bearing, readable or transferable object that crosses a service boundary.

Workspace files are not ResourceRefs by default.

---

## 4. System-Level Architecture

```text
Cloud Agent Control Plane
  ├─ Capability Registry
  ├─ Capability Descriptor Registry
  ├─ Session Manager
  ├─ Workspace Binding Store
  ├─ Tool Router / Tool Gateway
  ├─ Signal Inbox
  ├─ Event Log
  ├─ Resource Metadata Catalog
  ├─ Policy / Audit Metadata
  ├─ UI / Control APIs
  └─ Future Agent Runtime

Capability Services
  ├─ sandbox-service
  ├─ browser-service
  ├─ mail-service
  ├─ document-service
  ├─ messaging-service
  ├─ gateway-service
  ├─ enterprise-tool-service
  └─ mcp-adapter-service

Runtime-native Capabilities
  ├─ todo
  ├─ plan
  ├─ summary
  ├─ memory
  ├─ heartbeat
  └─ context bookmarks

Agent-facing Adapters
  ├─ MCP servers exposed by capability services
  ├─ Control-plane MCP proxy, future
  └─ External MCP tools
```

The control plane coordinates services, sessions, workspace bindings, resource metadata, policy metadata, tool routing, and event/signal flow.

Capability Services own their domain state and enforce their local security boundaries.

A future Agent Runtime uses the control plane and capability services, but Phase 1 does not require a full runtime implementation.

---

## 5. Capability Service Pattern

A Capability Service is an independently deployable service that owns one capability domain.

```text
Capability Service
  ├─ Agent Surface
  ├─ Control Surface
  ├─ State Surface
  ├─ Signal Surface
  ├─ Observation Surface
  ├─ Resource Surface
  ├─ Governance Surface
  └─ Domain Runtime
```

Not every service implements every surface.

### 5.1 Agent Surface

The Agent Surface is used for model-initiated actions.

The preferred protocol is MCP.

Examples:

```text
sandbox.exec
sandbox.read_file
sandbox.write_file
sandbox.list_dir
sandbox.present_files
browser.open
browser.click
mail.search
mail.create_draft
document.parse
```

The Agent Surface should hide routing details from the model. The model should not need to know service IDs, backend IDs, workspace authority, object-store routing, or file transfer mechanics.

MCP tools operate inside an existing execution context. MCP should not be responsible for workspace creation, backend selection, resource import/export, or service registration.

### 5.2 Control Surface

The Control Surface is used by the control plane, UI, admin tooling, and local management.

Preferred protocols are Acorn-native HTTP/gRPC APIs.

Examples:

```text
DescribeCapabilities
CreateWorkspace
DestroyWorkspace
ImportResource
CreateArtifactFromPath
ExportResource
GetWorkspaceState
ConfigureAccount
BindMessagingAccount
```

Control APIs are not primary model-facing APIs.

### 5.3 State Surface

The State Surface is service pull.

A service exposes current state that it chooses to make available to the control plane/runtime.

Examples:

```text
sandbox workspace summary
sandbox mounted backend status
sandbox recent process summary
sandbox presented artifacts
browser current page and tabs
mail unread thread summary
gateway connection status
```

State is not context. Context is produced later by runtime middleware.

### 5.4 Signal Surface

The Signal Surface is service push.

A Signal is a service-submitted fact that the control plane/runtime may need to notice.

Examples:

```text
sandbox.artifact.created
sandbox.process.exited
browser.download.completed
mail.thread.updated
approval.requested
resource.available
```

Signals should be text-first and coarse-grained. Large content must be represented by references, not embedded into the Signal.

### 5.5 Observation Surface

Observation Events are used for UI, audit, logs, metrics, tracing, and replay.

Examples:

```text
tool.call.started
tool.call.completed
sandbox.stdout
sandbox.stderr
workspace.changed
resource.imported
artifact.created
policy.denied
```

Events do not automatically enter model context.

### 5.6 Resource Surface

The Resource Surface handles promoted content objects that cross service boundaries.

Examples:

```text
user upload
mail attachment
browser download
browser screenshot
sandbox artifact
document processing output
large tool result
```

Resources are represented by ResourceRef.

### 5.7 Governance Surface

The Governance Surface handles policy, approval, audit, permissions, and risk boundaries.

Governance is cross-cutting, but each service must enforce domain-specific checks internally.

Examples:

```text
approval before dangerous shell command
file export permission
network access policy
credential use policy
message sending approval
recipient allowlist
audit records
```

### 5.8 Domain Runtime

The Domain Runtime is service-specific code.

Examples:

```text
sandbox-service:
  workspace, backend profile, sandbox instance, process, artifact, import/export

browser-service:
  browser context, tab, page, download, screenshot

messaging-service:
  account, channel, conversation, message, attachment, delivery
```

Domain Runtime is not standardized into a generic template.

---

## 6. Capability Descriptor

Acorn should remove static service manifests as the source of truth.

Instead, each service exposes a runtime **Capability Descriptor**.

A Capability Descriptor describes what the running service actually implements.

It should include:

```text
service identity
capability contract
service version
transport endpoints
agent-facing MCP tools
native control features
state subjects
resource capabilities
signal/event types
governance hooks
sandbox/backend profiles when applicable
```

The descriptor may be generated from code registration. Static descriptors may be used for development, but runtime registration should reflect actual implemented surfaces.

### 6.1 Example Descriptor

```yaml
id: sandbox.local.dev
name: sandbox-service
kind: sandbox
contract: acorn.sandbox
version: dev

endpoints:
  control:
    grpc: localhost:9001
    http: localhost:8001
  agent:
    mcp:
      transport: http
      endpoint: http://localhost:8001/mcp

agent_surface:
  protocol: mcp
  tools:
    - name: sandbox.exec
    - name: sandbox.read_file
    - name: sandbox.write_file
    - name: sandbox.list_dir
    - name: sandbox.present_files

control_surface:
  features:
    - describe_capabilities
    - create_workspace
    - destroy_workspace
    - import_resource
    - create_artifact_from_path
    - export_resource
    - get_state

state_surface:
  subjects:
    - sandbox.workspace
    - sandbox.backend
    - sandbox.process
    - sandbox.artifact

resource_surface:
  capabilities:
    - import_resource
    - export_resource
    - create_artifact_from_path

signal_types:
  - sandbox.workspace.changed
  - sandbox.artifact.created
  - sandbox.resource.imported
  - sandbox.resource.exported

sandbox_profiles:
  - id: local-docker
    default: true
    isolation: container
    os: linux
    capabilities:
      - exec
      - filesystem
      - persistent_workspace

  - id: local-process
    isolation: process
    os: host
    capabilities:
      - exec
      - filesystem
      - persistent_workspace
```

### 6.2 Descriptor vs Manifest

```text
Old Manifest:
  static declaration, easy to drift from implementation

Capability Descriptor:
  runtime description of implemented surfaces and exposed tools
```

The project layout should use `descriptor/` instead of `manifest/`.

---

## 7. MCP and Native APIs

MCP is the preferred model-facing adapter protocol for Agent Surface tools.

MCP is not Acorn's internal control protocol.

Acorn-native APIs remain authoritative for:

```text
control
state
resource
signal
observation
governance
registration
workspace lifecycle
policy sync
```

### 7.1 Phase 1 MCP topology

In Phase 1, a capability service may expose MCP directly.

```text
Agent Runtime / MCP client
  -> sandbox-service MCP endpoint
      -> sandbox-service domain runtime
```

The MCP client or future runtime injects execution context. The model should not manually provide raw `workspace_id`, backend routing, or policy grants.

### 7.2 Future control-plane MCP proxy

Later, the control plane may proxy MCP to provide:

```text
central tool routing
model-facing tool filtering
approval orchestration
audit
credential injection
multi-service composition
multi-backend selection
```

Future topology:

```text
Agent Runtime
  -> Control Plane MCP Proxy / Tool Router
      -> Capability Service native APIs or MCP endpoint
```

Do not require this proxy for Phase 1.

### 7.3 MCP tools require existing execution context

MCP tools operate inside an existing session/workspace/run context.

They should not create sessions or workspaces. Workspace creation and backend/profile selection are control-plane operations through Acorn-native APIs.

---

## 8. Session, Run, Workspace, Sandbox

### 8.1 Session

A Session represents one user conversation or continuous task context.

In Phase 1:

```text
Session owns one primary workspace.
```

### 8.2 Run

A Run is a logical execution boundary within a Session.

Runs share the Session workspace in Phase 1.

Runs should carry identifiers for future runtime support:

```text
session_id
run_id
turn_id      # optional in Phase 1
tool_call_id
trace_id
```

### 8.3 Workspace

A Workspace is persistent file state owned by `sandbox-service`.

It is not globally shared.

A workspace may be mounted by different sandbox instances over time.

### 8.4 Sandbox Service

`sandbox-service` is a long-running Capability Service. It is not a sandbox instance.

It may manage:

```text
zero or more workspaces
zero or more sandbox instances
one or more sandbox backend profiles
processes
artifacts
resource import/export
sandbox state/signals/events
```

### 8.5 Sandbox Backend/Profile

A Sandbox Backend/Profile is a selectable execution implementation exposed by `sandbox-service`.

Examples:

```text
local-process
local-docker
local-firecracker
local-wasm
cloud-standard
cloud-gpu
remote-provider
```

The control plane may select a backend profile when creating or using a workspace.

The workspace remains a `sandbox-service` domain object and may outlive a sandbox instance.

### 8.6 Sandbox Instance

A Sandbox Instance is an execution environment managed internally by `sandbox-service`.

It may be:

```text
local process
container
VM
remote cloud sandbox
provider SDK allocation
```

A sandbox instance may be created lazily when a tool call needs execution.

Phase 1 does not require an explicit `CreateSandbox` API. A simple model is:

```text
CreateWorkspace(session_id, profile_id)
Exec(workspace_id, execution_context)
  -> sandbox-service lazily acquires sandbox instance
```

Explicit `AcquireSandbox`, `ReleaseSandbox`, snapshots, and overlays are future extensions.

---

## 9. State, EntityRef, WorkspacePathRef, Artifact, ResourceRef

Acorn should not use `ResourceRef` for every domain object.

Use different references for different semantics.

### 9.1 State

State is a service-exposed current view.

A service decides what state to expose.

State may include text, structured metadata, EntityRefs, WorkspacePathRefs, Artifacts, and ResourceRefs.

State is not responsible for:

```text
token budgeting
summarization for model input
runtime context injection
prompt formatting
```

Those are runtime/context middleware responsibilities.

### 9.2 EntityRef

EntityRef references a service-owned domain object.

Examples:

```text
sandbox.workspace
sandbox.process
browser.tab
mail.thread
calendar.event
approval.request
```

EntityRef is not a content object.

Example:

```text
EntityRef
  service_id
  type
  id
  display_name
  metadata_json
```

### 9.3 WorkspacePathRef

WorkspacePathRef references a path inside a sandbox workspace.

Example:

```text
WorkspacePathRef
  service_id
  workspace_id
  path
  kind: file | directory | unknown
```

A WorkspacePathRef is not globally readable. It is meaningful only through the owning `sandbox-service`.

### 9.4 Artifact

Artifact represents an execution-relevant output.

An Artifact may point to a WorkspacePathRef and may optionally have a ResourceRef after promotion/export.

Example:

```text
Artifact
  id
  workspace_path_ref
  resource_ref optional
  producer_service_id
  run_id
  tool_call_id
  artifact_type
  visibility
  created_at
  metadata_json
```

### 9.5 ResourceRef

ResourceRef references a promoted content object crossing a service boundary.

Examples:

```text
user upload
mail attachment
browser download
browser screenshot
sandbox promoted artifact
document output
large persisted tool result
```

A ResourceRef is not every workspace file.

Example:

```text
ResourceRef
  id
  authority
  uri
  name
  mime_type
  size
  content_hash
  owner_type
  owner_id
  metadata_json
```

### 9.6 Summary

```text
State            = service-exposed current view
EntityRef        = service domain object reference
WorkspacePathRef = sandbox-internal path reference
Artifact         = execution output semantics
ResourceRef      = promoted cross-service content handle
```

---

## 10. Workspace Files, Artifacts, and Resource Promotion

A workspace may contain millions of files. Acorn must not create global resources or events for every file.

Examples that must not flood the control plane:

```text
npm install
node_modules
.venv
site-packages
build cache
monorepo checkout
large generated temporary trees
```

### 10.1 Workspace files are internal by default

Workspace files remain `sandbox-service` internal paths unless explicitly promoted.

The control plane should not maintain a full workspace file index.

### 10.2 Explicit promotion

Promotion is the action of turning a workspace path into an Artifact and, when needed, a ResourceRef.

Preferred internal API:

```text
CreateArtifactFromPath
```

Alternative naming:

```text
PromoteWorkspacePath
```

Preferred agent tool wrapper:

```text
sandbox.present_files
```

The tool wrapper is model-facing. The authoritative implementation is a sandbox-service control/domain API.

### 10.3 Promotion flow

```text
/workspace/outputs/report.pdf
  -> sandbox.present_files([...]) or CreateArtifactFromPath(...)
  -> Artifact created
  -> optional ResourceRef created
  -> artifact.created event emitted
  -> control plane records metadata
```

### 10.4 Promotion is not file discovery

Acorn should not automatically promote every new file.

For large operations:

```text
npm install
  -> workspace.changed event
  -> no per-file resources
  -> no per-file events
```

For final outputs:

```text
generate_report.py
  -> /workspace/outputs/report.pdf
  -> present_files
  -> Artifact + ResourceRef
```

### 10.5 Resource Catalog scope

The control-plane Resource Catalog stores metadata for promoted content objects.

It is not a workspace filesystem index.

---

## 11. Resource Boundary

ResourceRef is the cross-service content boundary.

### 11.1 User upload

```text
user upload
  -> ResourceRef(authority=resource-store or upload-service)
  -> sandbox-service ImportResource
  -> WorkspacePathRef(/uploads/file)
```

### 11.2 Browser download

```text
browser download
  -> ResourceRef(authority=browser-service)
  -> sandbox-service ImportResource if needed
  -> WorkspacePathRef
```

### 11.3 Sandbox output

```text
WorkspacePathRef(/outputs/report.pdf)
  -> CreateArtifactFromPath / ExportResource
  -> Artifact
  -> ResourceRef(authority=sandbox-service)
```

### 11.4 Workspace migration

Workspace migration should be explicit.

```text
workspace snapshot export
  -> ResourceRef
  -> import into another sandbox-service authority
```

This is migration, not shared ownership.

---

## 12. Signal, Event, State, and Resource Boundary

### 12.1 State

State is service pull.

It is current service status exposed to control plane/runtime.

Examples:

```text
workspace summary
mounted backend profile
recent artifacts
running process summary
browser active tab
mail unread summary
```

### 12.2 Event

Event is observation.

Events are used for UI, audit, logs, metrics, tracing, replay, and debugging.

Events do not automatically enter model context.

Examples:

```text
tool.call.started
tool.call.completed
workspace.changed
artifact.created
resource.imported
policy.denied
```

Events should be coarse-grained. Do not emit one event per workspace file change.

### 12.3 Signal

Signal is attention-oriented push.

A Signal is a service-submitted fact that the control plane/runtime may need to consider.

Examples:

```text
sandbox.artifact.created
sandbox.process.failed
browser.download.completed
mail.thread.needs_attention
approval.requested
```

Not every Event is a Signal.

### 12.4 Resource

Resource is promoted content.

A Signal or State item may refer to a ResourceRef, but ResourceRef has its own authority, access rules, and lifecycle.

---

## 13. Permissions and Governance

Acorn uses:

```text
Central Policy, Distributed Enforcement
```

### 13.1 Control Plane responsibilities

The control plane is the source of policy configuration and audit metadata.

It manages:

```text
tenant/user/session/run policy
tool profiles
resource policy
workspace policy
sandbox profile selection
approval mode
credential scope
policy version
global audit metadata
```

### 13.2 Runtime / Tool Router responsibilities

The runtime or tool router performs preflight and user experience orchestration.

It may:

```text
hide unavailable tools from the model
preflight risky tool calls
create approval requests
wait for approval resolution
inject execution context
record run-level events
```

Runtime preflight is not the final security boundary.

### 13.3 Capability Service responsibilities

A Capability Service performs authoritative local enforcement.

A service must verify:

```text
caller identity
execution context token
session/workspace binding
resource ownership
path boundary
action permission
approval grants
credential scope
network policy
service-specific safety rules
```

Services must not blindly trust bare metadata such as `user_id` or `approval: true`.

### 13.4 ExecutionContextToken

Requests crossing into a service should carry a signed or otherwise authenticated execution context.

Recommended fields:

```text
ExecutionContextToken
  tenant_id
  user_id
  session_id
  run_id
  tool_call_id
  workspace_id
  service_id
  sandbox_profile_id optional
  allowed_actions
  resource_scope
  approval_grants
  expires_at
  policy_version
  trace_id
  signature
```

The service validates the token and still applies local domain checks.

### 13.5 Policy vocabulary

Policy actions should use a shared vocabulary.

Examples:

```text
workspace.create
workspace.read
workspace.write
workspace.exec
workspace.export
workspace.import
resource.read
resource.write
resource.export
resource.import
sandbox.network
sandbox.process.spawn
credential.use
mail.read
mail.send
browser.navigate
message.send
```

The control plane defines policy. Services enforce it.

---

## 14. Sandbox Capability Contract

`sandbox-service` implements the Sandbox Capability Contract.

It is a long-running service, not a single sandbox instance.

It owns:

```text
workspaces
sandbox backend profiles
sandbox instances
workspace paths
processes
artifacts
resource import/export
sandbox state
sandbox signals/events
```

### 14.1 Standard control features

Phase 1 should prioritize:

```text
DescribeCapabilities
CreateWorkspace
DestroyWorkspace
ImportResource
CreateArtifactFromPath
ExportResource
GetState
```

Optional/future:

```text
AcquireSandbox
ReleaseSandbox
SnapshotWorkspace
ForkWorkspace
RollbackWorkspace
```

### 14.2 Standard MCP tools

Phase 1 agent-facing tools may include:

```text
sandbox.exec
sandbox.read_file
sandbox.write_file
sandbox.list_dir
sandbox.search_files
sandbox.present_files
```

These tools should use existing execution context. They should not require the model to pass raw service routing details.

### 14.3 Backend profiles

A sandbox-service may expose multiple backend profiles.

Examples:

```text
local-process
local-docker
cloud-standard
cloud-gpu
remote-provider
```

The control plane selects a profile based on policy, user/session configuration, availability, and task needs.

### 14.4 Workspace creation flow

```text
1. Session created
2. Control plane selects sandbox-service and sandbox_profile_id
3. Control plane calls CreateWorkspace(session_id, sandbox_profile_id)
4. sandbox-service creates persistent workspace
5. control plane stores session -> workspace binding
```

### 14.5 Tool invocation flow

```text
1. Agent/runtime calls sandbox MCP tool
2. Runtime injects ExecutionContextToken
3. sandbox-service validates context
4. sandbox-service resolves workspace binding
5. sandbox-service acquires or reuses sandbox instance
6. tool executes against session workspace
7. service emits events/signals and returns result
```

---

## 15. Built-in Runtime Capabilities

Not every capability should become a service.

Built-in Runtime Capabilities are runtime-native and may maintain state inside the Agent Runtime.

Examples:

```text
todo
plan
summary
memory
scratchpad
heartbeat
context bookmarks
standing orders
```

A capability should remain built-in if it:

```text
is small
is runtime-native
does not require independent deployment
has simple state
has no complex external SDK or lifecycle
```

A capability should become a service if it has:

```text
complex lifecycle
external credentials
webhooks
file resources
separate permissions
local/cloud/enterprise deployment needs
strong isolation requirements
```

---

## 16. Future Agent Runtime Compatibility

Although Phase 1 does not implement the full Agent Runtime, APIs should remain compatible with one.

Future Agent Runtime may include:

```text
Session Manager
Run Manager
Turn Manager
Model Router
Tool Router
Context Engine
Middleware Pipeline
SubRun Manager
Policy/Approval Orchestrator
Event Stream
```

Services must not depend on the future runtime implementation.

They should only depend on stable Acorn-native contracts:

```text
ExecutionContext
ToolCall
ToolResult
State
Signal
Event
Artifact
ResourceRef
PolicyDecision
```

---

## 17. Model Context Plane

The Model Context Plane belongs to the future Agent Runtime.

Services provide inputs. Runtime middleware decides how those inputs become model context.

Candidate context sources:

```text
user messages
service state
service signals
tool results
artifacts
resources
summary store
memory store
runtime-native todo/plan state
heartbeat/reminder sources
```

Context middleware may:

```text
filter
rank
compress
summarize
deduplicate
redact
render
inject
```

These are runtime responsibilities. Capability Services should not inject prompt messages directly.

---

## 18. Standard Service Layout

A Capability Service should use a consistent outer layout.

```text
services/<capability>-service/
  cmd/
    <capability>-service/
      main.go

  configs/
    config.yaml

  internal/
    app/
      app.go

    conf/
      config.go

    server/
      http.go
      grpc.go
      mcp.go

    surface/
      agent/
      control/
      state/
      signal/
      observation/
      resource/
      governance/

    domain/
      ... service-specific domain objects ...

    descriptor/
      descriptor.go

    service/
      health.go

    version/
      version.go
```

`server/` contains transport setup.

`surface/` contains external-facing service surfaces.

`domain/` contains service-specific domain runtime code.

`descriptor/` exposes the runtime Capability Descriptor.

There should be no authoritative static `manifest/` package in Phase 1.

---

## 19. Contract Testing Principles

Contract tests should exist at multiple levels.

### 19.1 Generic service contracts

All services may be tested for:

```text
health/readiness
DescribeCapabilities response
surface registration
MCP tool exposure when applicable
state response shape
signal shape
event shape
resource reference shape
governance hook shape
```

### 19.2 Sandbox contract tests

Sandbox services should be tested for:

```text
workspace creation
session-workspace binding
backend profile declaration
resource import
artifact creation from path
resource export
MCP tool invocation with execution context
state exposure
coarse-grained events
service-side authorization failure
```

### 19.3 Future runtime context contracts

When Agent Runtime is added, test:

```text
collecting signals
querying state
normalizing context items
applying middleware
assembling model input
recording context decisions
routing tool calls
approval orchestration
```

---

## 20. Compatibility With Mature Agent Frameworks

Acorn's abstractions are designed to cover patterns from mature agent frameworks without copying their implementation shape.

### 20.1 DeepAgents-style harnesses

DeepAgents-style middleware maps to future Agent Runtime middleware, not capability services.

Examples:

```text
filesystem middleware -> sandbox-service tools + workspace APIs
skills middleware     -> future skill package/runtime capability
subagent middleware   -> future SubRun Manager
summarization         -> future Context Engine
human-in-the-loop     -> Governance/Approval
```

### 20.2 DeerFlow-style thread/workspace systems

DeerFlow-style thread state maps to:

```text
Session
Workspace
State Surface
Artifacts
Uploads/ResourceRefs
Runtime middleware
```

Explicit file presentation maps to Acorn's `present_files` / `CreateArtifactFromPath` model.

### 20.3 OpenClaw-style gateway systems

Gateway/channel/device abstractions should become future Gateway/Ingress services, not generic Signals.

```text
Inbound message -> Ingress/Trigger
Important fact  -> Signal
Observed event   -> Event
```

### 20.4 Hermes-style product systems

Hermes-style toolsets, credentials, approvals, memory plugins, cron jobs, and gateways map to:

```text
Tool profiles
Policy/approval
Credential scopes
Runtime-native memory/context sources
Automation service, future
Gateway services, future
Capability services
```

---

## 21. Non-Goals

Acorn should not:

```text
force every tool into a service
force every service to implement every surface
make MCP the only internal protocol
expose all control operations as MCP tools
make workspace a global shared filesystem
register every workspace file as ResourceRef
emit per-file events for large directory changes
use static manifests as authoritative truth
require full Agent Runtime in Phase 1
let services directly mutate model messages
let runtime preflight replace service-side authorization
let external MCP tools automatically participate in State/Signal/Resource without adapters
```

---

## 22. Phase 1 Vertical Slice

The first meaningful vertical slice should be:

```text
1. sandbox-service starts
2. control plane calls DescribeCapabilities
3. control plane records descriptor
4. new session is created
5. control plane selects sandbox-service + sandbox_profile_id
6. control plane calls CreateWorkspace
7. control plane stores session -> workspace binding
8. user upload becomes ResourceRef
9. control plane asks sandbox-service ImportResource
10. agent/runtime calls sandbox MCP tool, e.g. sandbox.exec
11. sandbox-service executes in session workspace
12. output file is created inside workspace
13. agent calls sandbox.present_files or system calls CreateArtifactFromPath
14. sandbox-service creates Artifact and ResourceRef
15. control plane records resource metadata
16. event log shows workspace/resource/artifact/tool events
17. GetState shows current workspace/artifact state
```

This slice validates:

```text
Capability Descriptor
Control Surface
MCP Agent Surface
Session workspace binding
Resource import
Artifact promotion
Resource metadata
State Surface
Event/Signal boundary
Policy placeholder
Service-side authorization
```

---

## 23. Glossary

**Agent Surface**
Model-facing action surface. MCP is preferred.

**Artifact**
Execution-relevant output, usually created from a workspace path. It may become a ResourceRef after promotion/export.

**Capability Descriptor**
Runtime description of a service's actual implemented surfaces, endpoints, tools, features, and backend profiles.

**Capability Service**
An independently deployable service that owns one capability domain.

**Control Surface**
Acorn-native API used by the control plane, UI, admin tooling, and management systems.

**EntityRef**
Reference to a service-owned domain object such as a workspace, process, browser tab, or mail thread.

**Event**
Observation record for UI, audit, logs, metrics, tracing, and replay.

**ExecutionContextToken**
Authenticated context passed to services for tool/control calls. It binds tenant, user, session, run, workspace, permissions, policy version, and trace information.

**MCP**
Preferred model-facing protocol for agent tools. It is an adapter protocol, not Acorn's sole internal protocol.

**ResourceRef**
Reference to a promoted, content-bearing, readable or transferable object crossing a service boundary.

**Sandbox Backend/Profile**
Selectable execution implementation behind `sandbox-service`, such as local process, Docker, VM, cloud sandbox, or remote provider.

**Sandbox Instance**
Concrete execution environment managed by `sandbox-service`. It may be disposable and may mount a persistent workspace.

**Session**
A user conversation or continuous task context. In Phase 1 it owns one primary workspace.

**Signal**
Attention-oriented pushed fact from a service or runtime.

**State**
Service-exposed current view. It is not model context by itself.

**Workspace**
Persistent file state owned by `sandbox-service`. It is not a global shared filesystem.

**WorkspacePathRef**
Reference to a path inside a sandbox workspace. It is not globally readable and is not a ResourceRef by default.

---

## 24. Core Rule Summary

```text
1. Acorn Phase 1 is a capability substrate, not a full agent runtime.
2. Complex capabilities are independent Capability Services.
3. Runtime-native capabilities may remain built in.
4. Services expose surfaces; internals remain service-specific code.
5. Static manifests should be removed as authoritative truth.
6. Services expose runtime Capability Descriptors.
7. MCP is the preferred agent-facing tool protocol, not the internal control protocol.
8. Acorn-native APIs own control, state, resource, signal, event, and governance.
9. One session owns one primary workspace in Phase 1.
10. sandbox-service is not a sandbox instance; it may manage many workspaces, instances, and backend profiles.
11. Workspace files are not ResourceRefs by default.
12. ResourceRef means promoted cross-service content object.
13. Artifact promotion is explicit through CreateArtifactFromPath / present_files.
14. State is a service-exposed current view, not runtime context.
15. Runtime/context middleware owns filtering, summarization, and injection.
16. Events are observation; Signals are attention-oriented facts.
17. Control Plane defines policy; Capability Services enforce policy locally.
18. Runtime/tool router performs preflight and approval orchestration, but is not the final security boundary.
19. Workspace is owned by sandbox-service, not the whole platform.
20. Configuration selects implementations; code implements behavior.
```
