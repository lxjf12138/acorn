package backend

import (
	"time"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
)

type SandboxLease struct {
	ID          string
	BackendID   string
	WorkspaceID string

	Attachment *attachment.WorkspaceAttachment

	CreatedAt time.Time
	Metadata  map[string]string
}
