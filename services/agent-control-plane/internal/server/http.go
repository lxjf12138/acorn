package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"strconv"
	"time"

	klog "github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	"github.com/lxjf12138/acorn/packages/core/telemetry"
	"github.com/lxjf12138/acorn/packages/servicekit"
	"github.com/lxjf12138/acorn/packages/servicekit/httpx"
	executiondomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/execution"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/service"
)

const multipartUploadOverheadBytes = int64(10 << 20)
const execRequestMaxBytes = int64(1 << 20)

func NewHTTPServer(cfg *conf.Config, statusService *service.StatusService, workspaceService *service.WorkspaceService, resourceService *service.ResourceService, resourceGatewayService *service.ResourceGatewayService, uploadService *service.UploadService, executionService *service.ExecutionService, logger klog.Logger, tracingEnabled bool) *khttp.Server {
	srv := khttp.NewServer(
		khttp.Address(cfg.Server.HTTP.Addr),
		khttp.Timeout(cfg.Server.HTTP.TimeoutDuration()),
		khttp.Middleware(servicekit.DefaultServerMiddleware(servicekit.ServerMiddlewareOptions{
			Logger:         logger,
			TracingEnabled: tracingEnabled,
		})...),
		khttp.ErrorEncoder(httpx.ErrorEncoder),
	)

	router := srv.Route("/")
	router.GET("/healthz", func(ctx khttp.Context) error {
		return ctx.JSON(nethttp.StatusOK, statusService.Health())
	})
	router.GET("/readyz", func(ctx khttp.Context) error {
		return ctx.JSON(nethttp.StatusOK, statusService.Ready())
	})
	router.GET("/version", func(ctx khttp.Context) error {
		return ctx.JSON(nethttp.StatusOK, statusService.Version())
	})
	router.POST("/sessions/{session_id}/workspace", func(ctx khttp.Context) error {
		req, err := readCreateSessionWorkspaceRequest(ctx)
		if err != nil {
			return err
		}
		record, err := workspaceService.CreateSessionWorkspaceWithInput(ctx, service.CreateSessionWorkspaceInput{
			SessionID:          ctx.Vars().Get("session_id"),
			TenantID:           req.TenantID,
			UserID:             req.UserID,
			RequestedProfileID: req.RequestedProfileID,
		})
		if err != nil {
			return err
		}
		return httpx.WriteProtoJSON(ctx, nethttp.StatusOK, record)
	})
	router.GET("/sessions/{session_id}/workspace", func(ctx khttp.Context) error {
		record, err := workspaceService.GetSessionWorkspace(ctx, ctx.Vars().Get("session_id"))
		if err != nil {
			return err
		}
		return httpx.WriteProtoJSON(ctx, nethttp.StatusOK, record)
	})
	router.GET("/sessions/{session_id}/workspace/state", func(ctx khttp.Context) error {
		sessionState, err := workspaceService.GetSessionWorkspaceState(ctx, ctx.Vars().Get("session_id"))
		if err != nil {
			return err
		}
		return writeSessionWorkspaceStateJSON(ctx, nethttp.StatusOK, sessionState)
	})
	router.GET("/sessions/{session_id}/workspace/files", func(ctx khttp.Context) error {
		pageSize, err := parseInt32Query(ctx.Query().Get("page_size"), "page_size")
		if err != nil {
			return err
		}
		resp, err := workspaceService.ListSessionWorkspaceDir(ctx, ctx.Vars().Get("session_id"), ownerUserID(ctx), ctx.Query().Get("path"), pageSize, ctx.Query().Get("page_token"))
		if err != nil {
			return err
		}
		return httpx.WriteProtoJSON(ctx, nethttp.StatusOK, resp)
	})
	router.GET("/sessions/{session_id}/workspace/files/preview", func(ctx khttp.Context) error {
		maxBytes, err := parseInt64Query(ctx.Query().Get("max_bytes"), "max_bytes")
		if err != nil {
			return err
		}
		resp, err := workspaceService.PreviewSessionWorkspaceFile(ctx, ctx.Vars().Get("session_id"), ownerUserID(ctx), ctx.Query().Get("path"), maxBytes)
		if err != nil {
			return err
		}
		return httpx.WriteProtoJSON(ctx, nethttp.StatusOK, resp)
	})
	router.POST("/sessions/{session_id}/workspace/files/export", func(ctx khttp.Context) error {
		req, err := readExportWorkspacePathRequest(ctx)
		if err != nil {
			return err
		}
		record, err := workspaceService.ExportSessionWorkspacePath(ctx, ctx.Vars().Get("session_id"), req.UserID, req.Path, req.ResourceName, req.MimeType)
		if err != nil {
			return err
		}
		return writeRegisterResourceJSON(ctx, nethttp.StatusCreated, record)
	})
	router.POST("/sessions/{session_id}/workspace/files/import", func(ctx khttp.Context) error {
		req, err := readImportResourceRequest(ctx)
		if err != nil {
			return err
		}
		resp, err := workspaceService.ImportResourceToSessionWorkspace(ctx, ctx.Vars().Get("session_id"), req.UserID, req.ResourceID, req.DestinationPath, req.ConflictPolicy)
		if err != nil {
			return err
		}
		return httpx.WriteProtoJSON(ctx, nethttp.StatusCreated, resp)
	})
	router.POST("/sessions/{session_id}/workspace/exec", func(ctx khttp.Context) error {
		httpx.MaxBytesKratosBody(ctx, execRequestMaxBytes)
		req, err := readExecWorkspaceCommandRequest(ctx)
		if err != nil {
			return err
		}
		result, err := workspaceService.ExecSessionWorkspaceCommandWithRecord(ctx, service.ExecSessionWorkspaceCommandInput{
			SessionID:      ctx.Vars().Get("session_id"),
			UserID:         req.UserID,
			Command:        req.Command,
			Args:           req.Args,
			CWD:            req.CWD,
			Env:            req.Env,
			Timeout:        req.Timeout,
			MaxStdoutBytes: req.MaxStdoutBytes,
			MaxStderrBytes: req.MaxStderrBytes,
		})
		if err != nil {
			return err
		}
		return writeExecWorkspaceCommandJSON(ctx, nethttp.StatusOK, result)
	})
	router.GET("/sessions/{session_id}/executions", func(ctx khttp.Context) error {
		filter, err := executionFilterFromQuery(ctx, ctx.Vars().Get("session_id"))
		if err != nil {
			return err
		}
		result, err := executionService.List(ctx, filter)
		if err != nil {
			return err
		}
		return ctx.JSON(nethttp.StatusOK, listExecutionsResponseFromResult(result))
	})
	router.GET("/sessions/{session_id}/executions/{execution_id}", func(ctx khttp.Context) error {
		record, err := executionService.Get(ctx, ctx.Vars().Get("execution_id"))
		if err != nil {
			return err
		}
		if record.SessionID != ctx.Vars().Get("session_id") {
			return status.Error(codes.NotFound, "execution not found")
		}
		return ctx.JSON(nethttp.StatusOK, executionResponseFromRecord(record))
	})
	router.POST("/resources", func(ctx khttp.Context) error {
		var req resourcev1.RegisterResourceRequest
		if err := httpx.ReadProtoJSON(ctx, &req); err != nil {
			return err
		}
		record, err := resourceService.RegisterRecord(ctx, &req)
		if err != nil {
			return err
		}
		return writeRegisterResourceJSON(ctx, nethttp.StatusCreated, record)
	})
	router.GET("/resources/{resource_id}", func(ctx khttp.Context) error {
		record, err := resourceService.GetRecord(ctx, ctx.Vars().Get("resource_id"))
		if err != nil {
			return err
		}
		return writeGetResourceJSON(ctx, nethttp.StatusOK, record)
	})
	router.GET("/resources", func(ctx khttp.Context) error {
		filter, err := resourceFilterFromQuery(ctx)
		if err != nil {
			return err
		}
		records, err := resourceService.ListRecords(ctx, filter)
		if err != nil {
			return err
		}
		return httpx.WriteProtoJSON(ctx, nethttp.StatusOK, &resourcev1.ListResourcesResponse{
			Resources: records,
		})
	})
	router.POST("/resources/upload", func(ctx khttp.Context) error {
		httpx.MaxBytesKratosBody(ctx, uploadRequestBodyLimit(cfg.Resource.UploadMaxBytes))
		input, err := readUploadResourceInput(ctx)
		if err != nil {
			return err
		}
		if closer, ok := input.Source.(io.Closer); ok {
			defer closer.Close()
		}
		if form := ctx.Request().MultipartForm; form != nil {
			defer form.RemoveAll()
		}
		record, err := uploadService.UploadResource(ctx, input)
		if err != nil {
			return err
		}
		return writeRegisterResourceJSON(ctx, nethttp.StatusCreated, record)
	})
	router.GET("/resources/{resource_id}/download", func(ctx khttp.Context) error {
		return downloadResource(ctx, resourceGatewayService)
	})

	return srv
}

