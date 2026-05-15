package servicekit

// BuildInfo contains service identity and build metadata shared by Kratos
// services.
type BuildInfo struct {
	Name    string
	Version string
}
