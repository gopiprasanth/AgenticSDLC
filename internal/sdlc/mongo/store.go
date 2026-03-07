package mongo

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"agenticsdlc/internal/sdlc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrRunNotFound = errors.New("workflow run not found")
var ErrArtifactNotFound = errors.New("artifact not found")
var ErrRunAlreadyExists = errors.New("workflow run already exists with different data")

type Store struct {
	runs      *mongo.Collection
	artifacts *mongo.Collection
}

type runDocument struct {
	WorkflowID string    `bson:"workflowId"`
	ProjectID  string    `bson:"projectId"`
	Status     string    `bson:"status"`
	Attempt    int       `bson:"attempt"`
	Stage      string    `bson:"stage"`
	LastError  string    `bson:"lastError"`
	UpdatedAt  time.Time `bson:"updatedAt"`
}

type artifactDocument struct {
	ArtifactID   string    `bson:"artifactId"`
	WorkflowID   string    `bson:"workflowId"`
	ArtifactType string    `bson:"artifactType"`
	Content      []byte    `bson:"content"`
	CreatedAt    time.Time `bson:"createdAt"`
}

func NewStore(db *mongo.Database) *Store {
	return &Store{
		runs:      db.Collection("workflow_runs"),
		artifacts: db.Collection("artifacts"),
	}
}

func (s *Store) EnsureIndexes(ctx context.Context) error {
	if _, err := s.runs.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "workflowId", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("ux_workflow_runs_workflowId"),
	}); err != nil {
		return fmt.Errorf("ensure workflow_runs indexes: %w", err)
	}

	if _, err := s.artifacts.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "artifactId", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("ux_artifacts_artifactId"),
	}); err != nil {
		return fmt.Errorf("ensure artifacts indexes: %w", err)
	}

	return nil
}

func (s *Store) CreateRun(ctx context.Context, run sdlc.WorkflowRun) error {
	doc := runDocument{
		WorkflowID: run.WorkflowID,
		ProjectID:  run.ProjectID,
		Status:     run.Status,
		Attempt:    run.Attempt,
		Stage:      string(run.Stage),
		LastError:  run.LastError,
		UpdatedAt:  time.Now().UTC(),
	}
	result, err := s.runs.UpdateOne(
		ctx,
		bson.M{"workflowId": run.WorkflowID},
		bson.M{"$setOnInsert": doc},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("upsert workflow run: %w", err)
	}
	if result.UpsertedCount == 1 {
		return nil
	}

	existing, err := s.FindRun(ctx, run.WorkflowID)
	if err != nil {
		return fmt.Errorf("verify existing workflow run: %w", err)
	}
	if existing.ProjectID != run.ProjectID || existing.Status != run.Status || existing.Attempt != run.Attempt || existing.Stage != run.Stage || existing.LastError != run.LastError {
		return ErrRunAlreadyExists
	}
	return nil
}

func (s *Store) UpdateRun(ctx context.Context, run sdlc.WorkflowRun) error {
	update := bson.M{
		"$set": bson.M{
			"projectId": run.ProjectID,
			"status":    run.Status,
			"attempt":   run.Attempt,
			"stage":     string(run.Stage),
			"lastError": run.LastError,
			"updatedAt": time.Now().UTC(),
		},
	}
	result, err := s.runs.UpdateOne(ctx, bson.M{"workflowId": run.WorkflowID}, update)
	if err != nil {
		return fmt.Errorf("update workflow run: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrRunNotFound
	}
	return nil
}

func (s *Store) FindRun(ctx context.Context, workflowID string) (sdlc.WorkflowRun, error) {
	var doc runDocument
	if err := s.runs.FindOne(ctx, bson.M{"workflowId": workflowID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return sdlc.WorkflowRun{}, ErrRunNotFound
		}
		return sdlc.WorkflowRun{}, fmt.Errorf("find workflow run: %w", err)
	}
	return sdlc.WorkflowRun{
		WorkflowID: doc.WorkflowID,
		ProjectID:  doc.ProjectID,
		Status:     doc.Status,
		Attempt:    doc.Attempt,
		Stage:      sdlc.Stage(doc.Stage),
		LastError:  doc.LastError,
	}, nil
}

func (s *Store) SaveArtifact(ctx context.Context, workflowID string, artifactType string, content []byte) (string, error) {
	hash := sha256.Sum256(content)
	artifactID := workflowID + "-" + artifactType + "-" + fmt.Sprintf("%x", hash[:])
	doc := artifactDocument{
		ArtifactID:   artifactID,
		WorkflowID:   workflowID,
		ArtifactType: artifactType,
		Content:      content,
		CreatedAt:    time.Now().UTC(),
	}
	_, err := s.artifacts.UpdateOne(
		ctx,
		bson.M{"artifactId": artifactID},
		bson.M{"$setOnInsert": doc},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return "", fmt.Errorf("upsert artifact: %w", err)
	}
	return artifactID, nil
}

func (s *Store) LoadArtifact(ctx context.Context, artifactID string) ([]byte, error) {
	var doc artifactDocument
	if err := s.artifacts.FindOne(ctx, bson.M{"artifactId": artifactID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrArtifactNotFound
		}
		return nil, fmt.Errorf("load artifact: %w", err)
	}
	return doc.Content, nil
}
