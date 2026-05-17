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

	"github.com/lxjf12138/acorn/packages/core/telemetry"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
	backenddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/backend"
	pathdomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/path"
	"go.opentelemetry.io/otel/attribute"
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

	MaxStdoutBytes int64
	MaxStderrBytes int64
}

type Backend struct {
	id string

	defaultTimeout time.Duration
	maxTimeout     time.Duration

	maxStdoutBytes int64
	maxStderrBytes int64
}

func NewBackend(cfg Config) *Backend {
	id := cfg.ID
	if id == "" {
		id = defaultBackendID
	}
	return &Backend{
		id:             id,
		defaultTimeout: durationOrDefault(cfg.DefaultTimeout, defaultTimeout),
		maxTimeout:     durationOrDefault(cfg.MaxTimeout, maxTimeout),
		maxStdoutBytes: valueOrDefaultInt64(cfg.MaxStdoutBytes, defaultMaxStdoutBytes),
		maxStderrBytes: valueOrDefaultInt64(cfg.MaxStderrBytes, defaultMaxStderrBytes),
	}
}

func (b *Backend) ID() string {
	return b.id
}

func (b *Backend) Kind() string {
	return Kind
}

func (b *Backend) Acquire(ctx context.Context, req backenddomain.AcquireRequest) (*backenddomain.SandboxLease, error) {
	_, span := telemetry.Start(ctx, "sandbox-service/localprocess", telemetry.SpanSandboxBackendAcquire)
	defer span.End()
	span.SetAttributes(
		attribute.String(telemetry.AttrOperation, "sandbox.backend.acquire"),
		attribute.String(telemetry.AttrSandboxBackendID, b.ID()),
		attribute.String(telemetry.AttrSandboxProfileID, req.ProfileID),
	)
	if req.Attachment == nil || req.Attachment.Kind != attachment.KindLocalPath {
		telemetry.RecordError(span, backenddomain.ErrUnsupportedAttachment)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, backenddomain.ErrUnsupportedAttachment
	}
	if req.Attachment.LocalPath == "" {
		telemetry.RecordError(span, backenddomain.ErrAttachmentNotReady)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, backenddomain.ErrAttachmentNotReady
	}
	info, err := os.Stat(req.Attachment.LocalPath)
	if errors.Is(err, os.ErrNotExist) {
		telemetry.RecordError(span, backenddomain.ErrAttachmentNotReady)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, backenddomain.ErrAttachmentNotReady
	}
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}
	if !info.IsDir() {
		telemetry.RecordError(span, backenddomain.ErrAttachmentNotReady)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, backenddomain.ErrAttachmentNotReady
	}
	span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusOK))
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
	ctx, span := telemetry.Start(ctx, "sandbox-service/localprocess", telemetry.SpanSandboxBackendExec)
	defer span.End()
	span.SetAttributes(
		attribute.String(telemetry.AttrOperation, "sandbox.backend.exec"),
		attribute.String(telemetry.AttrSandboxBackendID, b.ID()),
		attribute.String(telemetry.AttrExecCommandName, telemetry.SafeCommandName(req.Command)),
		attribute.Int(telemetry.AttrExecArgCount, len(req.Args)),
	)
	if strings.TrimSpace(req.Command) == "" {
		telemetry.RecordError(span, backenddomain.ErrInvalidRequest)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		return nil, backenddomain.ErrInvalidRequest
	}
	if lease == nil || lease.Attachment == nil || lease.Attachment.Kind != attachment.KindLocalPath {
		telemetry.RecordError(span, backenddomain.ErrUnsupportedAttachment)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, backenddomain.ErrUnsupportedAttachment
	}
	cwd, err := resolveCWD(lease.Attachment.LocalPath, req.CWD)
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		return nil, err
	}
	timeout := b.clampTimeout(req.Timeout)
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, req.Command, req.Args...)
	cmd.Dir = cwd
	cmd.Env = buildEnv(lease.Attachment.LocalPath, cwd, req.Env)
	stdout := newLimitedBuffer(b.outputLimit(req.MaxStdoutBytes, b.maxStdoutBytes))
	stderr := newLimitedBuffer(b.outputLimit(req.MaxStderrBytes, b.maxStderrBytes))
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	if execCtx.Err() == context.DeadlineExceeded {
		telemetry.RecordError(span, backenddomain.ErrExecTimeout)
		span.SetAttributes(
			attribute.Bool(telemetry.AttrExecTimedOut, true),
			attribute.String(telemetry.AttrStatus, telemetry.StatusTimeout),
		)
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
		span.SetAttributes(
			attribute.Int(telemetry.AttrExecExitCode, result.ExitCode),
			attribute.Bool(telemetry.AttrExecStdoutTruncated, result.StdoutTruncated),
			attribute.Bool(telemetry.AttrExecStderrTruncated, result.StderrTruncated),
			attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
		)
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		result.ErrorMessage = err.Error()
		span.SetAttributes(
			attribute.Int(telemetry.AttrExecExitCode, result.ExitCode),
			attribute.Bool(telemetry.AttrExecStdoutTruncated, result.StdoutTruncated),
			attribute.Bool(telemetry.AttrExecStderrTruncated, result.StderrTruncated),
			attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
		)
		return result, nil
	}
	err = fmt.Errorf("%w: %v", backenddomain.ErrExecStart, err)
	telemetry.RecordError(span, err)
	span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
	return nil, err
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

func (b *Backend) outputLimit(requested int64, max int64) int64 {
	if max <= 0 {
		max = defaultMaxStdoutBytes
	}
	if requested <= 0 {
		return max
	}
	if requested > max {
		return max
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