func uploadRequestBodyLimit(maxUploadBytes int64) int64 {
	if maxUploadBytes <= 0 {
		maxUploadBytes = 100 * 1024 * 1024
	}
	return maxUploadBytes + multipartUploadOverheadBytes
}

func readUploadResourceInput(ctx khttp.Context) (service.UploadResourceInput, error) {
	var input service.UploadResourceInput
	if err := ctx.Request().ParseMultipartForm(32 << 20); err != nil {
		var maxBytesErr *nethttp.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return input, status.Error(codes.ResourceExhausted, "upload too large")
		}
		return input, status.Error(codes.InvalidArgument, "invalid multipart upload")
	}
	file, header, err := ctx.Request().FormFile("file")
	if err != nil {
		return input, status.Error(codes.InvalidArgument, "file field is required")
	}
	input.UserID = ctx.Request().FormValue("user_id")
	if input.UserID == "" {
		input.UserID = ownerUserID(ctx)
	}
	input.SessionID = ctx.Request().FormValue("session_id")
	input.Name = ctx.Request().FormValue("name")
	if input.Name == "" && header != nil {
		input.Name = header.Filename
	}
	input.MimeType = ctx.Request().FormValue("mime_type")
	if input.MimeType != "" {
		input.Source = file
		return input, nil
	}
	if header != nil {
		input.MimeType = header.Header.Get("Content-Type")
	}
	head, err := io.ReadAll(io.LimitReader(file, 512))
	if err != nil {
		return input, err
	}
	if input.MimeType == "" && len(head) > 0 {
		input.MimeType = nethttp.DetectContentType(head)
	}
	if input.MimeType == "" {
		input.MimeType = "application/octet-stream"
	}
	input.Source = readCloser{
		Reader: io.MultiReader(bytes.NewReader(head), file),
		Closer: file,
	}
	return input, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func ownerUserID(ctx khttp.Context) string {
	if userID := ctx.Query().Get("user_id"); userID != "" {
		return userID
	}
	if userID := ctx.Header().Get("X-User-ID"); userID != "" {
		return userID
	}
	return "dev-user"
}

