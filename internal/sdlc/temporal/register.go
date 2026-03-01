package temporal

import (
	"context"

	"agenticsdlc/internal/sdlc"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	wfsdk "go.temporal.io/sdk/workflow"
)

type ActivityHandler interface {
	Product(ctx context.Context, req sdlc.SDLCRequest) error
	Developer(ctx context.Context, req sdlc.SDLCRequest) error
	Security(ctx context.Context, req sdlc.SDLCRequest) error
}

type NoopActivities struct{}

func (NoopActivities) Product(context.Context, sdlc.SDLCRequest) error   { return nil }
func (NoopActivities) Developer(context.Context, sdlc.SDLCRequest) error { return nil }
func (NoopActivities) Security(context.Context, sdlc.SDLCRequest) error  { return nil }

func Register(w worker.Worker, activities ActivityHandler) {
	w.RegisterWorkflowWithOptions(OrchestrationWorkflow, wfsdk.RegisterOptions{Name: WorkflowName})
	w.RegisterActivityWithOptions(activities.Product, activity.RegisterOptions{Name: activityProduct})
	w.RegisterActivityWithOptions(activities.Developer, activity.RegisterOptions{Name: activityDeveloper})
	w.RegisterActivityWithOptions(activities.Security, activity.RegisterOptions{Name: activitySecurity})
}
