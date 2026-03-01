package temporal

import (
	"time"

	"agenticsdlc/internal/sdlc"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const WorkflowName = "agentic.sdlc.workflow"

const (
	activityProduct   = "agentic.sdlc.activity.product"
	activityDeveloper = "agentic.sdlc.activity.developer"
	activitySecurity  = "agentic.sdlc.activity.security"
)

func OrchestrationWorkflow(ctx workflow.Context, req sdlc.SDLCRequest, maxRetries int) error {
	ao := workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
		StartToCloseTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	if err := workflow.ExecuteActivity(ctx, activityProduct, req).Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, activityDeveloper, req).Get(ctx, nil); err != nil {
		return err
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := workflow.ExecuteActivity(ctx, activitySecurity, req).Get(ctx, nil); err == nil {
			return nil
		}
		if attempt == maxRetries {
			return sdlc.ErrSecurityGateFailed
		}
		if err := workflow.ExecuteActivity(ctx, activityDeveloper, req).Get(ctx, nil); err != nil {
			return err
		}
	}
	return sdlc.ErrSecurityGateFailed
}