func parseInt32Query(value string, name string) (int32, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "invalid %s", name)
	}
	return int32(parsed), nil
}

func parseInt64Query(value string, name string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "invalid %s", name)
	}
	return parsed, nil
}

func parseIntQuery(value string, name string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "invalid %s", name)
	}
	return parsed, nil
}

type exportWorkspacePathRequest struct {
	Path         string `json:"path"`
	ResourceName string `json:"resource_name"`
	MimeType     string `json:"mime_type"`
	UserID       string `json:"user_id"`
}

type createSessionWorkspaceRequest struct {
	UserID             string `json:"user_id"`
	TenantID           string `json:"tenant_id"`
	RequestedProfileID string `json:"requested_profile_id"`
}

func readCreateSessionWorkspaceRequest(ctx khttp.Context) (createSessionWorkspaceRequest, error) {
	var req createSessionWorkspaceRequest
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return req, err
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return req, status.Error(codes.InvalidArgument, "invalid create workspace JSON")
		}
	}
	if req.UserID == "" {
		req.UserID = ownerUserID(ctx)
	}
	if req.TenantID == "" {
		req.TenantID = ctx.Query().Get("tenant_id")
	}
	if req.RequestedProfileID == "" {
		req.RequestedProfileID = ctx.Query().Get("requested_profile_id")
	}
	return req, nil
}

