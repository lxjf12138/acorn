# Acorn

Acorn is a service-oriented runtime architecture for AI agent capabilities.

The system is organized around:

- Cloud Agent Control Plane
- independently deployable Capability Services
- built-in runtime capabilities
- external MCP tools and adapters

A Capability Service may expose:

- Agent Surface
- Control Surface
- Signal Surface
- State Surface
- Observation Surface
- Resource Surface
- Governance Surface

The first concrete Capability Service is `sandbox-service`.

Sandbox workspace is owned by `sandbox-service`; it is not a global shared filesystem. Cross-service files flow through `ResourceRef`.

See [`docs/acorn_architecture.md`](docs/acorn_architecture.md).
