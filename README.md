# Acorn

A distributed capability runtime for AI agents.

## What Is This?

Acorn connects a Cloud Agent Control Plane with many Capability Nodes.

The Cloud Agent Control Plane runs agent sessions, model calls, context building,
tool routing, signal routing, trigger rules, registry, policy, and audit.

Capability Nodes provide local, cloud, and enterprise capabilities through MCP
tools, signals, control APIs, observation events, resource surfaces, and provider
runtimes.

## Core Architecture

- Cloud Agent Control Plane
- Capability Node
- Capability Provider
- MCP Agent Surface
- Signal Surface
- Control Surface
- Observation Surface
- Governance Surface
- Resource Surface

## Current Stage

This repository is in bootstrap stage.

Step 0 focuses on:

- repository skeleton
- architecture decisions
- shared contracts
- service boundaries

No production runtime is implemented yet.

## Repository Layout

```text
api/proto/                     Future protobuf contracts.
docs/                          Architecture, glossary, roadmap, and ADRs.
services/agent-control-plane/  Cloud-side agent runtime service shell.
services/capability-node/      Deployable capability provider runtime shell.
packages/core/                 Framework-independent domain contracts.
packages/mcp/                  MCP integration package.
packages/signal/               Signal protocol package.
packages/provider-sdk-go/      Go SDK for native Capability Providers.
scripts/                       Development and repository scripts.
```

## Development Plan

The initial engineering plan is documented in `docs/roadmap.md`.

The first milestone establishes repository shape, architecture decisions, and
shared terminology before adding Go workspace files, protobuf contracts, or
runtime implementation.

## Background Documents

- `docs/capability_node_product_analysis.md`
- `docs/capability_node_architecture_design.md`
