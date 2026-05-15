# Acorn Architecture

> This document defines Acorn's long-term architecture boundaries and invariants.  
> It intentionally does **not** track implementation progress, PR order, or temporary migration notes.

---

## 1. Executive Summary

Acorn is a service-oriented runtime architecture for AI agent capabilities.

The system is organized around:

1. **Cloud Agent Control Plane**
2. **Independent Capability Services**
3. **Built-in Runtime Capabilities**
4. **External MCP Tools and Adapters**

The core idea is:

> Acorn standardizes how capabilities connect to the agent runtime.  
> It does **not** force every capability to share one internal implementation model.

Complex capabilities such as sandbox, browser, mail, document processing, messaging channels, and enterprise tools should usually be implemented as independent Capability Services.

Lightweight runtime-native capabilities such as todos, planning notes, summaries, memory, heartbeats, and context bookmarks can remain built into the Agent Runtime.

External tools can be integrated through MCP or adapters without becoming full Capability Services.

---

## 2. Core Architecture Principles

### 2.1 Service boundary over plugin hosting

Acorn should not assume a single generic `capability-node` process that hosts all capability providers.

Complex capabilities should be implemented as independently deployable services:

```text
services/
  agent-control-plane/
  sandbox-service/
  browser-service/        # future
  mail-service/           # future
  document-service/       # future
  messaging-service/      # future, e.g. wechat/slack/telegram
  mcp-adapter-service/    # future
```

Each service may run locally, in the cloud, in an enterprise network, or at the edge.

### 2.2 Unified surfaces, not unified internals

Acorn standardizes service surfaces, not service internals.

For example, both `sandbox-service` and `browser-service` may expose signal, state, resource, control, and agent surfaces, but their domain runtime is different.

```text
sandbox-service:
  sandbox, workspace, process, artifact, import/export

browser-service:
  browser context, tab, page, download, screenshot, console event

mail-service:
  account, thread, message, attachment, draft
```

### 2.3 Code implements behavior; configuration selects behavior

Implementation differences must be code-level differences, not YAML templates.

Configuration can:

- select an implementation,
- provide parameters,
- enable or disable features,
- set limits,
- declare endpoints,
- configure delivery and retry policies.

Configuration must not attempt to define how a local Linux sandbox, Windows sandbox, cloud sandbox, browser controller, or messaging connector works internally.

### 2.4 Workspace is not a global shared filesystem

Workspace is not a top-level Acorn shared filesystem.

For sandbox:

> A workspace is a `sandbox-service` domain object.

A workspace is persistent file state managed by `sandbox-service`. A sandbox may mount a workspace for execution. A workspace may survive sandbox restart or rebuild.

Cross-service file exchange must use `ResourceRef`, not direct workspace sharing.

### 2.5 Services provide facts and state; runtime builds model context

Capability Services and built-in capabilities should not directly write model messages.

They provide:

- **Signals**: pushed text facts and resource references.
- **State**: pullable current state summaries.
- **Resources**: cross-service files, attachments, and artifacts.

The Agent Runtime owns model context assembly.

---

## 3. System-Level Architecture

```text
Cloud Agent Control Plane
  ├─ Agent Runtime
  ├─ Session Manager
  ├─ Tool Router
  ├─ Signal Ingress / Inbox
  ├─ Context Engine
  ├─ Capability Service Registry
  ├─ Resource Store / Object Store
  ├─ Resource Metadata
  ├─ Policy / Audit Metadata
  └─ UI / Control APIs

Capability Services
  ├─ sandbox-service
  ├─ browser-service
  ├─ mail-service
  ├─ messaging-service
  ├─ document-service
  ├─ enterprise-tool-service
  └─ mcp-adapter-service

Built-in Runtime Capabilities
  ├─ todo
  ├─ plan
  ├─ summary
  ├─ memory
  ├─ heartbeat
  └─ context bookmarks

External Tools
  ├─ MCP tools
  ├─ HTTP tool adapters
  └─ enterprise-specific adapters
```

The Control Plane coordinates sessions, models, tools, routing, resources, policy, and context construction.

Capability Services own their own domain state and expose capabilities through common surfaces.

Built-in Runtime Capabilities can maintain state inside the Agent Runtime and participate in context construction without becoming services.

---

## 4. Capability Service Pattern

A Capability Service is an independently deployable service that owns one capability domain.

