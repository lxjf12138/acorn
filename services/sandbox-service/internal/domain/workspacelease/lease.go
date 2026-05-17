package workspacelease

import "time"

type Mode string

const (
	ModeRead  Mode = "read"
	ModeWrite Mode = "write"
)

type Lease struct {
	ID          string
	WorkspaceID string
	Mode        Mode

	Holder string
	Reason string

	AcquiredAt time.Time
	Metadata   map[string]string
}

func (l *Lease) Clone() *Lease {
	if l == nil {
		return nil
	}
	out := *l
	if l.Metadata != nil {
		out.Metadata = make(map[string]string, len(l.Metadata))
		for key, value := range l.Metadata {
			out.Metadata[key] = value
		}
	}
	return &out
}
