package runner

import (
	"context"
	"errors"
	"testing"

	"agenticsdlc/internal/sdlc"
	"github.com/stretchr/testify/require"
)

type temporalMock struct {
	workflowID string
	err        error
}

func (t temporalMock) StartWorkflow(context.Context, sdlc.SDLCRequest) (string, error) {
	if t.err != nil {
		return "", t.err
	}
	return t.workflowID, nil
}

type mongoAuditMock struct {
	err error
}

func (m mongoAuditMock) WriteStartEvent(context.Context, string, sdlc.SDLCRequest) error {
	return m.err
}

func TestServiceStart_HappyPath(t *testing.T) {
	svc := NewService(temporalMock{workflowID: "wf-100"}, mongoAuditMock{})

	workflowID, err := svc.Start(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-100", ProjectID: "proj-100"})
	require.NoError(t, err)
	require.Equal(t, "wf-100", workflowID)
}

func TestServiceStart_TemporalFailure(t *testing.T) {
	svc := NewService(temporalMock{err: errors.New("temporal down")}, mongoAuditMock{})

	workflowID, err := svc.Start(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-101", ProjectID: "proj-101"})
	require.Empty(t, workflowID)
	require.ErrorContains(t, err, "start temporal workflow")
}

func TestServiceStart_MongoFailureReturnsWorkflowID(t *testing.T) {
	svc := NewService(temporalMock{workflowID: "wf-102"}, mongoAuditMock{err: errors.New("mongo down")})

	workflowID, err := svc.Start(context.Background(), sdlc.SDLCRequest{WorkflowID: "wf-102", ProjectID: "proj-102"})
	require.Equal(t, "wf-102", workflowID)
	require.ErrorContains(t, err, "write start audit event")

	var startErr *StartError
	require.ErrorAs(t, err, &startErr)
	require.Equal(t, "wf-102", startErr.WorkflowID)
}
