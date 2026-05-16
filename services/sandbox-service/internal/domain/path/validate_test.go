package path

import "testing"

func TestNormalizeWorkspacePathValid(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		allowRoot bool
		want      string
	}{
		{name: "empty root", input: "", allowRoot: true, want: ""},
		{name: "dot root", input: ".", allowRoot: true, want: ""},
		{name: "relative file", input: "./src/main.go", want: "src/main.go"},
		{name: "duplicate slash", input: "src//main.go", want: "src/main.go"},
		{name: "output file", input: "outputs/report.pdf", want: "outputs/report.pdf"},
		{name: "dotfile", input: ".git/config", want: ".git/config"},
		{name: "node module", input: "node_modules/react/index.js", want: "node_modules/react/index.js"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeWorkspacePath(tt.input, tt.allowRoot)
			if err != nil {
				t.Fatalf("NormalizeWorkspacePath returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeWorkspacePath = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeWorkspacePathInvalid(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		allowRoot bool
	}{
		{name: "empty non root", input: ""},
		{name: "dot non root", input: "."},
		{name: "slash root", input: "/", allowRoot: true},
		{name: "absolute", input: "/workspace/a.txt"},
		{name: "parent", input: "../secret"},
		{name: "middle parent", input: "a/../secret"},
		{name: "deep parent", input: "a/../../secret"},
		{name: "nul", input: "a/\x00/b"},
		{name: "windows drive", input: `C:\temp\file.txt`},
		{name: "unc", input: `\\server\share`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := NormalizeWorkspacePath(tt.input, tt.allowRoot); err == nil {
				t.Fatalf("NormalizeWorkspacePath returned %q, expected error", got)
			}
		})
	}
}
