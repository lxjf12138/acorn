package event

import "time"

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

const (
	TypeResourceUploaded              = "resource.uploaded"
	TypeResourceImportedToWorkspace   = "resource.imported_to_workspace"
	TypeResourceExportedFromWorkspace = "resource.exported_from_workspace"

	TypeWorkspaceCreated       = "workspace.created"
	TypeWorkspaceExecCompleted = "workspace.exec.completed"
	TypeWorkspaceExecFailed    = "workspace.exec.failed"
)

type EventRecord struct {
	ID string

	Type     string
	Severity Severity

	OccurredAt time.Time

	ServiceID string

	TenantID    string
	UserID      string
	SessionID   string
	WorkspaceID string
	ResourceID  string

	TraceID string
	SpanID  string

	Subject EventSubject
	Actor   EventActor

	PayloadJSON []byte
}

type EventActor struct {
	Type string
	ID   string
}

type EventSubject struct {
	Type string
	ID   string
}