type importResourceRequest struct {
	ResourceID      string `json:"resource_id"`
	DestinationPath string `json:"destination_path"`
	ConflictPolicy  string `json:"conflict_policy"`
	UserID          string `json:"user_id"`
}

type execWorkspaceCommandRequest struct {
	Command        string            `json:"command"`
	Args           []string          `json:"args"`
	CWD            string            `json:"cwd"`
	Env            map[string]string `json:"env"`
	TimeoutSeconds int64             `json:"timeout_seconds"`
	MaxStdoutBytes int64             `json:"max_stdout_bytes"`
	MaxStderrBytes int64             `json:"max_stderr_bytes"`
	UserID         string            `json:"user_id"`
}

func readExecWorkspaceCommandRequest(ctx khttp.Context) (struct {
	Command        string
	Args           []string
	CWD            string
	Env            map[string]string
	Timeout        time.Duration
	MaxStdoutBytes int64
	MaxStderrBytes int64
	UserID         string
}, error) {
	var raw execWorkspaceCommandRequest
	out := struct {
		Command        string
		Args           []string
		CWD            string
		Env            map[string]string
		Timeout        time.Duration
		MaxStdoutBytes int64
		MaxStderrBytes int64
		UserID         string
	}{}
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		var maxBytesErr *nethttp.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return out, status.Error(codes.ResourceExhausted, "exec request is too large")
		}
		return out, err
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			return out, status.Error(codes.InvalidArgument, "invalid exec request JSON")
		}
	}
	if raw.Command == "" {
		return out, status.Error(codes.InvalidArgument, "command is required")
	}
	if raw.TimeoutSeconds < 0 {
		return out, status.Error(codes.InvalidArgument, "timeout_seconds must be non-negative")
	}
	out.Command = raw.Command
	out.Args = append([]string(nil), raw.Args...)
	out.CWD = raw.CWD
	out.Env = cloneStringMap(raw.Env)
	out.Timeout = time.Duration(raw.TimeoutSeconds) * time.Second
	out.MaxStdoutBytes = raw.MaxStdoutBytes
	out.MaxStderrBytes = raw.MaxStderrBytes
	out.UserID = raw.UserID
	if out.UserID == "" {
		out.UserID = ownerUserID(ctx)
	}
	return out, nil
}

func readImportResourceRequest(ctx khttp.Context) (struct {
	ResourceID      string
	DestinationPath string
	ConflictPolicy  sandboxv1.ImportConflictPolicy
	UserID          string
}, error) {
	var raw importResourceRequest
	out := struct {
		ResourceID      string
		DestinationPath string
		ConflictPolicy  sandboxv1.ImportConflictPolicy
		UserID          string
	}{}
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return out, err
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			return out, status.Error(codes.InvalidArgument, "invalid import request JSON")
		}
	}
	if raw.ResourceID == "" {
		return out, status.Error(codes.InvalidArgument, "resource_id is required")
	}
	out.ResourceID = raw.ResourceID
	out.DestinationPath = raw.DestinationPath
	out.UserID = raw.UserID
	if out.UserID == "" {
		out.UserID = ownerUserID(ctx)
	}
	if raw.ConflictPolicy == "" {
		out.ConflictPolicy = sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_FAIL_IF_EXISTS
		return out, nil
	}
	value, ok := sandboxv1.ImportConflictPolicy_value[raw.ConflictPolicy]
	if !ok {
		return out, status.Errorf(codes.InvalidArgument, "invalid conflict_policy: %s", raw.ConflictPolicy)
	}
	out.ConflictPolicy = sandboxv1.ImportConflictPolicy(value)
	if out.ConflictPolicy == sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_UNSPECIFIED {
		out.ConflictPolicy = sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_FAIL_IF_EXISTS
	}
	return out, nil
}