```text
Capability Service
  ├─ Agent Surface
  ├─ Control Surface
  ├─ Signal Surface
  ├─ State Surface
  ├─ Observation Surface
  ├─ Resource Surface
  ├─ Governance Surface
  └─ Domain Runtime
```

Not every Capability Service must implement every surface. Surfaces are optional capabilities declared by the service manifest.

### 4.1 Agent Surface

The Agent Surface is used by the model or agent runtime for model-initiated actions.

The preferred protocol is MCP.

Examples:

```text
sandbox.exec
sandbox.read_file
browser.open
browser.click
mail.search
wechat.send_message
document.parse
```

The Agent Surface should hide routing details from the model. The model should not need to know service IDs, workspace authority, object-store routing, or file transfer mechanics.

### 4.2 Control Surface

The Control Surface is used by the control plane, UI, admin tooling, or local management.

Preferred protocols are HTTP and gRPC.

Examples:

```text
create sandbox
list workspaces
inspect process
bind messaging account
list browser tabs
configure webhook
inspect delivery status
```

Control APIs are not the primary model-facing API.

### 4.3 Signal Surface

The Signal Surface is service push.

A Signal is a service- or runtime-submitted text fact that may be considered by the Agent Runtime.

Examples:

```text
wechat.message.received
sandbox.cpu.high
sandbox.process.exited
sandbox.artifact.discovered
browser.download.completed
browser.console.error
todo.updated
resource.available
context.summary.generated
runtime.heartbeat
```

A Signal should be text-first. Large files and attachments should be represented by `ResourceRef` and referenced by the Signal.

A service may emit Signals through HTTP, gRPC, a stream, a queue, an outbox forwarder, or another delivery mechanism. Delivery mechanism is not the Signal itself.

### 4.4 State Surface

The State Surface is runtime pull.

The Agent Runtime may ask a service for current model-relevant state summaries while building context.

Examples:

```text
sandbox workspace summary
sandbox recent process summary
sandbox artifact list
browser current page summary
browser recent downloads
wechat conversation summary
todo open items
pending approval summary
```

State Surface calls are not MCP tool calls. They are internal runtime queries used for context construction.

The response should be text-first and budget-aware.

### 4.5 Observation Surface

The Observation Surface exposes service runtime telemetry for UI, debugging, audit, metrics, and tracing.

Examples:

```text
run.started
run.completed
tool.call.started
tool.call.completed
sandbox.stdout
sandbox.stderr
message.delivery.failed
resource.imported
context.compacted
```

Observation Events do not automatically enter model context. They may be used by policies or context middleware, but their default role is observability.

### 4.6 Resource Surface

The Resource Surface handles cross-service resources.

Examples:

```text
user upload
browser download
mail attachment
wechat image or file
sandbox export
document processing output
large tool result
```

Resources are represented by `ResourceRef`.

### 4.7 Governance Surface

The Governance Surface handles policy, approval, audit, permissions, and risk boundaries.

Examples:

```text
approval before sending a message
approval before running a dangerous shell command
recipient allowlist
file export permission
network access policy
malicious plugin defense
audit records
```

Governance is cross-cutting, but each service may need domain-specific checks.

### 4.8 Domain Runtime

The Domain Runtime is service-specific code.

It owns the service's internal objects, lifecycle, and implementation.

Examples:

```text
sandbox-service:
  sandbox, workspace, process, artifact, import/export

browser-service:
  browser context, page, tab, download, screenshot

messaging-service:
  account, conversation, message, attachment, delivery
```

Domain Runtime is not standardized into a generic YAML template.

---

## 5. Built-in Runtime Capabilities

Not every capability should become a service.

Built-in Runtime Capabilities are runtime-native capabilities that can maintain state inside the Agent Runtime.

Examples:

```text
todo
plan
summary
memory
scratchpad
heartbeat
context bookmarks
short-lived reasoning notes
```

Rules:

1. Built-in capabilities may maintain state.
2. If a built-in capability wants to affect model context, it should expose that state through a Context Source.
3. Built-in capabilities do not need Agent/Control/Signal/State/Resource service surfaces.
4. Built-in capabilities can have runtime-side middleware when appropriate.

Example:

```text
todo.update tool
  -> updates runtime todo state
  -> Todo Context Source reads todo state
  -> Context Middleware decides whether/how to inject a reminder
```

A capability should remain built-in if it is small, runtime-native, does not require independent deployment, and has simple state.

