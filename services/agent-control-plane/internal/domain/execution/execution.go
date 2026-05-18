package execution

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type Status string

const (
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusTimeout   Status = "timeout"
	StatusCanceled  Status = "canceled"
)

type ExecutionRecord struct {
	ID string

	TenantID  string
	UserID    string
	SessionID string

	WorkspaceID        string
	ServiceWorkspaceID string

	SandboxServiceID string
	SandboxProfileID string
	SandboxBackendID string

	Status Status

	CommandName string
	ArgCount    int
	CWDSet      bool

	ExitCode     int32
	ErrorCode    string
	ErrorMessage string

	StdoutSizeBytes int64
	StderrSizeBytes int64
	StdoutTruncated bool
	StderrTruncated bool

	TraceID string
	SpanID  string

	StartedAt   time.Time
	CompletedAt time.Time
	UpdatedAt   time.Time
}

func NewRecordID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "exec_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("exec_%d", time.Now().UnixNano())
}
