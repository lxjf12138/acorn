package localprocess

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
	backenddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/backend"
)

func TestBackendExecCommandSucceeds(t *testing.T) {
	backend, lease := newTestBackendAndLease(t)

	result, err := backend.Exec(context.Background(), lease, backenddomain.ExecRequest{
		Command: helperCommand(),
		Args:    helperArgs("echo", "hello"),
		Env:     helperEnv(),
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if result.ExitCode != 0 || string(result.Stdout) != "hello" || len(result.Stderr) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBackendExecCWD(t *testing.T) {
	backend, lease := newTestBackendAndLease(t)
	if err := os.Mkdir(filepath.Join(lease.Attachment.LocalPath, "subdir"), 0o700); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	result, err := backend.Exec(context.Background(), lease, backenddomain.ExecRequest{
		Command: helperCommand(),
		Args:    helperArgs("pwd"),
		Env:     helperEnv(),
		CWD:     "subdir",
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(lease.Attachment.LocalPath, "subdir"))
	if err != nil {
		t.Fatalf("EvalSymlinks subdir: %v", err)
	}
	if string(result.Stdout) != want {
		t.Fatalf("stdout = %q, want %q", string(result.Stdout), want)
	}
}

func TestBackendExecCWDRejectsTraversalAndSymlinkEscape(t *testing.T) {
	backend, lease := newTestBackendAndLease(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(lease.Attachment.LocalPath, "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	tests := []struct {
		name string
		cwd  string
	}{
		{name: "traversal", cwd: ".."},
		{name: "symlink escape", cwd: "escape"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := backend.Exec(context.Background(), lease, backenddomain.ExecRequest{
				Command: helperCommand(),
				Args:    helperArgs("echo", "hello"),
				Env:     helperEnv(),
				CWD:     tt.cwd,
			})
			if !errors.Is(err, backenddomain.ErrInvalidCWD) {
				t.Fatalf("expected invalid cwd, got %v", err)
			}
		})
	}
}

func TestBackendExecMissingCommand(t *testing.T) {
	backend, lease := newTestBackendAndLease(t)
	_, err := backend.Exec(context.Background(), lease, backenddomain.ExecRequest{})
	if !errors.Is(err, backenddomain.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestBackendExecNonzeroExit(t *testing.T) {
	backend, lease := newTestBackendAndLease(t)
	result, err := backend.Exec(context.Background(), lease, backenddomain.ExecRequest{
		Command: helperCommand(),
		Args:    helperArgs("exit", "7"),
		Env:     helperEnv(),
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if result.ExitCode != 7 || result.ErrorMessage == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBackendExecTimeout(t *testing.T) {
	backend, lease := newTestBackendAndLease(t)
	_, err := backend.Exec(context.Background(), lease, backenddomain.ExecRequest{
		Command: helperCommand(),
		Args:    helperArgs("sleep"),
		Env:     helperEnv(),
		Timeout: 10 * time.Millisecond,
	})
	if !errors.Is(err, backenddomain.ErrExecTimeout) {
		t.Fatalf("expected exec timeout, got %v", err)
	}
}

func TestBackendExecOutputLimits(t *testing.T) {
	backend, lease := newTestBackendAndLease(t)
	result, err := backend.Exec(context.Background(), lease, backenddomain.ExecRequest{
		Command:        helperCommand(),
		Args:           helperArgs("both", "abcdef"),
		Env:            helperEnv(),
		MaxStdoutBytes: 3,
		MaxStderrBytes: 2,
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if string(result.Stdout) != "abc" || !result.StdoutTruncated ||
		string(result.Stderr) != "ab" || !result.StderrTruncated {
		t.Fatalf("unexpected limited result: %+v", result)
	}
}

func TestBackendExecEnv(t *testing.T) {
	backend, lease := newTestBackendAndLease(t)
	result, err := backend.Exec(context.Background(), lease, backenddomain.ExecRequest{
		Command: helperCommand(),
		Args:    helperArgs("env", "ACORN_TEST_ENV"),
		Env: mergeEnv(helperEnv(), map[string]string{
			"ACORN_TEST_ENV": "ok",
			"PWD":            "bad",
		}),
	})
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if string(result.Stdout) != "ok" {
		t.Fatalf("unexpected env output: %q", string(result.Stdout))
	}
	pwd, err := backend.Exec(context.Background(), lease, backenddomain.ExecRequest{
		Command: helperCommand(),
		Args:    helperArgs("env", "PWD"),
		Env:     mergeEnv(helperEnv(), map[string]string{"PWD": "bad"}),
	})
	if err != nil {
		t.Fatalf("Exec PWD returned error: %v", err)
	}
	if strings.TrimSpace(string(pwd.Stdout)) == "bad" {
		t.Fatalf("PWD override was not rejected")
	}
}

func TestBackendAcquire(t *testing.T) {
	backend := NewBackend(Config{})
	root := t.TempDir()
	lease, err := backend.Acquire(context.Background(), backenddomain.AcquireRequest{
		WorkspaceID: "ws-test",
		Attachment:  localPathAttachment(root),
	})
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	if lease.ID == "" || lease.BackendID != backend.ID() || lease.WorkspaceID != "ws-test" || lease.Attachment.LocalPath != root {
		t.Fatalf("unexpected lease: %+v", lease)
	}
	if err := backend.Release(context.Background(), lease); err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
}

func TestBackendAcquireErrors(t *testing.T) {
	backend := NewBackend(Config{})
	tests := []struct {
		name string
		req  backenddomain.AcquireRequest
		err  error
	}{
		{name: "missing attachment", req: backenddomain.AcquireRequest{}, err: backenddomain.ErrUnsupportedAttachment},
		{name: "non local", req: backenddomain.AcquireRequest{Attachment: &attachment.WorkspaceAttachment{Kind: attachment.KindDockerBind}}, err: backenddomain.ErrUnsupportedAttachment},
		{name: "missing local path", req: backenddomain.AcquireRequest{Attachment: &attachment.WorkspaceAttachment{Kind: attachment.KindLocalPath}}, err: backenddomain.ErrAttachmentNotReady},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := backend.Acquire(context.Background(), tt.req)
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
}

func newTestBackendAndLease(t *testing.T) (*Backend, *backenddomain.SandboxLease) {
	t.Helper()
	backend := NewBackend(Config{DefaultTimeout: time.Second, MaxTimeout: 2 * time.Second})
	root := t.TempDir()
	lease, err := backend.Acquire(context.Background(), backenddomain.AcquireRequest{
		WorkspaceID: "ws-test",
		Attachment:  localPathAttachment(root),
	})
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	return backend, lease
}

func localPathAttachment(root string) *attachment.WorkspaceAttachment {
	return &attachment.WorkspaceAttachment{
		ID:          "att_ws-test",
		WorkspaceID: "ws-test",
		Kind:        attachment.KindLocalPath,
		LocalPath:   root,
	}
}

func helperCommand() string {
	return os.Args[0]
}

func helperArgs(args ...string) []string {
	return append([]string{"-test.run=TestHelperProcess", "--"}, args...)
}

func helperEnv() map[string]string {
	return map[string]string{"GO_WANT_HELPER_PROCESS": "1"}
}

func mergeEnv(left map[string]string, right map[string]string) map[string]string {
	out := make(map[string]string, len(left)+len(right))
	for key, value := range left {
		out[key] = value
	}
	for key, value := range right {
		out[key] = value
	}
	return out
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	args = args[1:]
	if len(args) == 0 {
		os.Exit(0)
	}
	switch args[0] {
	case "echo":
		if len(args) > 1 {
			_, _ = os.Stdout.WriteString(args[1])
		}
	case "pwd":
		cwd, _ := os.Getwd()
		_, _ = os.Stdout.WriteString(cwd)
	case "exit":
		os.Exit(7)
	case "sleep":
		time.Sleep(time.Second)
	case "both":
		if len(args) > 1 {
			_, _ = os.Stdout.WriteString(args[1])
			_, _ = os.Stderr.WriteString(args[1])
		}
	case "env":
		if len(args) > 1 {
			_, _ = os.Stdout.WriteString(os.Getenv(args[1]))
		}
	}
	os.Exit(0)
}
