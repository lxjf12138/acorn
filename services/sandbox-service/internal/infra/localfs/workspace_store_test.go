package localfs

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
)

func TestWorkspaceStoreCreateBackingWorkspace(t *testing.T) {
	store := newTestWorkspaceStore(t)
	backing, err := store.CreateBackingWorkspace(context.Background(), workspacestore.CreateBackingWorkspaceRequest{
		WorkspaceID:      "ws-test",
		SandboxProfileID: "local-process",
	})
	if err != nil {
		t.Fatalf("CreateBackingWorkspace returned error: %v", err)
	}
	if backing.StoreKind != Kind || backing.StoreWorkspaceID != "ws-test" {
		t.Fatalf("unexpected backing workspace: %+v", backing)
	}
	if _, err := os.Stat(filepath.Join(store.baseDir, "ws-test")); err != nil {
		t.Fatalf("workspace root was not created: %v", err)
	}
}

func TestWorkspaceStoreListDir(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	writeFile(t, store.baseDir, "ws-test/b.txt", "bee")
	writeFile(t, store.baseDir, "ws-test/a.txt", "aye")
	if err := os.Mkdir(filepath.Join(store.baseDir, "ws-test/src"), 0o700); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}

	listing, err := store.ListDir(context.Background(), workspacestore.ListDirRequest{
		WorkspaceID: "ws-test",
		PageSize:    2,
	})
	if err != nil {
		t.Fatalf("ListDir returned error: %v", err)
	}
	if listing.Directory.Path != "" || listing.Directory.Kind != sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY {
		t.Fatalf("unexpected directory: %+v", listing.Directory)
	}
	if got := pathInfoNames(listing.Entries); strings.Join(got, ",") != "a.txt,b.txt" {
		t.Fatalf("unexpected entries: %v", got)
	}
	if listing.NextToken == "" {
		t.Fatal("expected next token")
	}

	second, err := store.ListDir(context.Background(), workspacestore.ListDirRequest{
		WorkspaceID: "ws-test",
		PageSize:    2,
		PageToken:   listing.NextToken,
	})
	if err != nil {
		t.Fatalf("ListDir second page returned error: %v", err)
	}
	if got := pathInfoNames(second.Entries); strings.Join(got, ",") != "src" {
		t.Fatalf("unexpected second page: %v", got)
	}
	if second.NextToken != "" {
		t.Fatalf("unexpected next token: %q", second.NextToken)
	}
}