A capability should become a service if it has complex lifecycle, external SDKs, webhooks, file resources, separate permissions, local/cloud/enterprise deployment needs, or strong isolation requirements.

---

## 6. External MCP Tools and Adapters

External MCP tools may provide model-initiated tool calls without becoming Capability Services.

An MCP tool integration gives the model a callable tool and a tool result path.

It does not automatically provide:

- Signal Surface,
- State Surface,
- Resource Surface,
- Control Surface,
- Governance Surface.

If an external tool needs to push model-relevant facts, expose current state, manage resources, or participate in governance, it should be wrapped by an adapter or implemented as a Capability Service.

---

## 7. Signal, State, Event, and Resource

### 7.1 Signal

Signal is service or runtime push.

A Signal is a text-first fact that may be considered by the Agent Runtime.

Recommended v0 shape:

```text
Signal
  id
  type
  source_service_id
  provider_id
  subject
  text
  resource_refs
  severity
  dedupe_key
  occurred_at
  expires_at
  metadata_json
```

The core protocol should not enumerate every signal type. Types should be namespaced strings.

Examples:

```text
wechat.message.received
sandbox.cpu.high
sandbox.process.exited
browser.download.completed
resource.available
runtime.heartbeat
```

Signal should not carry large files. Use `ResourceRef`.

### 7.2 State

State is runtime pull.

A service may expose current model-relevant state through a State Surface.

Recommended v0 shape:

```text
GetState(request)
  scope
  subject
  reason
  budget

StateItem
  type
  text
  resource_refs
  priority
  metadata_json
```

Example reasons:

```text
run_start
before_model
after_tool
after_compaction
resume_from_idle
manual_refresh
```

State responses should be text-first. Metadata is optional and should not be required for default rendering.

### 7.3 Event

Event is observation.

Events are for UI, audit, logs, metrics, tracing, and debugging.

Events do not automatically become model context.

### 7.4 ResourceRef

ResourceRef is the cross-service resource reference.

A ResourceRef may be referenced by Signal or State items.

Signal/State text should describe the resource in model-readable form. The ResourceRef carries the machine-readable reference.

---

## 8. Model Context Plane

The Model Context Plane is owned by the Agent Runtime.

Capability Services and built-in tools do not directly write model messages.

They provide:

- Signals,
- State items,
- ResourceRefs,
- Tool results,
- Runtime state.

The Agent Runtime Context Engine turns these inputs into model messages.

```text
Context Sources
  ├─ Signal Inbox
  ├─ Service State Surface
  ├─ Built-in Runtime State
  ├─ Resource Bindings
  ├─ Tool Results
  ├─ Summary Store
  ├─ Memory Store
  └─ Heartbeat / Reminder Sources

Context Engine
  ├─ collect
  ├─ normalize
  ├─ deduplicate
  ├─ filter
  ├─ prioritize
  ├─ budget / compress
  ├─ render
  └─ audit

Model Input
  ├─ system messages
  ├─ developer messages
  ├─ user messages
  ├─ tool messages
  ├─ resource blocks
  ├─ image/file parts
  ├─ summaries
  └─ reminders
```

### 8.1 Context Source

A Context Source provides candidate context.

Sources may come from:

```text
service signals
service state
built-in runtime state
resources
tool results
summary
memory
attachments
workspace bindings
heartbeat
```

### 8.2 Context Item

A Context Item is a normalized candidate model-visible item.

Recommended v0 shape:

```text
ContextItem
  id
  type
  source
  text
  resource_refs
  priority
  metadata_json
```

### 8.3 Context Middleware

Context Middleware processes Context Items.

It may:

- deduplicate,
- filter,
- sort,
- compress,
- enforce budget,
- render resource references,
- apply safety or governance checks,
- decide whether to include or suppress items.

Context Middleware is a runtime implementation mechanism. It is not a service integration protocol.

Services should not provide arbitrary runtime middleware. Services provide Signal and State. The runtime middleware pipeline handles them uniformly.

### 8.4 Context Assembler

The Context Assembler renders Context Items into final model input.

It is the single owner of final message formatting.

Context Sources should not directly write model messages.

### 8.5 Mature framework compatibility

This model can represent common mechanisms seen in mature agent runtimes:

