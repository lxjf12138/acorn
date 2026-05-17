package profile

const LocalProcessDevID = "local-process-dev"

type Capability string

const (
	CapabilityWorkspaceView     Capability = "workspace_view"
	CapabilityWorkspaceResource Capability = "workspace_resource"
	CapabilityWorkspaceExec     Capability = "workspace_exec"
	CapabilityLocalProcessExec  Capability = "local_process_exec"
)

type IsolationClass string

const (
	IsolationDevProcess IsolationClass = "dev_process"
	IsolationProcess    IsolationClass = "process"
	IsolationContainer  IsolationClass = "container"
	IsolationVM         IsolationClass = "vm"
)

type Profile struct {
	ID          string
	DisplayName string
	Description string

	Enabled bool
	Default bool

	IsolationClass IsolationClass

	WorkspaceStoreKind string
	AttachmentKind     string
	BackendID          string

	Capabilities []Capability

	Metadata map[string]string
}

func (p *Profile) Clone() *Profile {
	if p == nil {
		return nil
	}
	out := *p
	out.Capabilities = append([]Capability(nil), p.Capabilities...)
	if p.Metadata != nil {
		out.Metadata = make(map[string]string, len(p.Metadata))
		for key, value := range p.Metadata {
			out.Metadata[key] = value
		}
	}
	return &out
}

func (p *Profile) HasCapability(capability Capability) bool {
	if p == nil {
		return false
	}
	for _, candidate := range p.Capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}
