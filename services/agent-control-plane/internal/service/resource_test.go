package service

import (
	"context"
	"testing"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestResourceServiceRegisterResource(t *testing.T) {
	service := NewResourceService(resourcedomain.NewMemoryStore())
	record, err := service.RegisterResource(context.Background(), &resourcev1.RegisterResourceRequest{
		Ref: &resourcev1.ResourceRef{
			AuthorityServiceId: "resource-store",
			Name:               "report.pdf",
			MimeType:           "application/pdf",
		},
		OwnerUserId: "user-1",
		SessionId:   "session-1",
		Source: &resourcev1.ResourceSource{
			Type:            "sandbox_export",
			SourceServiceId: "sandbox-service",
			SourcePath:      "outputs/report.pdf",
		},
		Visibility: resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	})
	if err != nil {
		t.Fatalf("RegisterResource returned error: %v", err)
	}
	if record.GetRef().GetId() == "" || record.GetRef().GetAuthorityServiceId() != "resource-store" {
		t.Fatalf("unexpected resource ref: %+v", record.GetRef())
	}
	if record.GetSource().GetType() != "sandbox_export" {
		t.Fatalf("unexpected source: %+v", record.GetSource())
	}
}

func TestResourceServiceRejectsInvalidResource(t *testing.T) {
	service := NewResourceService(resourcedomain.NewMemoryStore())
	_, err := service.RegisterResource(context.Background(), &resourcev1.RegisterResourceRequest{
		Ref: &resourcev1.ResourceRef{Name: "report.pdf"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestResourceServiceGetAndListResources(t *testing.T) {
	service := NewResourceService(resourcedomain.NewMemoryStore())
	created, err := service.RegisterResource(context.Background(), &resourcev1.RegisterResourceRequest{
		Ref: &resourcev1.ResourceRef{
			AuthorityServiceId: "resource-store",
			Name:               "report.pdf",
		},
		OwnerUserId: "user-1",
		SessionId:   "session-1",
	})
	if err != nil {
		t.Fatalf("RegisterResource returned error: %v", err)
	}
	got, err := service.GetResource(context.Background(), created.GetRef().GetId())
	if err != nil {
		t.Fatalf("GetResource returned error: %v", err)
	}
	if got.GetRef().GetName() != "report.pdf" {
		t.Fatalf("unexpected resource: %+v", got)
	}
	listed, err := service.ListResources(context.Background(), resourcedomain.Filter{
		OwnerUserID: "user-1",
		SessionID:   "session-1",
	})
	if err != nil {
		t.Fatalf("ListResources returned error: %v", err)
	}
	if len(listed) != 1 || listed[0].GetRef().GetId() != created.GetRef().GetId() {
		t.Fatalf("unexpected resources: %+v", listed)
	}
}