func TestWorkspaceStoreListDirErrors(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	writeFile(t, store.baseDir, "ws-test/file.txt", "content")

	tests := []struct {
		name string
		req  workspacestore.ListDirRequest
		err  error
	}{
		{name: "missing workspace id", req: workspacestore.ListDirRequest{}, err: workspacestore.ErrWorkspaceNotFound},
		{name: "missing path", req: workspacestore.ListDirRequest{WorkspaceID: "ws-test", Path: "missing"}, err: workspacestore.ErrPathNotFound},
		{name: "file path", req: workspacestore.ListDirRequest{WorkspaceID: "ws-test", Path: "file.txt"}, err: workspacestore.ErrPathNotDirectory},
		{name: "traversal", req: workspacestore.ListDirRequest{WorkspaceID: "ws-test", Path: "../secret"}, err: workspacestore.ErrInvalidPath},
		{name: "bad page token", req: workspacestore.ListDirRequest{WorkspaceID: "ws-test", PageToken: "bad"}, err: workspacestore.ErrInvalidPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.ListDir(context.Background(), tt.req)
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
}

func TestWorkspaceStorePreviewFile(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	writeFile(t, store.baseDir, "ws-test/report.txt", "hello world")

	preview, err := store.PreviewFile(context.Background(), workspacestore.PreviewFileRequest{
		WorkspaceID: "ws-test",
		Path:        "report.txt",
		MaxBytes:    5,
	})
	if err != nil {
		t.Fatalf("PreviewFile returned error: %v", err)
	}
	if preview.File.Path != "report.txt" || string(preview.Bytes) != "hello" || !preview.Truncated {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if preview.File.SizeBytes != int64(len("hello world")) || preview.MimeType == "" || preview.File.ModifiedAt.IsZero() {
		t.Fatalf("missing preview metadata: %+v", preview)
	}
}

func TestWorkspaceStorePreviewFileErrors(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	if err := os.Mkdir(filepath.Join(store.baseDir, "ws-test/dir"), 0o700); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	writeFile(t, store.baseDir, "ws-test/file.txt", "content")

	tests := []struct {
		name string
		req  workspacestore.PreviewFileRequest
		err  error
	}{
		{name: "missing workspace id", req: workspacestore.PreviewFileRequest{Path: "file.txt"}, err: workspacestore.ErrWorkspaceNotFound},
		{name: "missing file", req: workspacestore.PreviewFileRequest{WorkspaceID: "ws-test", Path: "missing.txt"}, err: workspacestore.ErrPathNotFound},
		{name: "directory", req: workspacestore.PreviewFileRequest{WorkspaceID: "ws-test", Path: "dir"}, err: workspacestore.ErrPathIsDirectory},
		{name: "traversal", req: workspacestore.PreviewFileRequest{WorkspaceID: "ws-test", Path: "../secret"}, err: workspacestore.ErrInvalidPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.PreviewFile(context.Background(), tt.req)
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
}

func TestWorkspaceStoreExportPath(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	writeFile(t, store.baseDir, "ws-test/outputs/report.txt", "report")

	exported, err := store.ExportPath(context.Background(), workspacestore.ExportPathRequest{
		WorkspaceID: "ws-test",
		Path:        "outputs/report.txt",
	})
	if err != nil {
		t.Fatalf("ExportPath returned error: %v", err)
	}
	if exported.Source.Path != "outputs/report.txt" || exported.SizeBytes != int64(len("report")) || exported.MimeType == "" {
		t.Fatalf("unexpected exported path: %+v", exported)
	}
	reader, err := exported.Open(context.Background())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read exported content: %v", err)
	}
	if string(body) != "report" {
		t.Fatalf("unexpected exported content: %q", string(body))
	}
}

func TestWorkspaceStoreExportPathErrors(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	if err := os.Mkdir(filepath.Join(store.baseDir, "ws-test/dir"), 0o700); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}

	tests := []struct {
		name string
		req  workspacestore.ExportPathRequest
		err  error
	}{
		{name: "missing workspace id", req: workspacestore.ExportPathRequest{Path: "file.txt"}, err: workspacestore.ErrWorkspaceNotFound},
		{name: "missing file", req: workspacestore.ExportPathRequest{WorkspaceID: "ws-test", Path: "missing.txt"}, err: workspacestore.ErrPathNotFound},
		{name: "directory", req: workspacestore.ExportPathRequest{WorkspaceID: "ws-test", Path: "dir"}, err: workspacestore.ErrPathIsDirectory},
		{name: "traversal", req: workspacestore.ExportPathRequest{WorkspaceID: "ws-test", Path: "../secret"}, err: workspacestore.ErrInvalidPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.ExportPath(context.Background(), tt.req)
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
}

func TestWorkspaceStoreImportFile(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	if err := os.Mkdir(filepath.Join(store.baseDir, "ws-test/inputs"), 0o700); err != nil {
		t.Fatalf("mkdir inputs: %v", err)
	}

	imported, err := store.ImportFile(context.Background(), workspacestore.ImportFileRequest{
		WorkspaceID:       "ws-test",
		Path:              "inputs/report.txt",
		MimeType:          "text/plain",
		Source:            strings.NewReader("report"),
		ExpectedSizeBytes: 6,
		ExpectedHash:      "sha256:845e91831319e89c4d656bdb80c278ac09a7230d61e5dfd2e1b1fbb436ac8917",
		ConflictPolicy:    workspacestore.ImportConflictFailIfExists,
	})
	if err != nil {
		t.Fatalf("ImportFile returned error: %v", err)
	}
	if imported.Path.Path != "inputs/report.txt" ||
		imported.Path.Kind != sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE ||
		imported.SizeBytes != 6 ||
		imported.MimeType != "text/plain" ||
		imported.ContentHash != "sha256:845e91831319e89c4d656bdb80c278ac09a7230d61e5dfd2e1b1fbb436ac8917" {
		t.Fatalf("unexpected import result: %+v", imported)
	}
	body, err := os.ReadFile(filepath.Join(store.baseDir, "ws-test/inputs/report.txt"))
	if err != nil {
		t.Fatalf("read imported file: %v", err)
	}
	if string(body) != "report" {
		t.Fatalf("unexpected imported body: %q", string(body))
	}
}

func TestWorkspaceStoreImportFileOverwriteAndEmpty(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	writeFile(t, store.baseDir, "ws-test/existing.txt", "old")

	imported, err := store.ImportFile(context.Background(), workspacestore.ImportFileRequest{
		WorkspaceID:    "ws-test",
		Path:           "existing.txt",
		Source:         strings.NewReader(""),
		ConflictPolicy: workspacestore.ImportConflictOverwrite,
	})
	if err != nil {
		t.Fatalf("ImportFile overwrite returned error: %v", err)
	}
	if imported.SizeBytes != 0 || imported.ContentHash != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("unexpected empty import result: %+v", imported)
	}
	body, err := os.ReadFile(filepath.Join(store.baseDir, "ws-test/existing.txt"))
	if err != nil {
		t.Fatalf("read overwritten file: %v", err)
	}
	if string(body) != "" {
		t.Fatalf("unexpected overwritten body: %q", string(body))
	}
}

func TestWorkspaceStoreImportFileErrors(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	writeFile(t, store.baseDir, "ws-test/existing.txt", "old")
	if err := os.Mkdir(filepath.Join(store.baseDir, "ws-test/dir"), 0o700); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}

	tests := []struct {
		name string
		req  workspacestore.ImportFileRequest
		err  error
	}{
		{name: "missing workspace id", req: workspacestore.ImportFileRequest{Path: "file.txt", Source: strings.NewReader("x")}, err: workspacestore.ErrWorkspaceNotFound},
		{name: "root path", req: workspacestore.ImportFileRequest{WorkspaceID: "ws-test", Path: "", Source: strings.NewReader("x")}, err: workspacestore.ErrInvalidPath},
		{name: "traversal", req: workspacestore.ImportFileRequest{WorkspaceID: "ws-test", Path: "../secret", Source: strings.NewReader("x")}, err: workspacestore.ErrInvalidPath},
		{name: "parent missing", req: workspacestore.ImportFileRequest{WorkspaceID: "ws-test", Path: "missing/file.txt", Source: strings.NewReader("x")}, err: workspacestore.ErrParentNotFound},
		{name: "directory target", req: workspacestore.ImportFileRequest{WorkspaceID: "ws-test", Path: "dir", Source: strings.NewReader("x")}, err: workspacestore.ErrPathIsDirectory},
		{name: "existing fail", req: workspacestore.ImportFileRequest{WorkspaceID: "ws-test", Path: "existing.txt", Source: strings.NewReader("new"), ConflictPolicy: workspacestore.ImportConflictFailIfExists}, err: workspacestore.ErrPathAlreadyExists},
		{name: "hash mismatch", req: workspacestore.ImportFileRequest{WorkspaceID: "ws-test", Path: "hash.txt", Source: strings.NewReader("new"), ExpectedHash: "sha256:nope"}, err: workspacestore.ErrContentHashMismatch},
		{name: "too large", req: workspacestore.ImportFileRequest{WorkspaceID: "ws-test", Path: "large.txt", Source: strings.NewReader("large"), MaxBytes: 4}, err: workspacestore.ErrImportTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.ImportFile(context.Background(), tt.req)
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(store.baseDir, "ws-test/hash.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("hash mismatch left target file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.baseDir, "ws-test/large.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("too large import left target file: %v", err)
	}
}

func TestWorkspaceStoreImportFileSymlinkPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test")
	}
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	writeFile(t, store.baseDir, "ws-test/file.txt", "content")
	if err := os.Symlink(filepath.Join(store.baseDir, "ws-test/file.txt"), filepath.Join(store.baseDir, "ws-test/link.txt")); err != nil {
		t.Fatalf("symlink file: %v", err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(store.baseDir, "ws-test/linkdir")); err != nil {
		t.Fatalf("symlink outside dir: %v", err)
	}

	if _, err := store.ImportFile(context.Background(), workspacestore.ImportFileRequest{WorkspaceID: "ws-test", Path: "link.txt", Source: strings.NewReader("x"), ConflictPolicy: workspacestore.ImportConflictOverwrite}); !errors.Is(err, workspacestore.ErrSymlinkNotAllowed) {
		t.Fatalf("expected target symlink rejection, got %v", err)
	}
	if _, err := store.ImportFile(context.Background(), workspacestore.ImportFileRequest{WorkspaceID: "ws-test", Path: "linkdir/file.txt", Source: strings.NewReader("x")}); !errors.Is(err, workspacestore.ErrSymlinkNotAllowed) && !errors.Is(err, workspacestore.ErrPathEscapesWorkspace) {
		t.Fatalf("expected parent symlink rejection, got %v", err)
	}
}

func TestWorkspaceStoreSymlinkPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test")
	}
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")
	writeFile(t, store.baseDir, "ws-test/file.txt", "content")
	if err := os.Symlink(filepath.Join(store.baseDir, "ws-test/file.txt"), filepath.Join(store.baseDir, "ws-test/link.txt")); err != nil {
		t.Fatalf("symlink file: %v", err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside secret: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(store.baseDir, "ws-test/linkdir")); err != nil {
		t.Fatalf("symlink outside dir: %v", err)
	}

	if _, err := store.PreviewFile(context.Background(), workspacestore.PreviewFileRequest{WorkspaceID: "ws-test", Path: "link.txt"}); !errors.Is(err, workspacestore.ErrSymlinkNotAllowed) {
		t.Fatalf("expected leaf symlink rejection, got %v", err)
	}
	if _, err := store.ExportPath(context.Background(), workspacestore.ExportPathRequest{WorkspaceID: "ws-test", Path: "link.txt"}); !errors.Is(err, workspacestore.ErrSymlinkNotAllowed) {
		t.Fatalf("expected export symlink rejection, got %v", err)
	}
	if _, err := store.ListDir(context.Background(), workspacestore.ListDirRequest{WorkspaceID: "ws-test", Path: "linkdir"}); !errors.Is(err, workspacestore.ErrPathEscapesWorkspace) {
		t.Fatalf("expected list parent symlink escape rejection, got %v", err)
	}
	if _, err := store.PreviewFile(context.Background(), workspacestore.PreviewFileRequest{WorkspaceID: "ws-test", Path: "linkdir/secret.txt"}); !errors.Is(err, workspacestore.ErrPathEscapesWorkspace) {
		t.Fatalf("expected preview parent symlink escape rejection, got %v", err)
	}
	if _, err := store.ExportPath(context.Background(), workspacestore.ExportPathRequest{WorkspaceID: "ws-test", Path: "linkdir/secret.txt"}); !errors.Is(err, workspacestore.ErrPathEscapesWorkspace) {
		t.Fatalf("expected export parent symlink escape rejection, got %v", err)
	}
}

func TestWorkspaceStoreMissingBackingRoot(t *testing.T) {
	store := newTestWorkspaceStore(t)
	if _, err := store.ListDir(context.Background(), workspacestore.ListDirRequest{WorkspaceID: "missing"}); !errors.Is(err, workspacestore.ErrWorkspaceNotReady) {
		t.Fatalf("expected missing backing root error, got %v", err)
	}
}

func newTestWorkspaceStore(t *testing.T) *WorkspaceStore {
	t.Helper()
	store, err := NewWorkspaceStore(Config{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewWorkspaceStore returned error: %v", err)
	}
	return store
}

func createBacking(t *testing.T, store *WorkspaceStore, workspaceID string) {
	t.Helper()
	if _, err := store.CreateBackingWorkspace(context.Background(), workspacestore.CreateBackingWorkspaceRequest{WorkspaceID: workspaceID, SandboxProfileID: "local-process"}); err != nil {
		t.Fatalf("CreateBackingWorkspace returned error: %v", err)
	}
}

func writeFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func pathInfoNames(entries []workspacestore.PathInfo) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Name)
	}
	return out
}
