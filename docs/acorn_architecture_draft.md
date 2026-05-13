# Acorn Architecture Draft

> Status: Draft  
> Scope: Current architecture direction after the latest design discussion  
> Purpose: Establish the service boundary before continuing implementation

---

## 1. Summary

Acorn is a service-oriented runtime for AI agent capabilities.

The system is composed of:

```text
Cloud Agent Control Plane
+
Independently deployable Capability Services
+
Optional Local Node Agent / Connector
```

A **Capability Service** is an independent Kratos service that provides one capability domain, such as sandbox, browser, mail, enterprise tools, or MCP adapter.

Each Capability Service may expose:

```text
Agent Surface       -> MCP
Control Surface     -> HTTP/gRPC
Observation Surface -> Events, logs, status, metrics
Resource Surface    -> ResourceRef, import/export/read operations
Runtime             -> capability-specific state and execution logic
```

The key architectural rule is:

```text
Complex runtime capabilities are independent services.
They are not plugins loaded into one generic capability-node process.
```

---

## 2. Core Architecture

```text
┌───────────────────────────────────────────────────────────┐
│                 Cloud Agent Control Plane                 │
│                                                           │
│  Agent Runtime                                            │
│  Session Manager                                          │
│  Model Router                                             │
│  Tool Router                                              │
│  Signal Router                                            │
│  Capability Service Registry                              │
│  Resource Store / Object Store                            │
│  Policy / Audit Metadata                                  │
└───────────────────────────┬───────────────────────────────┘
                            │
                            │ MCP / HTTP / gRPC / Events
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
┌───────▼────────┐  ┌───────▼────────┐  ┌───────▼────────────┐
│ sandbox-service│  │ browser-service│  │ enterprise-service │
│ Kratos Service │  │ Kratos Service │  │ Kratos Service     │
└────────────────┘  └────────────────┘  └────────────────────┘
```

Future optional local deployment may include:

```text
local-node-agent
  ├─ local service discovery
  ├─ cloud connector
  ├─ local health aggregation
  ├─ event forwarding
  └─ local UI / status
```

The local node agent is not the owner of sandbox/browser/mail runtime by default. It is a connector/supervisor, not a provider host.

---

## 3. Main Components

### 3.1 Cloud Agent Control Plane

The Cloud Agent Control Plane is responsible for:

```text
agent sessions
model calls
context building
tool routing
signal routing
capability service registry
resource store / object store
resource metadata
global policy metadata
audit metadata
user-facing cloud APIs
```

It does **not** own capability-specific runtime state such as:

```text
sandbox workspace
browser tabs
mail sync state
enterprise tool internal state
```

Those belong to the corresponding Capability Service.

---

### 3.2 Capability Service

A Capability Service is an independent deployable Kratos service.

Examples:

```text
sandbox-service
browser-service
mail-service
enterprise-tool-service
mcp-adapter-service
document-service
```

A Capability Service may expose:

```text
MCP Agent Surface
HTTP/gRPC Control Surface
Event / Observation Surface
Resource Surface
Policy / Audit hooks
```

Each service owns its own runtime complexity.

For example:

```text
sandbox-service owns:
  sandbox workspace
  environments
  process execution
  file import/export
  artifacts

browser-service owns:
  browser process
  tabs
  profiles
  screenshots
  downloads
  browser events

mail-service owns:
  accounts
  sync state
  messages
  attachments
  drafts
```

---

### 3.3 Optional Local Node Agent

A Local Node Agent may be introduced later.

Its purpose is:

```text
local capability service discovery
connection to cloud control plane
health aggregation
event forwarding
local authentication
local UI support
```

It should not be treated as a generic runtime host for all providers.

The difference is:

```text
Wrong:
  local-node-agent hosts sandbox/browser/mail providers internally

Preferred:
  local-node-agent discovers and connects sandbox-service/browser-service/mail-service
```

---

## 4. Service Surface Model

Every Capability Service follows the same conceptual surface model.

