package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"agenticsdlc/internal/sdlc"
	mongostore "agenticsdlc/internal/sdlc/mongo"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mongocontainer "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func safeMongoContainer(ctx context.Context) (c *mongocontainer.MongoDBContainer, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("mongodb container panic: %v", r)
		}
	}()
	return mongocontainer.Run(ctx, "mongo:7")
}

func TestMongoStore_RunAndArtifactClaimCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mongoC, err := safeMongoContainer(ctx)
	if err != nil {
		t.Skipf("docker unavailable for mongodb container: %v", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(mongoC) }()

	uri, err := mongoC.ConnectionString(ctx)
	require.NoError(t, err)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	require.NoError(t, err)
	defer func() { _ = client.Disconnect(ctx) }()

	store := mongostore.NewStore(client.Database("agentic"))

	run := sdlc.WorkflowRun{WorkflowID: "mongo-wf-1", ProjectID: "proj-1", Status: "running", Stage: sdlc.StageProduct}
	require.NoError(t, store.CreateRun(ctx, run))

	run.Stage = sdlc.StageSecurity
	run.Status = "completed"
	require.NoError(t, store.UpdateRun(ctx, run))

	loaded, err := store.FindRun(ctx, run.WorkflowID)
	require.NoError(t, err)
	require.Equal(t, "completed", loaded.Status)
	require.Equal(t, sdlc.StageSecurity, loaded.Stage)

	artifactID, err := store.SaveArtifact(ctx, run.WorkflowID, "prd", []byte("# PRD"))
	require.NoError(t, err)
	require.NotEmpty(t, artifactID)

	content, err := store.LoadArtifact(ctx, artifactID)
	require.NoError(t, err)
	require.Equal(t, []byte("# PRD"), content)
}
