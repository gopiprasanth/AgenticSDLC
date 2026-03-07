package temporal

import (
	"context"
	"fmt"

	"agenticsdlc/internal/sdlc"
	"go.temporal.io/sdk/activity"
)

type Activities struct {
	engine sdlc.WorkflowEngine
	audit  sdlc.AuditWriter
}

func NewActivities(engine sdlc.WorkflowEngine, audit sdlc.AuditWriter) *Activities {
	return &Activities{engine: engine, audit: audit}
}

func (a *Activities) Product(ctx context.Context, req sdlc.SDLCRequest) error {
	return a.executeStage(ctx, req, sdlc.StageProduct, a.engine.ExecuteProduct)
}

func (a *Activities) Developer(ctx context.Context, req sdlc.SDLCRequest) error {
	return a.executeStage(ctx, req, sdlc.StageDeveloper, a.engine.ExecuteDeveloper)
}

func (a *Activities) Security(ctx context.Context, req sdlc.SDLCRequest) error {
	return a.executeStage(ctx, req, sdlc.StageSecurity, a.engine.ExecuteSecurity)
}

func (a *Activities) executeStage(ctx context.Context, req sdlc.SDLCRequest, stage sdlc.Stage, fn func(context.Context, sdlc.SDLCRequest) error) error {
	err := fn(ctx, req)
	status := "completed"
	detail := "ok"
	if err != nil {
		status = "failed"
		detail = err.Error()
	}
	if auditErr := a.audit.WriteActivityEvent(ctx, sdlc.ActivityAuditEvent{
		EventID:    activityEventID(ctx, status),
		WorkflowID: req.WorkflowID,
		Stage:      stage,
		Status:     status,
		Detail:     detail,
	}); auditErr != nil {
		if err != nil {
			return fmt.Errorf("stage error: %v; audit write: %w", err, auditErr)
		}
		return fmt.Errorf("audit write: %w", auditErr)
	}
	if err != nil {
		return err
	}
	return nil
}

func activityEventID(ctx context.Context, status string) string {
	info, ok := safeActivityInfo(ctx)
	if !ok || info.WorkflowExecution.ID == "" || info.WorkflowExecution.RunID == "" {
		return "local-" + status
	}
	return composeActivityEventID(info.WorkflowExecution.ID, info.WorkflowExecution.RunID, info.ActivityID, status, info.Attempt)
}

func composeActivityEventID(workflowID, runID, activityID, status string, attempt int32) string {
	return fmt.Sprintf("%s-%s-%s-%s-%d", workflowID, runID, activityID, status, attempt)
}

func safeActivityInfo(ctx context.Context) (activity.Info, bool) {
	ok := true
	var info activity.Info
	func() {
		defer func() {
			if recover() != nil {
				ok = false
			}
		}()
		info = activity.GetInfo(ctx)
	}()
	return info, ok
}
