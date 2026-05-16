package server

// MCP transport placeholder.
//
// MCP is the agent-facing adapter for model-initiated sandbox actions.
//
// It operates inside an existing execution context. It does not create
// workspaces or select sandbox profiles by itself.
//
// HTTP and gRPC remain the Acorn-native transports for control, state,
// resource, observation, and governance surfaces.
//
// The descriptor declares the MCP agent surface, but this service does not yet
// import an MCP SDK or start an MCP server.
