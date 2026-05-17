package attachment

type Kind string

const (
	KindLocalPath       Kind = "local_path"
	KindDockerBind      Kind = "docker_bind"
	KindRemotePath      Kind = "remote_path"
	KindRemoteWorkspace Kind = "remote_workspace"
)

type WorkspaceAttachment struct {
	ID          string
	WorkspaceID string
	Kind        Kind

	LocalPath         string
	GuestPath         string
	RemoteWorkspaceID string

	ReadOnly bool
	Metadata map[string]string
}

type TargetKind string

const (
	TargetLocalProcess TargetKind = "local_process"
	TargetDocker       TargetKind = "docker"
	TargetVM           TargetKind = "vm"
	TargetRemoteAgent  TargetKind = "remote_agent"
)

type Target struct {
	Kind      TargetKind
	BackendID string
	ReadOnly  bool
	Metadata  map[string]string
}
