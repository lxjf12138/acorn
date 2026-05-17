package sandboxpolicy

type Scope struct {
	TenantID string
	UserID   string
}

type Policy struct {
	Scope Scope

	DefaultProfileID  string
	AllowedProfileIDs []string
}

type ResolveWorkspaceProfileRequest struct {
	TenantID string
	UserID   string

	RequestedProfileID string

	AvailableProfileIDs map[string]struct{}
}

type ResolveWorkspaceProfileResult struct {
	ProfileID string
	Source    string
}
