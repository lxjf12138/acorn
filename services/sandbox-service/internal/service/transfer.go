package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	resourceblob "github.com/lxjf12138/acorn/packages/core/resourceblob"
	"github.com/lxjf12138/acorn/packages/core/telemetry"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	leasedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacelease"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WorkspaceTransferService struct {
	sandboxv1.UnimplementedWorkspaceTransferServiceServer

	serviceID      string
	workspaceStore workspacedomain.Store
	backing        workspacestore.Store
	blobStore      resourceblob.Store
	exportStore    exporteddomain.Store
	leases         leasedomain.Manager
}

func NewWorkspaceTransferService(serviceID string, workspaceStore workspacedomain.Store, backing workspacestore.Store, blobStore resourceblob.Store, exportStore exporteddomain.Store, leases leasedomain.Manager) *WorkspaceTransferService {
	return &WorkspaceTransferService{
		serviceID:      serviceID,
		workspaceStore: workspaceStore,
		backing:        backing,
		blobStore:      blobStore,
		exportStore:    exportStore,
		leases:         leases,
	}
}

func (s *WorkspaceTransferService) ImportResourceToWorkspace(stream sandboxv1.WorkspaceTransferService_ImportResourceToWorkspaceServer) error {
	ctx, span := telemetry.Start(stream.Context(), "sandbox-service/service", telemetry.SpanWorkspaceTransferImport)
	defer span.End()
	span.SetAttributes(attribute.String(telemetry.AttrOperation, "workspace.transfer.import"))
	first, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			err = status.Error(codes.InvalidArgument, "import header is required")
		}
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return err
	}
	header := first.GetHeader()
	if header == nil {
		err := status.Error(codes.InvalidArgument, "first import message must be header")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		return err
	}
	if header.GetServiceWorkspaceId() == "" {
		err := status.Error(codes.InvalidArgument, "service_workspace_id is required")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		return err
	}
	if header.GetResource().GetId() == "" {
		err := status.Error(codes.InvalidArgument, "resource.id is required")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		return err
	}
	if header.GetDestinationPath() == "" {
		err := status.Error(codes.InvalidArgument, "destination_path is required")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		return err
	}
	workspace, err := s.workspaceStore.Get(ctx, header.GetServiceWorkspaceId())
	if errors.Is(err, workspacedomain.ErrNotFound) {
		err = status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		if status.Code(err) == codes.Unknown {
			err = status.Errorf(codes.Internal, "get hosted workspace: %v", err)
		}
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return err
	}
	if workspace.StoreWorkspaceID == "" {
		workspace.StoreWorkspaceID = workspace.ID
	}
	span.SetAttributes(
		attribute.String(telemetry.AttrWorkspaceProfileID, workspace.SandboxProfileID),
		attribute.String(telemetry.AttrResourceMimeType, header.GetResource().GetMimeType()),
		attribute.Int64(telemetry.AttrResourceSizeBytes, header.GetResource().GetSizeBytes()),
	)
	lease, err := acquireWorkspaceLease(ctx, s.leases, workspace.ID, leasedomain.ModeWrite, "import_resource", header.GetScope())
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return err
	}
	defer releaseWorkspaceLease(ctx, s.leases, lease)

	imported, err := s.backing.ImportFile(ctx, workspacestore.ImportFileRequest{
		WorkspaceID:       workspace.StoreWorkspaceID,
		Path:              header.GetDestinationPath(),
		Name:              header.GetResource().GetName(),
		MimeType:          header.GetResource().GetMimeType(),
		Source:            &importResourceStreamReader{stream: stream},
		ExpectedSizeBytes: header.GetResource().GetSizeBytes(),
		ExpectedHash:      header.GetResource().GetContentHash(),
		ConflictPolicy:    importConflictPolicy(header.GetConflictPolicy()),
	})
	if err != nil {
		if _, ok := status.FromError(err); ok {
			telemetry.RecordError(span, err)
			span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
			return err
		}
		mapped := mapWorkspaceStoreError(err)
		telemetry.RecordError(span, mapped)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(mapped)))
		return mapped
	}
	resp := &sandboxv1.ImportResourceToWorkspaceResponse{
		Path: &sandboxv1.WorkspacePathRef{
			Workspace: &workspacev1.WorkspaceHostRef{
				ServiceId:          s.serviceID,
				ServiceWorkspaceId: workspace.ID,
				SandboxProfileId:   workspace.SandboxProfileID,
			},
			Path:        imported.Path.Path,
			Kind:        imported.Path.Kind,
			DisplayName: imported.Path.Name,
		},
		SizeBytes:   imported.SizeBytes,
		ContentHash: imported.ContentHash,
		MimeType:    imported.MimeType,
	}
	if err := stream.SendAndClose(resp); err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return err
	}
	span.SetAttributes(
		attribute.String(telemetry.AttrResourceMimeType, imported.MimeType),
		attribute.Int64(telemetry.AttrResourceSizeBytes, imported.SizeBytes),
		attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
	)
	return nil
}

