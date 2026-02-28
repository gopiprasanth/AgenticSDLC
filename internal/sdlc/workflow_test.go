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
	developerErr       error
	developerErrByCall map[int]error
	securityErrByCall  map[int]error
	securityCalls      int
	developerCalls     int
}

func (f *fakeEngine) ExecuteProduct(context.Context, sdlc.SDLCRequest) error {
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

func TestCoordinatorRun_HappyPath(t *testing.T) {
	store := memory.NewStore()
	engine := &fakeEngine{}
	coordinator := sdlc.NewCoordinator(store, engine, 1)

	err := coordinator.Run(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-1", ProjectID: "proj-1"})
	require.NoError(t, err)

	run, err := store.FindRun(context.Background(), "wf-1")
	require.NoError(t, err)
	require.Equal(t, "completed", run.Status)
	require.Equal(t, sdlc.StageSecurity, run.Stage)
	require.Equal(t, 0, run.Attempt)
}

func TestCoordinatorRun_SecurityFailThenPass(t *testing.T) {
	store := memory.NewStore()
	engine := &fakeEngine{securityErrByCall: map[int]error{1: errors.New("gosec fail")}}
	coordinator := sdlc.NewCoordinator(store, engine, 2)

	err := coordinator.Run(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-2", ProjectID: "proj-2"})
	require.NoError(t, err)
	require.Equal(t, 2, engine.developerCalls, "initial development + remediation should run")

	run, err := store.FindRun(context.Background(), "wf-2")
	require.NoError(t, err)
	require.Equal(t, "completed", run.Status)
	require.Equal(t, 1, run.Attempt)
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
		developerErrByCall: map[int]error{2: errors.New("developer fix failed")},
	}
	coordinator := sdlc.NewCoordinator(store, engine, 2)

	err := coordinator.Run(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-4", ProjectID: "proj-4"})
	require.ErrorContains(t, err, "developer remediation")
}