### 4.1 Agent Surface

The Agent Surface is for model-driven tool calls.

Default protocol:

```text
MCP
```

Examples:

```text
sandbox.bash
sandbox.read_file
browser.open
browser.act
mail.search
mail.send
```

The Agent Surface should be simple and suitable for LLM tool calling.

---

### 4.2 Control Surface

The Control Surface is for UI, operators, and the Control Plane.

Default protocols:

```text
HTTP
gRPC
```

Examples:

```text
sandbox create/restart
sandbox import resource
sandbox export file
browser list tabs
browser close tab
mail account status
provider health
```

The Control Surface is not the same as MCP.

---

### 4.3 Observation Surface

The Observation Surface exposes runtime state.

Examples:

```text
events
logs
stdout/stderr
process status
browser console logs
browser network events
resource created
artifact discovered
```

Observation events are not automatically agent inputs.

---

### 4.4 Signal Surface

A Signal is an external fact that may enter the agent runtime.

Examples:

```text
mail.message.received
browser.download.completed
sandbox.process.exited
webhook.received
schedule.tick
```

Signals are processed by the Control Plane's Signal Router and may later trigger:

```text
UI notification
context insertion
agent wakeup
tool call
approval request
```

Signal is not a tool call.

---

### 4.5 Resource Surface

The Resource Surface exposes files or file-like objects through `ResourceRef`.

Examples:

```text
user upload
browser download
mail attachment
document processing output
sandbox exported artifact
external file
node-local resource
```

Resource is the cross-provider file/object abstraction.

Resource is not Workspace.

---

## 5. Resource and Workspace Boundary

This is one of the most important architecture rules.

### 5.1 Resource

A Resource is a control-plane or provider-visible object.

It can represent:

```text
uploaded file
browser download
mail attachment
document output
sandbox exported file
external object
```

A Resource can live in:

```text
control-plane object store
node-local resource store
external provider
sandbox export result
```

Resource should be described by `ResourceRef`.

Conceptually:

```text
ResourceRef
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

Examples:

```text
authority = object_store
owner_type = upload
owner_id = upload_123

authority = object_store
owner_type = browser
owner_id = browser_session_456

authority = object_store
owner_type = sandbox
owner_id = sandbox_run_789
```

---

### 5.2 Sandbox Workspace

Workspace is not a top-level Acorn concept.

Workspace belongs to `sandbox-service`.

A sandbox workspace is the sandbox-side physical execution workspace.

Typical structure:

```text
/workspace
/uploads
/outputs
/.metadata
```

A file belongs to the sandbox workspace only after:

```text
1. it has been imported into sandbox
2. it was created by sandbox execution
3. it was part of sandbox creation initial resources
```

A file that exists only in the control-plane object store is **not** a workspace file.

---

### 5.3 Import

Import moves a Resource into sandbox workspace.

```text
Resource Store / Object Store
  -> sandbox-service.ImportResource
  -> Sandbox Workspace
```

Example:

```text
resource://uploads/data.csv
  -> /uploads/data.csv
```

Only after import can the model refer to:

```text
/uploads/data.csv
```

as a workspace path.

---

### 5.4 Export

Export moves a sandbox file into Resource Store.

```text
Sandbox Workspace
  -> sandbox-service.ExportFile
  -> Resource Store / Object Store
```

Example:

```text
/outputs/report.md
  -> resource://artifacts/report.md
```

After export, other services can use it as a Resource.

If the sandbox later modifies `/outputs/report.md`, the exported Resource does not update automatically. A new export is required.

---

### 5.5 Why not shared Workspace?

The system should not allow arbitrary providers to directly share one workspace.

Avoid this model:

```text
browser-service writes directly into sandbox workspace
mail-service writes directly into sandbox workspace
document-service writes directly into sandbox workspace
```

This causes unclear ownership:

```text
Who is authoritative?
Who can write?
How are conflicts handled?
How does sandbox see updates?
How do permissions work?
How is cross-node sync handled?
```

Preferred model:

```text
browser download -> Resource
mail attachment -> Resource
document output -> Resource

