package server

import (
	"encoding/json"
	"io"
	nethttp "net/http"

	klog "github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"github.com/lxjf12138/acorn/packages/servicekit"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/service"
)

func NewHTTPServer(cfg *conf.Config, statusService *service.StatusService, workspaceService *service.WorkspaceService, resourceService *service.ResourceService, logger klog.Logger) *khttp.Server {
	srv := khttp.NewServer(
		khttp.Address(cfg.Server.HTTP.Addr),
		khttp.Timeout(cfg.Server.HTTP.TimeoutDuration()),
		khttp.Middleware(servicekit.DefaultServerMiddleware(logger)...),
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
		record, err := workspaceService.CreateSessionWorkspace(ctx, ctx.Vars().Get("session_id"), ownerUserID(ctx))
		if err != nil {
			return err
		}
		return writeProtoJSON(ctx, nethttp.StatusOK, record)
	})
	router.GET("/sessions/{session_id}/workspace", func(ctx khttp.Context) error {
		record, err := workspaceService.GetSessionWorkspace(ctx, ctx.Vars().Get("session_id"))
		if err != nil {
			return err
		}
		return writeProtoJSON(ctx, nethttp.StatusOK, record)
	})
	router.GET("/sessions/{session_id}/workspace/state", func(ctx khttp.Context) error {
		sessionState, err := workspaceService.GetSessionWorkspaceState(ctx, ctx.Vars().Get("session_id"))
		if err != nil {
			return err
		}
		return writeSessionWorkspaceStateJSON(ctx, nethttp.StatusOK, sessionState)
	})
	router.POST("/resources", func(ctx khttp.Context) error {
		var req resourcev1.RegisterResourceRequest
		if err := readProtoJSON(ctx, &req); err != nil {
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
		return writeProtoJSON(ctx, nethttp.StatusOK, &resourcev1.ListResourcesResponse{
			Resources: records,
		})
	})

	return srv
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

func readProtoJSON(ctx khttp.Context, msg proto.Message) error {
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return err
	}
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(body, msg)
}

func writeProtoJSON(ctx khttp.Context, code int, msg proto.Message) error {
	body, err := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}.Marshal(msg)
	if err != nil {
		return err
	}
	return ctx.Blob(code, "application/json", body)
}

func writeRegisterResourceJSON(ctx khttp.Context, code int, record *resourcev1.ResourceRecord) error {
	return writeProtoJSON(ctx, code, &resourcev1.RegisterResourceResponse{Resource: record})
}

func writeGetResourceJSON(ctx khttp.Context, code int, record *resourcev1.ResourceRecord) error {
	return writeProtoJSON(ctx, code, &resourcev1.GetResourceResponse{Resource: record})
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
