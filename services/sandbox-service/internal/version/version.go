package version

var (
	ServiceName = "sandbox-service"
	Version     = "dev"
	Commit      = "unknown"
	BuildTime   = "unknown"
)

type Info struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

func GetInfo() Info {
	return Info{
		Service:   ServiceName,
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
	}
}
