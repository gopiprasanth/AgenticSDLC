package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"agenticsdlc/internal/sdlc"
	mongostore "agenticsdlc/internal/sdlc/mongo"
	temporalsdlc "agenticsdlc/internal/sdlc/temporal"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mongocontainer "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const testTaskQueue = "agentic-e2e-task-queue"

type mongoBackedEngine struct {
	store *mongostore.Store
}

func (e *mongoBackedEngine) ExecuteProduct(ctx context.Context, req sdlc.SDLCRequest) error {
	return e.store.CreateRun(ctx, sdlc.WorkflowRun{WorkflowID: req.WorkflowID, ProjectID: req.ProjectID, Status: "running", Stage: sdlc.StageProduct})
}

func (e *mongoBackedEngine) ExecuteDeveloper(ctx context.Context, req sdlc.SDLCRequest) error {
	return e.store.UpdateRun(ctx, sdlc.WorkflowRun{WorkflowID: req.WorkflowID, ProjectID: req.ProjectID, Status: "running", Stage: sdlc.StageDeveloper})
}

func (e *mongoBackedEngine) ExecuteSecurity(ctx context.Context, req sdlc.SDLCRequest) error {
	return e.store.UpdateRun(ctx, sdlc.WorkflowRun{WorkflowID: req.WorkflowID, ProjectID: req.ProjectID, Status: "completed", Stage: sdlc.StageSecurity})
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

func TestE2E_TemporalWorkflowAndMongoStoreAreWired(t *testing.T) {
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
	store := mongostore.NewStore(mongoClient.Database("agentic"))
	auditStore := mongostore.NewAuditStore(mongoClient.Database("agentic"))
	require.NoError(t, store.EnsureIndexes(ctx))
	require.NoError(t, auditStore.EnsureIndexes(ctx))

	endpoint, err := temporalC.Endpoint(ctx, "")
	require.NoError(t, err)
	hostport := strings.TrimPrefix(endpoint, "http://")

	temporalClient, err := temporalclient.Dial(temporalclient.Options{HostPort: hostport, Namespace: "default"})
	require.NoError(t, err)
	defer temporalClient.Close()

	w := worker.New(temporalClient, testTaskQueue, worker.Options{})
	temporalsdlc.Register(w, temporalsdlc.NewActivities(&mongoBackedEngine{store: store}, auditStore))
	require.NoError(t, w.Start())
	defer w.Stop()

	workflowID := "e2e-wf-temporal-mongo"
	we, err := temporalClient.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{ID: workflowID, TaskQueue: testTaskQueue}, temporalsdlc.WorkflowName, sdlc.SDLCRequest{WorkflowID: workflowID, ProjectID: "e2e-proj"}, 1)
	require.NoError(t, err)
	require.NotEmpty(t, we.GetID())

	err = we.Get(ctx, nil)
	require.NoError(t, err)

	run, err := store.FindRun(ctx, workflowID)
	require.NoError(t, err)
	require.Equal(t, "completed", run.Status)
	require.Equal(t, sdlc.StageSecurity, run.Stage)

	count, err := mongoClient.Database("agentic").Collection("audit_events").CountDocuments(ctx, bson.M{"workflowId": workflowID})
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(3))
}
