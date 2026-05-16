package servicekit

// BuildInfo contains service identity and build metadata shared by Kratos
// services.
type BuildInfo struct {
	ID      string
	Name    string
	Version string
}
