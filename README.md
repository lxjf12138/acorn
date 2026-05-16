# Acorn

Acorn is a Phase 1 capability substrate for future AI agent runtimes.

The system is organized around:

- Cloud Agent Control Plane
- independently deployable Capability Services
- runtime Capability Descriptors
- sandbox profiles and backends
- MCP agent-surface adapters
- Acorn-native control/state/resource/signal/event/governance APIs

A Capability Service exposes a runtime Capability Descriptor describing:

- Agent Surface
- Control Surface
- Signal Surface
- State Surface
- Observation Surface
- Resource Surface
- Governance Surface
- sandbox profiles, when applicable

The first concrete Capability Service is `sandbox-service`.

Sandbox workspace is owned by `sandbox-service`; it is not a global shared filesystem. Workspace files become `ResourceRef` only after explicit artifact/resource promotion.

See [`docs/architecture.md`](docs/architecture.md).
