package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
)

// Workspace is sandbox-service-owned persistent workspace state. It is not a
// control-plane WorkspaceRecord and does not expose sandbox instance identity.
type Workspace struct {
	ID               string
	SandboxProfileID string
	DisplayName      string
	Status           workspacev1.WorkspaceStatus
	StoreKind        string
	StoreWorkspaceID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	MetadataJSON     []byte
}

func NewID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "ws_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("ws_%d", time.Now().UnixNano())
}
