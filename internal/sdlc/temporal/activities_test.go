package temporal

import (
	"context"
	"errors"
	"testing"

	"agenticsdlc/internal/sdlc"
	"github.com/stretchr/testify/require"
)

type engineStub struct {
	productErr   error
	developerErr error
	securityErr  error
}

func (e engineStub) ExecuteProduct(context.Context, sdlc.SDLCRequest) error   { return e.productErr }
func (e engineStub) ExecuteDeveloper(context.Context, sdlc.SDLCRequest) error { return e.developerErr }
func (e engineStub) ExecuteSecurity(context.Context, sdlc.SDLCRequest) error  { return e.securityErr }

type auditStub struct {
	events []sdlc.ActivityAuditEvent
	err    error
}

func (a *auditStub) WriteStartEvent(context.Context, string, string, sdlc.SDLCRequest) error {
	return nil
}
func (a *auditStub) WriteActivityEvent(_ context.Context, event sdlc.ActivityAuditEvent) error {
	a.events = append(a.events, event)
	return a.err
}

func TestActivities_ProductWritesAudit(t *testing.T) {
	audit := &auditStub{}
	acts := NewActivities(engineStub{}, audit)

	err := acts.Product(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-1"})
	require.NoError(t, err)
	require.Len(t, audit.events, 1)
	require.Equal(t, "wf-1", audit.events[0].WorkflowID)
	require.Equal(t, sdlc.StageProduct, audit.events[0].Stage)
	require.Equal(t, "completed", audit.events[0].Status)
}

func TestActivities_DeveloperFailureStillWritesAudit(t *testing.T) {
	audit := &auditStub{}
	acts := NewActivities(engineStub{developerErr: errors.New("boom")}, audit)

	err := acts.Developer(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-2"})
	require.ErrorContains(t, err, "boom")
	require.Len(t, audit.events, 1)
	require.Equal(t, sdlc.StageDeveloper, audit.events[0].Stage)
	require.Equal(t, "failed", audit.events[0].Status)
}

func TestActivities_AuditFailureReturnsError(t *testing.T) {
	audit := &auditStub{err: errors.New("mongo unavailable")}
	acts := NewActivities(engineStub{}, audit)

	err := acts.Security(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-3"})
	require.ErrorContains(t, err, "audit write")
}

func TestComposeActivityEventID_IncludesRunID(t *testing.T) {
	eventID := composeActivityEventID("wf-1", "run-1", "activity-1", "completed", 2)
	require.Equal(t, "wf-1-run-1-activity-1-completed-2", eventID)
}
