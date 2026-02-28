package runner

import (
	"context"
	"fmt"

	"agenticsdlc/internal/sdlc"
)

type TemporalStarter interface {
	StartWorkflow(ctx context.Context, req sdlc.SDLCRequest) (string, error)
}

type MongoAuditWriter interface {
	WriteStartEvent(ctx context.Context, workflowID string, req sdlc.SDLCRequest) error
}

type Service struct {
	temporal TemporalStarter
	audit    MongoAuditWriter
}

func NewService(temporal TemporalStarter, audit MongoAuditWriter) *Service {
	return &Service{temporal: temporal, audit: audit}
}

func (s *Service) Start(ctx context.Context, req sdlc.SDLCRequest) (string, error) {
	workflowID, err := s.temporal.StartWorkflow(ctx, req)
	if err != nil {
		return "", fmt.Errorf("start temporal workflow: %w", err)
	}
	if err := s.audit.WriteStartEvent(ctx, workflowID, req); err != nil {
		return "", fmt.Errorf("write start audit event: %w", err)
	}
	return workflowID, nil
}
