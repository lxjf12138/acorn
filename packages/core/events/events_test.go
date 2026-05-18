package events

import "testing"

func TestEventNamesAreNonEmpty(t *testing.T) {
	names := []string{
		ResourceUploaded,
		ResourceImportedToWorkspace,
		ResourceExportedFromWorkspace,
		WorkspaceCreated,
		WorkspaceExecCompleted,
		WorkspaceExecFailed,
	}
	for _, name := range names {
		if name == "" {
			t.Fatal("event name must be non-empty")
		}
	}
}

func TestSeverityValuesAreNonEmpty(t *testing.T) {
	values := []Severity{SeverityInfo, SeverityWarning, SeverityError}
	for _, value := range values {
		if value == "" {
			t.Fatal("severity must be non-empty")
		}
	}
}