Resource -> explicit import -> sandbox workspace
sandbox output -> explicit export -> Resource
```

---

## 6. Model-Visible File Semantics

The model should see simple file paths only when those files are actually present in sandbox workspace.

Examples:

```text
/uploads/data.csv
/workspace/main.py
/outputs/report.md
```

These paths should mean:

```text
The file exists in sandbox-service's workspace.
```

If a user uploads a file but it has not been imported into sandbox yet, the model should not be told that it is a workspace path.

The runtime can handle import automatically before exposing the path to the model.

---

## 7. Capability Service Registry

The Control Plane should register Capability Services, not generic provider-host nodes.

Recommended concept:

```text
CapabilityService
  id
  name
  kind
  deployment_profile
  status
  version
  labels
  registered_at
  last_seen_at
  metadata_json
```

Examples:

```text
id = sandbox.local.1
kind = sandbox
deployment_profile = local

id = browser.cloud.1
kind = browser
deployment_profile = cloud

id = enterprise.crm.1
kind = enterprise_tool
deployment_profile = enterprise
```

This replaces the older ambiguous `Node` concept.

A capability service instance may expose one or more provider manifests, but the service is the deployment/runtime boundary.

---

## 8. Provider Manifest

Provider Manifest remains useful, but its meaning is adjusted.

A Provider Manifest declares what a Capability Service exposes.

For example:

```text
sandbox-service exposes ProviderManifest(id=sandbox, type=sandbox)
browser-service exposes ProviderManifest(id=browser, type=browser)
mcp-adapter-service may expose multiple ProviderManifest entries
```

Provider Manifest describes:

```text
agent tools
signals emitted
control features
observation events
resource types
security metadata
```

It is not necessarily a plugin loaded into one generic capability-node process.

---

## 9. Recommended Repository Structure

Current direction:

```text
services/
  agent-control-plane/
  sandbox-service/

packages/
  api/
  core/
  mcp/
  signal/
  provider-sdk-go/
```

Future services:

```text
services/
  browser-service/
  mail-service/
  local-node-agent/
  mcp-adapter-service/
```

Recommended `sandbox-service` structure:

```text
services/sandbox-service/
  cmd/
    sandbox-service/
      main.go

  configs/
    config.yaml

  internal/
    conf/
    server/
      http.go
      grpc.go
      mcp.go

    service/
      health.go

    sandbox/
      workspace/
      environment/
      process/
      importexport/
      artifact/
      tools/

    surface/
      control/
      observation/
      resource/

    version/
