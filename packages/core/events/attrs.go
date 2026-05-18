package events

const (
	AttrServiceID = "acorn.service.id"

	AttrResourceAuthorityServiceID = "acorn.resource.authority_service_id"
	AttrResourceMimeType           = "acorn.resource.mime_type"
	AttrResourceSizeBytes          = "acorn.resource.size_bytes"

	AttrSandboxProfileID = "acorn.sandbox.profile_id"
	AttrSandboxBackendID = "acorn.sandbox.backend_id"

	AttrExecCommandName     = "acorn.exec.command_name"
	AttrExecArgCount        = "acorn.exec.arg_count"
	AttrExecExitCode        = "acorn.exec.exit_code"
	AttrExecTimedOut        = "acorn.exec.timed_out"
	AttrExecStdoutTruncated = "acorn.exec.stdout_truncated"
	AttrExecStderrTruncated = "acorn.exec.stderr_truncated"
)
