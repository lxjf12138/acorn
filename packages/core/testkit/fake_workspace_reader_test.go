package testkit

import (
	"context"
	"testing"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
)

func TestFakeWorkspaceReaderReadOnly(t *testing.T) {
	reader := NewFakeWorkspaceReader()
	reader.AddWorkspace(&workspacev1.Workspace{Id: "ws-1"})
	reader.AddFile("ws-1", "a.txt", []byte("hello"), &workspacev1.FileInfo{
		Path: "a.txt",
		Size: 5,
	})

	files, err := reader.ListFiles(context.Background(), "ws-1", "")
	if err != nil {
		t.Fatalf("ListFiles returned error: %v", err)
	}
	if len(files) != 1 || files[0].GetPath() != "a.txt" {
		t.Fatalf("unexpected file listing: %+v", files)
	}

	content, err := reader.ReadFile(context.Background(), "ws-1", "a.txt")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if got := string(content); got != "hello" {
		t.Fatalf("unexpected file content: %q", got)
	}
}