func (s *WorkspaceTransferService) ExportWorkspacePath(ctx context.Context, req *sandboxv1.ExportWorkspacePathRequest) (*sandboxv1.ExportWorkspacePathResponse, error) {
	ctx, span := telemetry.Start(ctx, "sandbox-service/service", telemetry.SpanWorkspaceTransferExport)
	defer span.End()
	span.SetAttributes(attribute.String(telemetry.AttrOperation, "workspace.transfer.export"))
	workspace, err := s.workspace(ctx, req.GetServiceWorkspaceId())
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return nil, err
	}
	span.SetAttributes(attribute.String(telemetry.AttrWorkspaceProfileID, workspace.SandboxProfileID))
	lease, err := acquireWorkspaceLease(ctx, s.leases, workspace.ID, leasedomain.ModeRead, "export_workspace_path", req.GetScope())
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return nil, err
	}
	defer releaseWorkspaceLease(ctx, s.leases, lease)
	exported, err := s.exportableFile(ctx, workspace, req.GetPath())
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return nil, err
	}

	resourceID := newExportedResourceID()
	name := req.GetResourceName()
	if name == "" {
		name = exported.Source.Name
	}
	mimeType := req.GetMimeType()
	if mimeType == "" {
		mimeType = exported.MimeType
	}

	reader, err := exported.Open(ctx)
	if err != nil {
		err = status.Errorf(codes.Internal, "open exported workspace path: %v", err)
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}
	defer reader.Close()

	blob, err := s.blobStore.Put(ctx, resourceblob.PutRequest{
		ResourceID:   resourceID,
		Name:         name,
		MimeType:     mimeType,
		Source:       reader,
		MetadataJSON: req.GetMetadataJson(),
	})
	if err != nil {
		mapped := mapResourceBlobError(err)
		telemetry.RecordError(span, mapped)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(mapped)))
		return nil, mapped
	}

	if _, err := s.exportStore.Create(ctx, exporteddomain.Record{
		ResourceID: resourceID,

		BlobStoreKind: blob.StoreKind,
		BlobID:        blob.StoreBlobID,

		Name:        blob.Name,
		MimeType:    blob.MimeType,
		SizeBytes:   blob.SizeBytes,
		ContentHash: blob.ContentHash,

		SourceServiceWorkspaceID: workspace.ID,
		SourceWorkspacePath:      exported.Source.Path,
	}); err != nil {
		_ = s.blobStore.Delete(ctx, resourceID)
		if errors.Is(err, exporteddomain.ErrAlreadyExists) {
			err = status.Error(codes.AlreadyExists, "exported resource already exists")
			telemetry.RecordError(span, err)
			span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
			return nil, err
		}
		err = status.Errorf(codes.Internal, "create exported resource record: %v", err)
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}

	span.SetAttributes(
		attribute.String(telemetry.AttrResourceMimeType, blob.MimeType),
		attribute.Int64(telemetry.AttrResourceSizeBytes, blob.SizeBytes),
		attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
	)
	return &sandboxv1.ExportWorkspacePathResponse{
		Source: &sandboxv1.WorkspacePathRef{
			Workspace: &workspacev1.WorkspaceHostRef{
				ServiceId:          s.serviceID,
				ServiceWorkspaceId: workspace.ID,
				SandboxProfileId:   workspace.SandboxProfileID,
			},
			Path:        exported.Source.Path,
			Kind:        exported.Source.Kind,
			DisplayName: exported.Source.Name,
		},
		Resource: &resourcev1.ResourceRef{
			Id:                 resourceID,
			AuthorityServiceId: s.serviceID,
			Name:               blob.Name,
			MimeType:           blob.MimeType,
			SizeBytes:          blob.SizeBytes,
			ContentHash:        blob.ContentHash,
			MetadataJson:       append([]byte(nil), req.GetMetadataJson()...),
		},
	}, nil
}

type importResourceStreamReader struct {
	stream sandboxv1.WorkspaceTransferService_ImportResourceToWorkspaceServer
	buf    []byte
}

func (r *importResourceStreamReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 {
		msg, err := r.stream.Recv()
		if err != nil {
			if err == io.EOF {
				return 0, io.EOF
			}
			return 0, err
		}
		if msg.GetHeader() != nil {
			return 0, status.Error(codes.InvalidArgument, "import header must only be sent once")
		}
		data := msg.GetData()
		if len(data) == 0 {
			continue
		}
		r.buf = data
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func importConflictPolicy(policy sandboxv1.ImportConflictPolicy) workspacestore.ImportConflictPolicy {
	if policy == sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_OVERWRITE {
		return workspacestore.ImportConflictOverwrite
	}
	return workspacestore.ImportConflictFailIfExists
}

func (s *WorkspaceTransferService) workspace(ctx context.Context, serviceWorkspaceID string) (workspacedomain.Workspace, error) {
	if serviceWorkspaceID == "" {
		return workspacedomain.Workspace{}, status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	workspace, err := s.workspaceStore.Get(ctx, serviceWorkspaceID)
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return workspacedomain.Workspace{}, status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return workspacedomain.Workspace{}, status.Errorf(codes.Internal, "get hosted workspace: %v", err)
	}
	if workspace.StoreWorkspaceID == "" {
		workspace.StoreWorkspaceID = workspace.ID
	}
	return workspace, nil
}

func (s *WorkspaceTransferService) exportableFile(ctx context.Context, workspace workspacedomain.Workspace, inputPath string) (*workspacestore.ExportedPath, error) {
	exported, err := s.backing.ExportPath(ctx, workspacestore.ExportPathRequest{
		WorkspaceID: workspace.StoreWorkspaceID,
		Path:        inputPath,
	})
	if err != nil {
		return nil, mapWorkspaceStoreError(err)
	}
	return exported, nil
}

func newExportedResourceID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "res_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("res_%d", time.Now().UnixNano())
}
