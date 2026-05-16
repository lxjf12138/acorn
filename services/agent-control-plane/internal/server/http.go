package server

import (
	"encoding/json"
	nethttp "net/http"

	klog "github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/lxjf12138/acorn/packages/servicekit"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/service"
)

func NewHTTPServer(cfg *conf.Config, statusService *service.StatusService, workspaceService *service.WorkspaceService, logger klog.Logger) *khttp.Server {
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
