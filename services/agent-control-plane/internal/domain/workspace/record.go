package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
)

// Record is the control-plane workspace binding for one session. It does not
// own workspace files or sandbox instance lifecycle.
type Record struct {
	ID           string
	SessionID    string
	OwnerUserID  string
	Status       workspacev1.WorkspaceStatus
	CurrentHost  *workspacev1.WorkspaceHostRef
	CreatedAt    time.Time
	UpdatedAt    time.Time
	MetadataJSON []byte
}

func NewRecordID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "cp_ws_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("cp_ws_%d", time.Now().UnixNano())
}
