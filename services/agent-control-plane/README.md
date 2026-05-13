# Agent Control Plane

This service is the cloud-side control plane for agent runtime.

Responsibilities:

- agent sessions
- model routing
- context building
- tool routing
- signal routing
- trigger rules
- capability registry
- node registry
- global policy and audit
- cloud control APIs

Non-goals:

- does not execute local tools directly
- does not own provider runtime
- does not manage sandbox filesystem directly
