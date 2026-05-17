package telemetry

const (
	AttrOperation = "acorn.operation"
	AttrStatus    = "acorn.status"

	AttrServiceID = "acorn.service.id"

	AttrWorkspaceID        = "acorn.workspace.id"
	AttrWorkspaceProfileID = "acorn.workspace.profile_id"
	AttrWorkspacePathKind  = "acorn.workspace.path.kind"

	AttrResourceAuthorityServiceID = "acorn.resource.authority_service_id"
	AttrResourceMimeType           = "acorn.resource.mime_type"
	AttrResourceSizeBytes          = "acorn.resource.size_bytes"

	AttrSandboxProfileID = "acorn.sandbox.profile_id"
	AttrSandboxBackendID = "acorn.sandbox.backend_id"

	AttrExecExitCode        = "acorn.exec.exit_code"
	AttrExecTimedOut        = "acorn.exec.timed_out"
	AttrExecStdoutTruncated = "acorn.exec.stdout_truncated"
	AttrExecStderrTruncated = "acorn.exec.stderr_truncated"
	AttrExecCommandName     = "acorn.exec.command_name"
	AttrExecArgCount        = "acorn.exec.arg_count"

	AttrLeaseMode   = "acorn.workspace.lease.mode"
	AttrLeaseReason = "acorn.workspace.lease.reason"

	AttrTruncated = "acorn.truncated"
)
