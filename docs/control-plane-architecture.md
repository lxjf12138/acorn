# Agent Control Plane Architecture

The Agent Control Plane is the relationship, policy, gateway, and runtime-state
coordinator for Acorn.

It does not own sandbox workspace files. It does not execute commands directly.
It does not implement sandbox isolation. It does not replace OpenTelemetry. It
coordinates sessions, workspace hosts, resources, sandbox profiles, execution
records, and capability-service access.

## Internal Layers

```text
Agent Control Plane

API Layer
  HTTP API
  future gRPC API
  future MCP / agent adapter

Application Services
  WorkspaceService
  ResourceService
  ResourceGatewayService
  UploadService
  ExecutionService
  SandboxPolicyResolver

Domain Records
  WorkspaceRecord
  ResourceRecord
  ExecutionRecord
  future RunRecord

Gateways / Clients
  Sandbox Workspace Client
  Sandbox Resource Client
  Sandbox Exec Client
  Capability Descriptor Client

Stores
  WorkspaceRecordStore
  ResourceRecordStore
  ExecutionRecordStore
  future RunRecordStore
  ResourceBlobStore

Cross-cutting
  OpenTelemetry traces
  OpenTelemetry metrics
  OpenTelemetry log-based events
  Acorn telemetry semantics
```

The Control Plane is a coordinator, not a capability implementation. Capability
Services own domain execution and domain state; the Control Plane owns
relationships, policy decisions, routing, and runtime state records.

## Responsibilities

Control Plane responsibilities:

1. Bind sessions to hosted workspaces.
2. Track ResourceRecords and resource ownership metadata.
3. Route resource content through authority services.
4. Select sandbox profiles through SandboxPolicy.
5. Validate selected profiles against capability descriptors.
6. Forward workspace view/import/export/exec requests to sandbox-service.
7. Record workspace exec attempts as ExecutionRecords.
8. Emit Acorn telemetry semantics through OpenTelemetry.
9. Provide user-facing APIs for resource, workspace, and runtime state.

## Non-Goals

Control Plane non-goals:

1. Does not own workspace file contents.
2. Does not mount workspace directories.
3. Does not directly execute commands.
4. Does not enforce OS sandbox isolation.
5. Does not store live workspace trees.
6. Does not act as a workflow engine.
7. Does not provide a durable EventStore in Phase 1.
8. Does not replace OpenTelemetry traces, metrics, or logs/events.

## Domain Records

### WorkspaceRecord

WorkspaceRecord is the binding between a Control Plane session and a
capability-service hosted workspace.

It records:

```text
session_id
current_host.service_id
current_host.service_workspace_id
current_host.sandbox_profile_id
```

WorkspaceRecord does not contain file content. It does not contain a workspace
root path. It does not own the workspace. Workspace files are managed by the
sandbox-service WorkspaceStore.

### ResourceRecord

ResourceRecord is Control Plane catalog metadata for a ResourceRef.

It records:

```text
resource id
authority service id
mime type
size bytes
status
owner / scope metadata
```

ResourceRecord is metadata. Resource bytes live in a ResourceBlobStore or in
the authority service that owns the ResourceRef. ResourceRecordStore and
ResourceBlobStore are separate responsibilities.

### ExecutionRecord

ExecutionRecord is implemented Phase 1 Control Plane runtime state. It
represents one synchronous workspace exec attempt.

It records:

```text
execution_id
session_id
workspace binding
sandbox service id
sandbox profile id
command basename
arg count
status
exit_code
stdout/stderr size
stdout/stderr truncated flags
trace_id
span_id
timestamps
```

It does not record:

```text
full command args
env
stdout/stderr content
workspace path
file content
secret/token
```

ExecutionRecord is state. It is not telemetry. It is not an Event. It is not an
Agent Run. It correlates with OpenTelemetry through trace_id and span_id.

### Future RunRecord

RunRecord is a planned near-term object, not an implemented Phase 1 object. It
will represent one user or agent task attempt and can group multiple
ExecutionRecords.

```text
Session
  -> RunRecord
      -> ExecutionRecord
```

RunRecord will not initially be:

```text
workflow engine
async scheduler
graph runtime
checkpoint store
retry engine
event stream
```

## Telemetry Boundary

OpenTelemetry is the telemetry substrate. Kratos integrates transport
middleware. servicekit initializes providers and wiring. Acorn defines domain
telemetry semantics.

ExecutionRecord is runtime state and stores trace_id/span_id for correlation.
Log-based Acorn events are telemetry semantics carried by OpenTelemetry Logs.
They are not persisted in an Acorn EventStore in Phase 1.

## Run Model Guardrails From Reference Frameworks

Acorn borrows ID separation, state discipline, and store boundaries from
mature agent and automation systems. It does not copy workflow engines or graph
runtimes into Phase 1.

### Codex

Borrow:

```text
Thread / job / job item separation
explicit job/item status
guarded state transitions
```

Avoid:

```text
full rollout/history/replay model
collapsing session/run/execution into one object
```

### DeepAgents

Borrow:

```text
backend/middleware separation
result object discipline
```

Avoid:

```text
turning Minimal Run into a graph/checkpoint runtime
```

### Deer Flow

Borrow:

```text
RunManager / RunStore conceptual separation
explicit multitask strategy
```

Avoid:

```text
async manager
streaming
cancel / interrupt / rollback
checkpointing
```

### Hermes

Borrow:

```text
session-level concurrency control
stable execution/tool ids
```

Avoid:

```text
prompt queue / steering in Minimal Run
```

### OpenClaw

Borrow:

```text
small RunStatus set
explicit rejection of unsupported options
```

Avoid:

```text
broad runtime/workspace/approval options before implementation
```