```text
uploads injection:
  ResourceRef / State -> Context Item -> model-visible file list

todo reminder:
  Built-in or service state -> Context Item -> reminder

view image injection:
  Tool result / ResourceRef -> Context Item -> image or resource block

summary:
  Runtime summary store -> Context Item -> summary message

large result reference:
  Tool result -> ResourceRef -> Context Item -> model-visible reference

heartbeat:
  Runtime timer -> internal Signal -> Context Item -> prompt

sandbox status:
  sandbox-service Signal/State -> Context Item -> model-visible status

browser status:
  browser-service Signal/State -> Context Item -> model-visible status
```

---

## 9. Capability Class Contracts

Capability Service Pattern defines how services connect.

Capability Class Contracts define what a class of capability means.

Examples:

```text
Sandbox Capability Contract
Browser Capability Contract
Messaging Capability Contract
Mail Capability Contract
Document Capability Contract
```

The contract should define common domain concepts and standard signals/state/resource types for a capability class.

The contract should not prescribe one internal implementation.

---

## 10. Sandbox Capability Contract

`sandbox-service` implements the Sandbox Capability Contract.

It is a long-running Capability Service. It is not a single sandbox instance.

It may manage:

- zero or more sandboxes,
- zero or more workspaces,
- processes,
- artifacts,
- resource import/export,
- sandbox-related signals and state.

### 10.1 Sandbox

A sandbox is an execution context managed by `sandbox-service`.

A sandbox may mount a workspace, run processes, produce artifacts, and emit state changes.

A sandbox is not a Kratos service by itself.

### 10.2 Workspace

A workspace is persistent file state managed by `sandbox-service`.

A workspace may be mounted by a sandbox for execution.

A workspace is not a global shared filesystem and is not directly shared across services.

### 10.3 Process

A process is an observable execution unit inside a sandbox.

The underlying implementation may be local, cloud, Windows, Linux, container-based, VM-backed, or remote. The contract should preserve common semantics such as command, status, stdout/stderr, exit code, start time, finish time, and cancellation.

### 10.4 Artifact

An artifact is a user-relevant output discovered in a workspace.

Artifacts become cross-service resources only after explicit export.

### 10.5 Import / Export

```text
Import:
  ResourceRef -> workspace file

Export:
  workspace file -> ResourceRef
```

### 10.6 Sandbox Signals

Examples:

```text
sandbox.process.started
sandbox.process.exited
sandbox.cpu.high
sandbox.memory.high
sandbox.artifact.discovered
sandbox.resource.imported
sandbox.resource.exported
```

### 10.7 Sandbox State

Examples:

```text
workspace summary
current sandbox status
recent process summary
artifact list
resource import/export summary
pending approval summary
```

---

## 11. Resource and Workspace Boundary

Resource and workspace are different concepts.

```text
ResourceRef:
  cross-service object reference

Workspace:
  sandbox-service domain file state
```

Examples:

### User upload

```text
user upload
  -> ResourceRef(owner_type=upload)
  -> sandbox-service import
  -> workspace:/uploads/file
```

### Browser download

```text
browser-service download
  -> ResourceRef(owner_type=browser_download)
  -> sandbox-service import if needed
  -> workspace file
```

### Sandbox output

```text
workspace:/outputs/report.md
  -> sandbox-service export
  -> ResourceRef(owner_type=sandbox_artifact)
```

Workspace migration can be implemented by exporting a workspace snapshot resource and importing it into another sandbox-service authority.

This is explicit migration, not shared ownership.

---

## 12. Standard Service Layout

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
      signal/
      state/
      observation/
      resource/
      governance/

    domain/
      ... service-specific domain objects ...

    manifest/
      manifest.go

    service/
      health.go

    version/
      version.go
```

`server/` contains transport setup.

`surface/` contains external-facing service surfaces.

`domain/` contains service-specific domain runtime concepts.

`manifest/` declares what the service provides.

The outer layout should be shared. The domain content should remain service-specific.

---

## 13. Manifest and Registration

A Capability Service should declare:

- service ID,
- name,
- kind,
- contract,
- version,
- deployment profile,
- supported surfaces,
- features,
- resource types,
- signal types,
- state capabilities,
- governance requirements.

Example:

```yaml
id: sandbox.local.dev
name: sandbox-service
kind: sandbox
contract: acorn.sandbox
version: dev

surfaces:
  agent:
    protocol: mcp
  control:
    protocols: [grpc, http]
  signal:
    enabled: true
  state:
    enabled: true
  observation:
    enabled: true
  resource:
    enabled: true
  governance:
    enabled: true

features:
  - sandboxes
  - workspaces
  - process_execution
  - resource_import
  - resource_export
  - artifact_discovery
