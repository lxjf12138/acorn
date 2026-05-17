package telemetry

import "testing"

func TestSafeCommandName(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "simple", command: "go", want: "go"},
		{name: "unix path", command: "/usr/bin/go", want: "go"},
		{name: "windows path", command: `C:\Go\bin\go.exe`, want: "go.exe"},
		{name: "empty", command: "", want: ""},
		{name: "dot", command: ".", want: ""},
		{name: "dot dot", command: "..", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SafeCommandName(tt.command); got != tt.want {
				t.Fatalf("SafeCommandName(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}
