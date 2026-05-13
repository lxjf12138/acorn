package service

import "github.com/lxjf12138/acorn/services/capability-node/internal/version"

type StatusService struct{}

type StatusResponse struct {
	Status string `json:"status"`
}

func NewStatusService() *StatusService {
	return &StatusService{}
}

func (s *StatusService) Health() StatusResponse {
	return StatusResponse{Status: "ok"}
}

func (s *StatusService) Ready() StatusResponse {
	return StatusResponse{Status: "ready"}
}

func (s *StatusService) Version() version.Info {
	return version.GetInfo()
}