```

Important rule:

```text
sandbox workspace lives under sandbox-service internals.
There should not be packages/core/workspace or api/proto/acorn/workspace/v1.
```

---

## 10. Top-Level Protocol Domains

Recommended top-level proto domains:

```text
common
capability
provider
tool
signal
event
resource
```

Avoid top-level:

```text
workspace
```

Workspace belongs to future:

```text
sandbox.v1
```

only if/when sandbox public APIs require it.

---

## 11. Core Packages

Recommended `packages/core` domains:

```text
capability
provider
tool
signal
event
resource
testkit
```

Avoid:

```text
workspace
node
```

`node` should be replaced by `capability` or `capability service`.

---

## 12. Kratos Usage

Kratos should be used as the service shell for deployable services:

```text
agent-control-plane
sandbox-service
browser-service
mail-service
local-node-agent
```

Kratos is responsible for:

```text
HTTP/gRPC transport
service lifecycle
config loading
logging
middleware
graceful shutdown
```

Kratos should not be forced into domain/core packages.

Good boundary:

```text
services/* use Kratos
packages/core does not depend on Kratos
capability runtime internals avoid unnecessary Kratos coupling
```

---

## 13. MCP Usage

MCP is the default Agent Surface.

Each Capability Service may expose MCP.

For example:

```text
sandbox-service:
  sandbox.bash
  sandbox.read_file
  sandbox.write_file
  sandbox.list_files

browser-service:
  browser.open
  browser.act
  browser.snapshot
```

MCP should not replace:

```text
control API
observation API
resource API
policy/audit API
```

Those remain HTTP/gRPC/Event surfaces.

---

## 14. Security and Governance

Security is cross-cutting.

Each Capability Service should support hooks for:

```text
policy evaluation
approval
audit
permission
credential access logging
resource access logging
```

However, governance APIs do not need to be fully defined at the current stage.

Early rule:

```text
Do not bury safety logic inside only one provider implementation.
Each service should expose enough metadata/events for the Control Plane to review and audit.
```

---

## 15. Current Implementation Direction

Before building agent runtime or sandbox runtime, the repository should first align with this architecture.

Recommended next cleanup:

```text
1. Rename services/capability-node -> services/sandbox-service
2. Remove top-level workspace proto/core contracts
3. Remove common.Scope.workspace_id
4. Rename node concept to capability service
5. Rename ResourceRef.node_id -> service_id
6. Update generated protobuf code
7. Update core/testkit
8. Update Makefile / go.work
9. Add this architecture document
```

After that:

```text
PR next:
  add contract tests for:
    capability service
    provider
    tool
    signal
    event
    resource
```

Then:

```text
parallel work:
  agent-control-plane core
  sandbox-service runtime
```

---

## 16. Design Rules

The following rules should guide future implementation.

### Rule 1

```text
Each complex capability is an independent Kratos Capability Service.
```

### Rule 2

```text
MCP is for agent tool calls.
HTTP/gRPC/Event surfaces are for control, observation, resource, and governance.
```

### Rule 3

```text
Workspace is sandbox-side only.
```

### Rule 4

```text
A file is a sandbox workspace file only after import, sandbox creation, or sandbox execution.
```

### Rule 5

```text
Cross-provider file movement goes through ResourceRef and Resource Store, not shared workspace.
```

### Rule 6

```text
Capability Service is the deployment/runtime boundary.
Provider Manifest is the capability declaration boundary.
```

### Rule 7

```text
Kratos belongs to services, not packages/core.
```

### Rule 8

```text
Do not build a generic capability-node provider host unless a future local-node-agent use case explicitly requires it.
```

---

## 17. Terminology

### Cloud Agent Control Plane

Cloud-side service that runs agent sessions, model calls, routing, registry, signals, and resource metadata.

### Capability Service

An independently deployable Kratos service that provides one capability domain.

Examples:

```text
sandbox-service
browser-service
mail-service
enterprise-tool-service
mcp-adapter-service
```

### Agent Surface

The MCP-facing tool interface exposed to agents.

### Control Surface

HTTP/gRPC APIs for UI and control plane operations.

### Observation Surface

Events, logs, metrics, traces, status updates.

### Resource Surface

ResourceRef-based access to files or file-like objects.

### Resource

Cross-provider file/object abstraction.

### Sandbox Workspace

Sandbox-service internal execution workspace.

### Import

Resource Store to Sandbox Workspace.

### Export

Sandbox Workspace to Resource Store.

### Local Node Agent

Optional future connector/supervisor for local capability services. It is not the default provider runtime host.

---

## 18. Open Questions

These are intentionally left unresolved:

```text
How local-node-agent discovers local capability services
How cloud connector tunnels work
How approval workflows are modeled
How audit records are persisted
How sandbox import/export APIs are exposed
Whether sandbox public API should get acorn.sandbox.v1
How browser-service resource handling should be implemented
How object store is deployed in local-only mode
```

These should not block the current architecture cleanup.

---

## 19. Immediate Next Step

The immediate next implementation step is:

```text
Reframe capability-node into sandbox-service
and align proto/core naming with Capability Service architecture.
```

This must happen before contract tests and before starting sandbox runtime implementation.
