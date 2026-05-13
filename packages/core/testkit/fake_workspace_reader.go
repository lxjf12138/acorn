package testkit

import (
	"context"
	"sort"
	"sync"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	workspacecore "github.com/lxjf12138/acorn/packages/core/workspace"
	"google.golang.org/protobuf/proto"
)

type fakeFile struct {
	info    *workspacev1.FileInfo
	content []byte
}

type FakeWorkspaceReader struct {
	mu         sync.RWMutex
	workspaces map[string]*workspacev1.Workspace
	files      map[string]map[string]*fakeFile
}

func NewFakeWorkspaceReader() *FakeWorkspaceReader {
	return &FakeWorkspaceReader{
		workspaces: make(map[string]*workspacev1.Workspace),
		files:      make(map[string]map[string]*fakeFile),
	}
}

func (f *FakeWorkspaceReader) AddWorkspace(workspace *workspacev1.Workspace) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.workspaces[workspace.GetId()] = proto.Clone(workspace).(*workspacev1.Workspace)
	if _, ok := f.files[workspace.GetId()]; !ok {
		f.files[workspace.GetId()] = make(map[string]*fakeFile)
	}
}

func (f *FakeWorkspaceReader) AddFile(workspaceID, path string, content []byte, info *workspacev1.FileInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[workspaceID]; !ok {
		f.files[workspaceID] = make(map[string]*fakeFile)
	}
	fileInfo := proto.Clone(info).(*workspacev1.FileInfo)
	fileInfo.Path = path
	f.files[workspaceID][path] = &fakeFile{
		info:    fileInfo,
		content: append([]byte(nil), content...),
	}
}

func (f *FakeWorkspaceReader) GetWorkspace(_ context.Context, workspaceID string) (*workspacev1.Workspace, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	workspace, ok := f.workspaces[workspaceID]
	if !ok {
		return nil, workspacecore.ErrWorkspaceNotFound
	}
	return proto.Clone(workspace).(*workspacev1.Workspace), nil
}

func (f *FakeWorkspaceReader) ListFiles(_ context.Context, workspaceID string, path string) ([]*workspacev1.FileInfo, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	files, ok := f.files[workspaceID]
	if !ok {
		return nil, workspacecore.ErrWorkspaceNotFound
	}
	var out []*workspacev1.FileInfo
	for filePath, file := range files {
		if path == "" || path == "." || filePath == path {
			out = append(out, proto.Clone(file.info).(*workspacev1.FileInfo))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].GetPath() < out[j].GetPath()
	})
	return out, nil
}

func (f *FakeWorkspaceReader) ReadFile(_ context.Context, workspaceID string, path string) ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	files, ok := f.files[workspaceID]
	if !ok {
		return nil, workspacecore.ErrWorkspaceNotFound
	}
	file, ok := files[path]
	if !ok {
		return nil, workspacecore.ErrWorkspaceNotFound
	}
	return append([]byte(nil), file.content...), nil
}

var _ workspacecore.Reader = (*FakeWorkspaceReader)(nil)
