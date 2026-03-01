package temporal

import (
	"errors"
	"testing"

	"agenticsdlc/internal/sdlc"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestOrchestrationWorkflow_RemediationPath(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()

	securityCalls := 0
	env.RegisterActivityWithOptions(func(_ sdlc.SDLCRequest) error { return nil }, activity.RegisterOptions{Name: activityProduct})
	env.RegisterActivityWithOptions(func(_ sdlc.SDLCRequest) error { return nil }, activity.RegisterOptions{Name: activityDeveloper})
	env.RegisterActivityWithOptions(func(_ sdlc.SDLCRequest) error {
		securityCalls++
		if securityCalls == 1 {
			return errors.New("fail once")
		}
		return nil
	}, activity.RegisterOptions{Name: activitySecurity})
	env.RegisterWorkflow(OrchestrationWorkflow)

	env.ExecuteWorkflow(OrchestrationWorkflow, sdlc.SDLCRequest{WorkflowID: "wf-1"}, 2)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestOrchestrationWorkflow_FailsAfterRetries(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(_ sdlc.SDLCRequest) error { return nil }, activity.RegisterOptions{Name: activityProduct})
	env.RegisterActivityWithOptions(func(_ sdlc.SDLCRequest) error { return nil }, activity.RegisterOptions{Name: activityDeveloper})
	env.RegisterActivityWithOptions(func(_ sdlc.SDLCRequest) error { return errors.New("always fail") }, activity.RegisterOptions{Name: activitySecurity})
	env.RegisterWorkflow(OrchestrationWorkflow)

	env.ExecuteWorkflow(OrchestrationWorkflow, sdlc.SDLCRequest{WorkflowID: "wf-2"}, 1)
	require.True(t, env.IsWorkflowCompleted())
	require.ErrorContains(t, env.GetWorkflowError(), sdlc.ErrSecurityGateFailed.Error())
}
