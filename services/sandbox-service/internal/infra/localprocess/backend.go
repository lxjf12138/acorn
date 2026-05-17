package localprocess

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
	backenddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/backend"
	pathdomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/path"
)

const (
	Kind = "local-process-dev"

	defaultBackendID      = "local-process-dev"
	defaultTimeout        = 30 * time.Second
	maxTimeout            = 120 * time.Second
	defaultMaxStdoutBytes = int64(1024 * 1024)
	defaultMaxStderrBytes = int64(1024 * 1024)
)

type Config struct {
	ID string

	DefaultTimeout time.Duration
	MaxTimeout     time.Duration

	DefaultMaxStdoutBytes int64
	DefaultMaxStderrBytes int64
}

type Backend struct {
	id string

	defaultTimeout time.Duration
	maxTimeout     time.Duration

	defaultMaxStdoutBytes int64
	defaultMaxStderrBytes int64
}

func NewBackend(cfg Config) *Backend {
	id := cfg.ID
	if id == "" {
		id = defaultBackendID
	}
	return &Backend{
		id:                    id,
		defaultTimeout:        durationOrDefault(cfg.DefaultTimeout, defaultTimeout),
		maxTimeout:            durationOrDefault(cfg.MaxTimeout, maxTimeout),
		defaultMaxStdoutBytes: valueOrDefaultInt64(cfg.DefaultMaxStdoutBytes, defaultMaxStdoutBytes),
		defaultMaxStderrBytes: valueOrDefaultInt64(cfg.DefaultMaxStderrBytes, defaultMaxStderrBytes),
	}
}

func (b *Backend) ID() string {
	return b.id
}

func (b *Backend) Kind() string {
	return Kind
}

func (b *Backend) Acquire(_ context.Context, req backenddomain.AcquireRequest) (*backenddomain.SandboxLease, error) {
	if req.Attachment == nil || req.Attachment.Kind != attachment.KindLocalPath {
		return nil, backenddomain.ErrUnsupportedAttachment
	}
	if req.Attachment.LocalPath == "" {
		return nil, backenddomain.ErrAttachmentNotReady
	}
	info, err := os.Stat(req.Attachment.LocalPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, backenddomain.ErrAttachmentNotReady
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, backenddomain.ErrAttachmentNotReady
	}
	return &backenddomain.SandboxLease{
		ID:          fmt.Sprintf("lease_%d", time.Now().UnixNano()),
		BackendID:   b.id,
		WorkspaceID: req.WorkspaceID,
		Attachment:  req.Attachment,
		CreatedAt:   time.Now().UTC(),
		Metadata:    cloneMap(req.Metadata),
	}, nil
}

func (b *Backend) Release(context.Context, *backenddomain.SandboxLease) error {
	return nil
}

func (b *Backend) Exec(ctx context.Context, lease *backenddomain.SandboxLease, req backenddomain.ExecRequest) (*backenddomain.ExecResult, error) {
	if strings.TrimSpace(req.Command) == "" {
		return nil, backenddomain.ErrInvalidRequest
	}
	if lease == nil || lease.Attachment == nil || lease.Attachment.Kind != attachment.KindLocalPath {
		return nil, backenddomain.ErrUnsupportedAttachment
	}
	cwd, err := resolveCWD(lease.Attachment.LocalPath, req.CWD)
	if err != nil {
		return nil, err
	}
	timeout := b.clampTimeout(req.Timeout)
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, req.Command, req.Args...)
	cmd.Dir = cwd
	cmd.Env = buildEnv(lease.Attachment.LocalPath, cwd, req.Env)
	stdout := newLimitedBuffer(b.outputLimit(req.MaxStdoutBytes, b.defaultMaxStdoutBytes))
	stderr := newLimitedBuffer(b.outputLimit(req.MaxStderrBytes, b.defaultMaxStderrBytes))
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	if execCtx.Err() == context.DeadlineExceeded {
		return nil, backenddomain.ErrExecTimeout
	}
	result := &backenddomain.ExecResult{
		ExitCode:        0,
		Stdout:          stdout.Bytes(),
		Stderr:          stderr.Bytes(),
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		result.ErrorMessage = err.Error()
		return result, nil
	}
	return nil, fmt.Errorf("%w: %v", backenddomain.ErrExecStart, err)
}

func resolveCWD(root string, rel string) (string, error) {
	normalized, err := pathdomain.NormalizeWorkspacePath(rel, true)
	if err != nil {
		return "", backenddomain.ErrInvalidCWD
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if errors.Is(err, os.ErrNotExist) {
		return "", backenddomain.ErrAttachmentNotReady
	}
	if err != nil {
		return "", err
	}
	absCWD := filepath.Join(resolvedRoot, filepath.FromSlash(normalized))
	resolvedCWD, err := filepath.EvalSymlinks(absCWD)
	if errors.Is(err, os.ErrNotExist) {
		return "", backenddomain.ErrInvalidCWD
	}
	if err != nil {
		return "", err
	}
	if resolvedCWD != resolvedRoot && !strings.HasPrefix(resolvedCWD, resolvedRoot+string(os.PathSeparator)) {
		return "", backenddomain.ErrInvalidCWD
	}
	info, err := os.Stat(resolvedCWD)
	if errors.Is(err, os.ErrNotExist) {
		return "", backenddomain.ErrInvalidCWD
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", backenddomain.ErrInvalidCWD
	}
	return resolvedCWD, nil
}

func buildEnv(root string, cwd string, extra map[string]string) []string {
	env := []string{
		"PATH=/usr/bin:/bin:/usr/local/bin",
		"HOME=" + root,
		"PWD=" + cwd,
	}
	for key, value := range extra {
		if invalidEnvKey(key) || key == "PWD" {
			continue
		}
		env = append(env, key+"="+value)
	}
	return env
}

func invalidEnvKey(key string) bool {
	return key == "" || strings.Contains(key, "=") || strings.ContainsRune(key, 0)
}

func (b *Backend) clampTimeout(requested time.Duration) time.Duration {
	if requested <= 0 {
		requested = b.defaultTimeout
	}
	if b.maxTimeout > 0 && requested > b.maxTimeout {
		return b.maxTimeout
	}
	return requested
}

func (b *Backend) outputLimit(requested int64, fallback int64) int64 {
	if requested <= 0 {
		return fallback
	}
	return requested
}

func durationOrDefault(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func valueOrDefaultInt64(value int64, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func cloneMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
