package sdlc_test

import (
	"context"
	"errors"
	"testing"

	"agenticsdlc/internal/sdlc"
	"agenticsdlc/internal/sdlc/memory"
	"github.com/stretchr/testify/require"
)

type fakeEngine struct {
	productErr         error
	productErrByCall   map[int]error
	developerErr       error
	developerErrByCall map[int]error
	securityErrByCall  map[int]error
	productCalls       int
	securityCalls      int
	developerCalls     int
}

func (f *fakeEngine) ExecuteProduct(context.Context, sdlc.SDLCRequest) error {
	f.productCalls++
	if err, ok := f.productErrByCall[f.productCalls]; ok {
		return err
	}
	return f.productErr
}

func (f *fakeEngine) ExecuteDeveloper(context.Context, sdlc.SDLCRequest) error {
	f.developerCalls++
	if err, ok := f.developerErrByCall[f.developerCalls]; ok {
		return err
	}
	return f.developerErr
}

func (f *fakeEngine) ExecuteSecurity(context.Context, sdlc.SDLCRequest) error {
	f.securityCalls++
	if err, ok := f.securityErrByCall[f.securityCalls]; ok {
		return err
	}
	return nil
}

type recordingCommunicator struct {
	tasks     []sdlc.A2ATask
	errByType map[string]error
}

func (r *recordingCommunicator) SendTask(_ context.Context, task sdlc.A2ATask) error {
	if err, ok := r.errByType[task.TaskType]; ok {
		return err
	}
	r.tasks = append(r.tasks, task)
	return nil
}

func TestCoordinatorRun_HappyPath(t *testing.T) {
	store := memory.NewStore()
	engine := &fakeEngine{}
	comm := &recordingCommunicator{}
	coordinator := sdlc.NewCoordinator(store, engine, 1).WithA2ACommunicator(comm)

	err := coordinator.Run(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-1", ProjectID: "proj-1"})
	require.NoError(t, err)

	run, err := store.FindRun(context.Background(), "wf-1")
	require.NoError(t, err)
	require.Equal(t, "completed", run.Status)
	require.Equal(t, sdlc.StageSecurity, run.Stage)
	require.Equal(t, 0, run.Attempt)
	require.Len(t, comm.tasks, 2)
	require.Equal(t, "prd_ready", comm.tasks[0].TaskType)
	require.Equal(t, "changeset_ready", comm.tasks[1].TaskType)
}

func TestCoordinatorRun_DeveloperToProductClarificationLoop(t *testing.T) {
	store := memory.NewStore()
	engine := &fakeEngine{developerErrByCall: map[int]error{1: errors.New("requirements ambiguous")}}
	comm := &recordingCommunicator{}
	coordinator := sdlc.NewCoordinator(store, engine, 1).WithA2ACommunicator(comm)

	err := coordinator.Run(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-clarify", ProjectID: "proj-clarify"})
	require.NoError(t, err)
	require.Equal(t, 2, engine.productCalls, "product should run again to clarify requirements")
	require.Equal(t, 2, engine.developerCalls, "developer should retry after product clarification")

	taskTypes := make([]string, 0, len(comm.tasks))
	for _, task := range comm.tasks {
		taskTypes = append(taskTypes, task.TaskType)
	}
	require.Equal(t, []string{"prd_ready", "requirements_clarification_required", "prd_ready", "changeset_ready"}, taskTypes)
}

func TestCoordinatorRun_SecurityFailThenPass(t *testing.T) {
	store := memory.NewStore()
	engine := &fakeEngine{securityErrByCall: map[int]error{1: errors.New("gosec fail")}}
	comm := &recordingCommunicator{}
	coordinator := sdlc.NewCoordinator(store, engine, 2).WithA2ACommunicator(comm)

	err := coordinator.Run(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-2", ProjectID: "proj-2"})
	require.NoError(t, err)
	require.Equal(t, 2, engine.developerCalls, "initial development + remediation should run")

	run, err := store.FindRun(context.Background(), "wf-2")
	require.NoError(t, err)
	require.Equal(t, "completed", run.Status)
	require.Equal(t, 1, run.Attempt)

	taskTypes := make([]string, 0, len(comm.tasks))
	for _, task := range comm.tasks {
		taskTypes = append(taskTypes, task.TaskType)
	}
	require.Equal(t, []string{"prd_ready", "changeset_ready", "remediation_required", "remediation_ready"}, taskTypes)
}

func TestCoordinatorRun_SecurityFailsAfterMaxRetries(t *testing.T) {
	store := memory.NewStore()
	engine := &fakeEngine{securityErrByCall: map[int]error{1: errors.New("fail"), 2: errors.New("fail")}}
	coordinator := sdlc.NewCoordinator(store, engine, 1)

	err := coordinator.Run(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-3", ProjectID: "proj-3"})
	require.ErrorIs(t, err, sdlc.ErrSecurityGateFailed)

	run, findErr := store.FindRun(context.Background(), "wf-3")
	require.NoError(t, findErr)
	require.Equal(t, "failed", run.Status)
	require.Equal(t, sdlc.ErrSecurityGateFailed.Error(), run.LastError)
}

func TestCoordinatorRun_DeveloperRemediationFails(t *testing.T) {
	store := memory.NewStore()
	engine := &fakeEngine{
		securityErrByCall:  map[int]error{1: errors.New("security fail")},
		developerErrByCall: map[int]error{2: errors.New("developer fix failed"), 3: errors.New("developer fix failed")},
	}
	coordinator := sdlc.NewCoordinator(store, engine, 2)

	err := coordinator.Run(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-4", ProjectID: "proj-4"})
	require.ErrorContains(t, err, "developer remediation")
}

func TestCoordinatorRun_A2AFailureStopsWorkflow(t *testing.T) {
	store := memory.NewStore()
	engine := &fakeEngine{}
	comm := &recordingCommunicator{errByType: map[string]error{"changeset_ready": errors.New("a2a transport unavailable")}}
	coordinator := sdlc.NewCoordinator(store, engine, 1).WithA2ACommunicator(comm)

	err := coordinator.Run(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-5", ProjectID: "proj-5"})
	require.ErrorContains(t, err, "a2a developer->security")
	require.ErrorContains(t, err, "transport unavailable")

	run, findErr := store.FindRun(context.Background(), "wf-5")
	require.NoError(t, findErr)
	require.Equal(t, "failed", run.Status)
	require.Equal(t, "a2a transport unavailable", run.LastError)
	require.Equal(t, sdlc.StageDeveloper, run.Stage)
}
