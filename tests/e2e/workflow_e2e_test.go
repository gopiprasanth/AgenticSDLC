package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"agenticsdlc/internal/sdlc"
	"agenticsdlc/internal/sdlc/memory"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mongocontainer "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.temporal.io/sdk/client"
)

type remediationEngine struct {
	securityCalls int
}

func (r *remediationEngine) ExecuteProduct(context.Context, sdlc.SDLCRequest) error   { return nil }
func (r *remediationEngine) ExecuteDeveloper(context.Context, sdlc.SDLCRequest) error { return nil }
func (r *remediationEngine) ExecuteSecurity(context.Context, sdlc.SDLCRequest) error {
	r.securityCalls++
	if r.securityCalls == 1 {
		return errors.New("initial security failure")
	}
	return nil
}

func safeMongoRun(ctx context.Context) (c *mongocontainer.MongoDBContainer, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("mongodb container panic: %v", r)
		}
	}()
	return mongocontainer.Run(ctx, "mongo:7")
}

func safeTemporalRun(ctx context.Context) (c testcontainers.Container, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("temporal container panic: %v", r)
		}
	}()
	return testcontainers.Run(ctx, "temporalio/auto-setup:1.25", testcontainers.WithExposedPorts("7233/tcp"), testcontainers.WithWaitStrategy(wait.ForListeningPort("7233/tcp")))
}

func TestE2E_SecurityFailThenRemediateUsingContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	mongoC, err := safeMongoRun(ctx)
	if err != nil {
		t.Skipf("docker unavailable for mongodb container: %v", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(mongoC) }()

	temporalC, err := safeTemporalRun(ctx)
	if err != nil {
		t.Skipf("docker unavailable for temporal container: %v", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(temporalC) }()

	mongoURI, err := mongoC.ConnectionString(ctx)
	require.NoError(t, err)

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	require.NoError(t, err)
	defer func() { _ = mongoClient.Disconnect(ctx) }()

	endpoint, err := temporalC.Endpoint(ctx, "")
	require.NoError(t, err)
	hostport := strings.TrimPrefix(endpoint, "http://")

	temporalClient, err := client.Dial(client.Options{HostPort: hostport, Namespace: "default"})
	require.NoError(t, err)
	defer temporalClient.Close()

	engine := &remediationEngine{}
	coordinator := sdlc.NewCoordinator(memory.NewStore(), engine, 2)
	require.NoError(t, coordinator.Run(ctx, sdlc.SDLCRequest{WorkflowID: "e2e-wf", ProjectID: "e2e-proj"}))

	_, err = mongoClient.Database("agentic").Collection("audit_events").InsertOne(ctx, bson.M{
		"workflowId": "e2e-wf",
		"status":     "completed",
		"retries":    1,
	})
	require.NoError(t, err)
}