func readExportWorkspacePathRequest(ctx khttp.Context) (exportWorkspacePathRequest, error) {
	var req exportWorkspacePathRequest
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return req, err
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return req, status.Error(codes.InvalidArgument, "invalid export request JSON")
		}
	}
	if req.UserID == "" {
		req.UserID = ownerUserID(ctx)
	}
	return req, nil
}

func writeRegisterResourceJSON(ctx khttp.Context, code int, record *resourcev1.ResourceRecord) error {
	return httpx.WriteProtoJSON(ctx, code, &resourcev1.RegisterResourceResponse{Resource: record})
}

func writeGetResourceJSON(ctx khttp.Context, code int, record *resourcev1.ResourceRecord) error {
	return httpx.WriteProtoJSON(ctx, code, &resourcev1.GetResourceResponse{Resource: record})
}

type execWorkspaceCommandResponse struct {
	Execution *executionResponse `json:"execution"`
	Result    json.RawMessage    `json:"result"`
}

type executionResponse struct {
	ID                 string `json:"id"`
	TenantID           string `json:"tenant_id,omitempty"`
	UserID             string `json:"user_id,omitempty"`
	SessionID          string `json:"session_id,omitempty"`
	WorkspaceID        string `json:"workspace_id,omitempty"`
	ServiceWorkspaceID string `json:"service_workspace_id,omitempty"`
	SandboxServiceID   string `json:"sandbox_service_id,omitempty"`
	SandboxProfileID   string `json:"sandbox_profile_id,omitempty"`
	SandboxBackendID   string `json:"sandbox_backend_id,omitempty"`
	Status             string `json:"status"`
	CommandName        string `json:"command_name,omitempty"`
	ArgCount           int    `json:"arg_count"`
	CWDSet             bool   `json:"cwd_set"`
	ExitCode           int32  `json:"exit_code,omitempty"`
	ErrorCode          string `json:"error_code,omitempty"`
	ErrorMessage       string `json:"error_message,omitempty"`
	StdoutSizeBytes    int64  `json:"stdout_size_bytes,omitempty"`
	StderrSizeBytes    int64  `json:"stderr_size_bytes,omitempty"`
	StdoutTruncated    bool   `json:"stdout_truncated"`
	StderrTruncated    bool   `json:"stderr_truncated"`
	TraceID            string `json:"trace_id,omitempty"`
	SpanID             string `json:"span_id,omitempty"`
	StartedAt          string `json:"started_at,omitempty"`
	CompletedAt        string `json:"completed_at,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

type listExecutionsResponse struct {
	Executions    []*executionResponse `json:"executions"`
	NextPageToken string               `json:"next_page_token,omitempty"`
}

func writeExecWorkspaceCommandJSON(ctx khttp.Context, code int, result *service.ExecSessionWorkspaceCommandResult) error {
	marshaler := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}
	resultJSON, err := marshaler.Marshal(result.Response)
	if err != nil {
		return err
	}
	body, err := json.Marshal(execWorkspaceCommandResponse{
		Execution: executionResponseFromRecord(result.Execution),
		Result:    resultJSON,
	})
	if err != nil {
		return err
	}
	return ctx.Blob(code, "application/json", body)
}

func executionFilterFromQuery(ctx khttp.Context, sessionID string) (executiondomain.ListFilter, error) {
	limit, err := parseIntQuery(ctx.Query().Get("limit"), "limit")
	if err != nil {
		return executiondomain.ListFilter{}, err
	}
	statusValue, err := parseExecutionStatus(ctx.Query().Get("status"))
	if err != nil {
		return executiondomain.ListFilter{}, err
	}
	return executiondomain.ListFilter{
		TenantID:    ctx.Query().Get("tenant_id"),
		UserID:      ctx.Query().Get("user_id"),
		SessionID:   sessionID,
		WorkspaceID: ctx.Query().Get("workspace_id"),
		Status:      statusValue,
		Limit:       limit,
		PageToken:   ctx.Query().Get("page_token"),
	}, nil
}

func parseExecutionStatus(value string) (executiondomain.Status, error) {
	if value == "" {
		return "", nil
	}
	switch executiondomain.Status(value) {
	case executiondomain.StatusRunning,
		executiondomain.StatusSucceeded,
		executiondomain.StatusFailed,
		executiondomain.StatusTimeout,
		executiondomain.StatusCanceled:
		return executiondomain.Status(value), nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "invalid execution status: %s", value)
	}
}

func listExecutionsResponseFromResult(result *executiondomain.ListResult) listExecutionsResponse {
	resp := listExecutionsResponse{NextPageToken: result.NextPageToken}
	for _, record := range result.Records {
		resp.Executions = append(resp.Executions, executionResponseFromRecord(record))
	}
	return resp
}

func executionResponseFromRecord(record *executiondomain.ExecutionRecord) *executionResponse {
	if record == nil {
		return nil
	}
	return &executionResponse{
		ID:                 record.ID,
		TenantID:           record.TenantID,
		UserID:             record.UserID,
		SessionID:          record.SessionID,
		WorkspaceID:        record.WorkspaceID,
		ServiceWorkspaceID: record.ServiceWorkspaceID,
		SandboxServiceID:   record.SandboxServiceID,
		SandboxProfileID:   record.SandboxProfileID,
		SandboxBackendID:   record.SandboxBackendID,
		Status:             string(record.Status),
		CommandName:        record.CommandName,
		ArgCount:           record.ArgCount,
		CWDSet:             record.CWDSet,
		ExitCode:           record.ExitCode,
		ErrorCode:          record.ErrorCode,
		ErrorMessage:       record.ErrorMessage,
		StdoutSizeBytes:    record.StdoutSizeBytes,
		StderrSizeBytes:    record.StderrSizeBytes,
		StdoutTruncated:    record.StdoutTruncated,
		StderrTruncated:    record.StderrTruncated,
		TraceID:            record.TraceID,
		SpanID:             record.SpanID,
		StartedAt:          formatTime(record.StartedAt),
		CompletedAt:        formatTime(record.CompletedAt),
		UpdatedAt:          formatTime(record.UpdatedAt),
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func downloadResource(ctx khttp.Context, gateway *service.ResourceGatewayService) error {
	reqCtx, span := telemetry.Start(ctx.Request().Context(), "agent-control-plane/server", telemetry.SpanResourceDownload)
	defer span.End()
	span.SetAttributes(attribute.String(telemetry.AttrOperation, "resource.download"))

	resourceStream, err := gateway.OpenResourceForTransfer(reqCtx, ctx.Vars().Get("resource_id"), ownerUserID(ctx))
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, httpTelemetryStatusValue(err)))
		recordDownloadTransfer(reqCtx, httpTelemetryStatusValue(err), "", 0)
		return err
	}
	ref := resourceStream.Record().GetRef()
	span.SetAttributes(
		attribute.String(telemetry.AttrResourceAuthorityServiceID, ref.GetAuthorityServiceId()),
		attribute.String(telemetry.AttrResourceMimeType, ref.GetMimeType()),
		attribute.Int64(telemetry.AttrResourceSizeBytes, ref.GetSizeBytes()),
	)
	first, err := resourceStream.Recv()
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, httpTelemetryStatusValue(err)))
		recordDownloadTransfer(reqCtx, httpTelemetryStatusValue(err), ref.GetAuthorityServiceId(), 0)
		return err
	}
	var writtenBytes int64
	setDownloadHeaders(ctx, ref)
	if len(first.GetData()) > 0 {
		n, err := ctx.Response().Write(first.GetData())
		writtenBytes += int64(n)
		if err != nil {
			telemetry.RecordError(span, err)
			span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
			recordDownloadTransfer(reqCtx, telemetry.StatusError, ref.GetAuthorityServiceId(), 0)
			return err
		}
	}
	for {
		chunk, err := resourceStream.Recv()
		if err == io.EOF {
			span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusOK))
			recordDownloadTransfer(reqCtx, telemetry.StatusOK, ref.GetAuthorityServiceId(), writtenBytes)
			return nil
		}
		if err != nil {
			telemetry.RecordError(span, err)
			span.SetAttributes(attribute.String(telemetry.AttrStatus, httpTelemetryStatusValue(err)))
			recordDownloadTransfer(reqCtx, httpTelemetryStatusValue(err), ref.GetAuthorityServiceId(), 0)
			return err
		}
		if len(chunk.GetData()) == 0 {
			continue
		}
		n, err := ctx.Response().Write(chunk.GetData())
		writtenBytes += int64(n)
		if err != nil {
			telemetry.RecordError(span, err)
			span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
			recordDownloadTransfer(reqCtx, telemetry.StatusError, ref.GetAuthorityServiceId(), 0)
			return err
		}
	}
}

func httpTelemetryStatusValue(err error) string {
	switch status.Code(err) {
	case codes.OK:
		return telemetry.StatusOK
	case codes.InvalidArgument:
		return telemetry.StatusInvalid
	case codes.PermissionDenied:
		return telemetry.StatusDenied
	default:
		return telemetry.StatusError
	}
}

func setDownloadHeaders(ctx khttp.Context, ref *resourcev1.ResourceRef) {
	mimeType := ref.GetMimeType()
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	headers := ctx.Response().Header()
	headers.Set("Content-Type", mimeType)
	if ref.GetSizeBytes() > 0 {
		headers.Set("Content-Length", strconv.FormatInt(ref.GetSizeBytes(), 10))
	}
	headers.Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, httpx.SafeFilename(ref.GetName(), ref.GetId())))
	headers.Set("X-Acorn-Resource-ID", ref.GetId())
	if ref.GetContentHash() != "" {
		headers.Set("X-Acorn-Content-Hash", ref.GetContentHash())
	}
}

func resourceFilterFromQuery(ctx khttp.Context) (resourcedomain.Filter, error) {
	query := ctx.Query()
	statusValue, err := parseResourceStatus(query.Get("status"))
	if err != nil {
		return resourcedomain.Filter{}, err
	}
	visibilityValue, err := parseResourceVisibility(query.Get("visibility"))
	if err != nil {
		return resourcedomain.Filter{}, err
	}
	return resourcedomain.Filter{
		OwnerUserID: query.Get("user_id"),
		SessionID:   query.Get("session_id"),
		Status:      statusValue,
		Visibility:  visibilityValue,
	}, nil
}

func parseResourceStatus(value string) (resourcev1.ResourceStatus, error) {
	if value == "" {
		return resourcev1.ResourceStatus_RESOURCE_STATUS_UNSPECIFIED, nil
	}
	enumValue, ok := resourcev1.ResourceStatus_value[value]
	if !ok {
		return resourcev1.ResourceStatus_RESOURCE_STATUS_UNSPECIFIED, status.Errorf(codes.InvalidArgument, "invalid resource status: %s", value)
	}
	return resourcev1.ResourceStatus(enumValue), nil
}

func parseResourceVisibility(value string) (resourcev1.ResourceVisibility, error) {
	if value == "" {
		return resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_UNSPECIFIED, nil
	}
	enumValue, ok := resourcev1.ResourceVisibility_value[value]
	if !ok {
		return resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_UNSPECIFIED, status.Errorf(codes.InvalidArgument, "invalid resource visibility: %s", value)
	}
	return resourcev1.ResourceVisibility(enumValue), nil
}

func writeSessionWorkspaceStateJSON(ctx khttp.Context, code int, state *service.SessionWorkspaceState) error {
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}
	workspaceJSON, err := marshaler.Marshal(state.Record)
	if err != nil {
		return err
	}
	stateJSON, err := marshaler.Marshal(state.State)
	if err != nil {
		return err
	}
	body, err := json.Marshal(struct {
		Workspace json.RawMessage `json:"workspace"`
		State     json.RawMessage `json:"state"`
	}{
		Workspace: workspaceJSON,
		State:     stateJSON,
	})
	if err != nil {
		return err
	}
	return ctx.Blob(code, "application/json", body)
}
