# Acorn

Acorn is a Phase 1 capability substrate for future AI agent runtimes.

The system is organized around:

- Cloud Agent Control Plane
- independently deployable Capability Services
- runtime Capability Descriptors
- sandbox profiles and backends
- MCP agent-surface adapters
- Acorn-native control/state/view/resource/signal/governance APIs
- OpenTelemetry-based telemetry semantics

A Capability Service exposes a runtime Capability Descriptor describing:

- Agent Surface
- Control Surface
- State Surface
- View Surface
- Resource Surface
- Signal Surface
- Telemetry Semantics
- Governance Surface
- sandbox profiles, when applicable
- implemented endpoints and transport addresses

The first concrete Capability Service is `sandbox-service`.

Control Plane owns session and workspace records. `sandbox-service` owns hosted workspaces and their internal files; they are not a global shared filesystem.

Workspace files are not `ResourceRef`s by default. Users may browse or preview them through the Control Plane via the service's View Surface. Browse and preview are temporary and do not create `ResourceRef`s. Explicit export, download, upload, or cross-service transfer uses `ResourceRef`.

Descriptors are generated from the running service and its configuration; they are not static source-of-truth manifests.

See [`docs/architecture.md`](docs/architecture.md).
