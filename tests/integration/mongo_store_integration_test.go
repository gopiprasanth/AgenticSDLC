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
	"go.mongodb.org/mongo-driver/bson"
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
	require.NoError(t, store.EnsureIndexes(ctx))

	run := sdlc.WorkflowRun{WorkflowID: "mongo-wf-1", ProjectID: "proj-1", Status: "running", Stage: sdlc.StageProduct}
	require.NoError(t, store.CreateRun(ctx, run))
	require.NoError(t, store.CreateRun(ctx, run), "create should be idempotent for same workflowId")

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

	repeatArtifactID, err := store.SaveArtifact(ctx, run.WorkflowID, "prd", []byte("# PRD"))
	require.NoError(t, err)
	require.Equal(t, artifactID, repeatArtifactID, "claim-check id should be deterministic for same payload")
	require.Contains(t, artifactID, "mongo-wf-1-prd-")

	content, err := store.LoadArtifact(ctx, artifactID)
	require.NoError(t, err)
	require.Equal(t, []byte("# PRD"), content)
}

func TestMongoStore_CreateRunRejectsConflictingDuplicate(t *testing.T) {
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
	require.NoError(t, store.EnsureIndexes(ctx))

	base := sdlc.WorkflowRun{WorkflowID: "mongo-wf-conflict", ProjectID: "proj-a", Status: "running", Stage: sdlc.StageProduct}
	require.NoError(t, store.CreateRun(ctx, base))

	conflict := base
	conflict.ProjectID = "proj-b"
	err = store.CreateRun(ctx, conflict)
	require.ErrorIs(t, err, mongostore.ErrRunAlreadyExists)
}

func TestMongoAuditStore_WritesDurableIdempotentEvents(t *testing.T) {
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

	db := client.Database("agentic")
	auditStore := mongostore.NewAuditStore(db)
	require.NoError(t, auditStore.EnsureIndexes(ctx))

	require.NoError(t, auditStore.WriteStartEvent(ctx, "wf-audit", sdlc.SDLCRequest{WorkflowID: "wf-audit", ProjectID: "proj-1", Goal: "ship"}))
	require.NoError(t, auditStore.WriteStartEvent(ctx, "wf-audit", sdlc.SDLCRequest{WorkflowID: "wf-audit", ProjectID: "proj-1", Goal: "ship"}))

	require.NoError(t, auditStore.WriteActivityEvent(ctx, sdlc.ActivityAuditEvent{EventID: "evt-1", WorkflowID: "wf-audit", Stage: sdlc.StageProduct, Status: "completed", Detail: "ok"}))
	require.NoError(t, auditStore.WriteActivityEvent(ctx, sdlc.ActivityAuditEvent{EventID: "evt-1", WorkflowID: "wf-audit", Stage: sdlc.StageProduct, Status: "completed", Detail: "ok"}))

	count, err := db.Collection("audit_events").CountDocuments(ctx, bson.M{"workflowId": "wf-audit"})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

func TestMongoStore_EnsureIndexesCreatesUniqueConstraints(t *testing.T) {
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

	db := client.Database("agentic")
	store := mongostore.NewStore(db)
	require.NoError(t, store.EnsureIndexes(ctx))

	// Validate unique workflowId index via direct duplicate insert.
	runs := db.Collection("workflow_runs")
	_, err = runs.InsertOne(ctx, bson.M{"workflowId": "dup-wf", "projectId": "p1"})
	require.NoError(t, err)
	_, err = runs.InsertOne(ctx, bson.M{"workflowId": "dup-wf", "projectId": "p2"})
	require.Error(t, err)

	// Validate unique artifactId index via direct duplicate insert.
	artifacts := db.Collection("artifacts")
	_, err = artifacts.InsertOne(ctx, bson.M{"artifactId": "dup-art", "workflowId": "wf-1", "artifactType": "prd", "content": []byte("a")})
	require.NoError(t, err)
	_, err = artifacts.InsertOne(ctx, bson.M{"artifactId": "dup-art", "workflowId": "wf-2", "artifactType": "prd", "content": []byte("b")})
	require.Error(t, err)
}