```

Manifest is declaration, not implementation.

---

## 14. Kratos and Shared Service Infrastructure

Kratos is appropriate for long-running Capability Services and the Cloud Agent Control Plane.

Shared service infrastructure may live in `packages/servicekit`.

`servicekit` may provide:

- logger setup,
- build info,
- default middleware,
- server helpers,
- health helpers.

Rules:

```text
packages/core must not depend on Kratos.
packages/api must not depend on servicekit.
packages/servicekit may depend on Kratos.
Capability Services may depend on servicekit.
```

`servicekit` must not become a business framework or plugin host.

---

## 15. Configuration vs Code Implementation

Configuration can select and parameterize code implementations.

Configuration must not define implementation behavior from scratch.

Examples:

```text
Correct:
  sandbox-service config selects local implementation.
  browser-service config selects remote browser endpoint.
  messaging-service config sets webhook secret.

Incorrect:
  config describes how process isolation works.
  config defines browser runtime semantics.
  config defines messaging protocol behavior.
```

Complex runtime behavior belongs in code.

---

## 16. Contract Testing Principles

Contract tests should exist at multiple levels.

### 16.1 Generic service contracts

All services may be tested for:

- manifest validity,
- health/readiness,
- surface declarations,
- signal emission shape,
- state response shape,
- ResourceRef shape,
- governance hooks where applicable.

### 16.2 Capability class contracts

Sandbox services should be tested against Sandbox Contract concepts.

Browser services should be tested against Browser Contract concepts.

Messaging services should be tested against Messaging Contract concepts.

### 16.3 Runtime context contracts

Agent Runtime should be tested for:

- collecting Signals,
- querying State Surfaces,
- reading built-in runtime state,
- normalizing Context Items,
- applying middleware,
- assembling model input,
- recording context decisions.

---

## 17. Non-Goals

Acorn should not:

- force every tool into a service,
- force every service to implement every surface,
- expose all control operations as MCP tools,
- make workspace a global shared filesystem,
- use YAML templates to define complex runtime behavior,
- require the Agent Runtime to contain service-specific middleware for every external service,
- let services directly mutate model messages,
- let external MCP tools automatically participate in Signal/State unless wrapped or adapted.

---

## 18. Glossary

**Agent Surface**  
Model-facing action surface, usually MCP.

**Capability Service**  
An independently deployable service that owns one capability domain.

**Built-in Runtime Capability**  
A runtime-native capability such as todo, summary, memory, heartbeat, or plan. It may maintain state without becoming a service.

**Signal**  
A pushed text fact from a service or runtime that may be considered by the Agent Runtime.

**State Surface**  
A service interface used by the Agent Runtime to query current model-relevant state summaries.

**Observation Event**  
A runtime/service event used for UI, audit, logs, metrics, tracing, or debugging. It does not automatically enter model context.

**ResourceRef**  
A cross-service reference to a file, attachment, artifact, large tool result, or other resource.

**Context Source**  
Any source of candidate model context: signal inbox, service state, built-in runtime state, resource binding, tool result, summary, memory, or heartbeat.

**Context Item**  
A normalized candidate model-visible item.

**Context Engine**  
The Agent Runtime component that turns context sources into final model input.

**Context Middleware**  
Runtime pipeline used to filter, deduplicate, rank, compress, transform, and render context items.

**Context Assembler**  
The component that renders Context Items into final model messages and model input parts.

**Workspace**  
Sandbox-service-owned persistent file state. It is not a global shared filesystem.

**Sandbox**  
An execution context managed by sandbox-service. It may mount a workspace.

---

## 19. Core Rule Summary

```text
1. Complex capabilities are independent Capability Services.
2. Lightweight runtime-native capabilities may be built in.
3. External MCP tools do not automatically become services.
4. Services expose surfaces; internals remain service-specific code.
5. Signal is service/runtime push.
6. State is runtime pull.
7. Observation is for UI/audit/logs by default.
8. ResourceRef is the cross-service file/object boundary.
9. Workspace belongs to sandbox-service, not the whole platform.
10. Agent Runtime owns model context assembly.
11. Services provide facts/state/resources; runtime decides final model context.
12. Middleware is runtime context processing, not service integration.
13. Configuration selects implementations; code implements behavior.
14. Capability Class Contracts define shared semantics for a capability class.
15. Do not track PR progress in architecture documents.
```
