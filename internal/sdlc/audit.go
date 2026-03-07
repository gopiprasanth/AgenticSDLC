package sdlc

import "context"

type ActivityAuditEvent struct {
	EventID    string
	WorkflowID string
	Stage      Stage
	Status     string
	Detail     string
}

type AuditWriter interface {
	WriteStartEvent(ctx context.Context, workflowID string, req SDLCRequest) error
	WriteActivityEvent(ctx context.Context, event ActivityAuditEvent) error
}
