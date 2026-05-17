package backend

import "time"

type ExecRequest struct {
	Command string
	Args    []string
	CWD     string
	Env     map[string]string

	Timeout        time.Duration
	MaxStdoutBytes int64
	MaxStderrBytes int64
}

type ExecResult struct {
	ExitCode int

	Stdout []byte
	Stderr []byte

	StdoutTruncated bool
	StderrTruncated bool

	ErrorMessage string
}
