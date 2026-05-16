package service

import (
	"context"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/descriptor"
)

type DescriptorService struct {
	capabilityv1.UnimplementedCapabilityDescriptorServiceServer

	source *descriptor.Source
}

func NewDescriptorService(source *descriptor.Source) *DescriptorService {
	return &DescriptorService{source: source}
}

func (s *DescriptorService) GetCapabilityDescriptor(ctx context.Context, _ *capabilityv1.GetCapabilityDescriptorRequest) (*capabilityv1.GetCapabilityDescriptorResponse, error) {
	capabilityDescriptor, err := s.source.DescribeCapabilities(ctx)
	if err != nil {
		return nil, err
	}
	return &capabilityv1.GetCapabilityDescriptorResponse{
		CapabilityDescriptor: capabilityDescriptor,
	}, nil
}
