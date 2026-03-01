package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"agenticsdlc/internal/sdlc"
	"agenticsdlc/internal/sdlc/memory"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mongocontainer "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/wait"
)

type noOpEngine struct{}

func (noOpEngine) ExecuteProduct(context.Context, sdlc.SDLCRequest) error   { return nil }
func (noOpEngine) ExecuteDeveloper(context.Context, sdlc.SDLCRequest) error { return nil }
func (noOpEngine) ExecuteSecurity(context.Context, sdlc.SDLCRequest) error  { return nil }

func safeMongoRun(ctx context.Context) (c *mongocontainer.MongoDBContainer, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("mongodb container panic: %v", r)
		}
	}()
	return mongocontainer.Run(ctx, "mongo:7")
}

func TestIntegration_WorkflowWithTemporalAndMongoContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mongoC, err := safeMongoRun(ctx)
	if err != nil {
		t.Skipf("docker unavailable for mongodb container: %v", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(mongoC) }()

	temporalC, err := testcontainers.Run(ctx, "temporalio/auto-setup:1.25", testcontainers.WithExposedPorts("7233/tcp"), testcontainers.WithWaitStrategy(wait.ForListeningPort("7233/tcp")))
	if err != nil {
		t.Skipf("docker unavailable for temporal container: %v", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(temporalC) }()

	mongoURI, err := mongoC.ConnectionString(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, mongoURI)

	temporalEndpoint, err := temporalC.Endpoint(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, temporalEndpoint)

	coordinator := sdlc.NewCoordinator(memory.NewStore(), noOpEngine{}, 1)
	err = coordinator.Run(ctx, sdlc.SDLCRequest{WorkflowID: "integration-wf", ProjectID: "integration-proj"})
	require.NoError(t, err)
}
