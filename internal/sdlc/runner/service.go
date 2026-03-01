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

type StartError struct {
	WorkflowID string
	Err        error
}

func (e *StartError) Error() string {
	if e.WorkflowID == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("workflow %s started but post-start failed: %v", e.WorkflowID, e.Err)
}

func (e *StartError) Unwrap() error { return e.Err }

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
		return workflowID, &StartError{WorkflowID: workflowID, Err: fmt.Errorf("write start audit event: %w", err)}
	}
	return workflowID, nil
}
