package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agenticsdlc/internal/sdlc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var ErrRunNotFound = errors.New("workflow run not found")
var ErrArtifactNotFound = errors.New("artifact not found")

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
	_, err := s.runs.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("insert workflow run: %w", err)
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
	artifactID := fmt.Sprintf("%s-%s-%d", workflowID, artifactType, time.Now().UTC().UnixNano())
	doc := artifactDocument{
		ArtifactID:   artifactID,
		WorkflowID:   workflowID,
		ArtifactType: artifactType,
		Content:      content,
		CreatedAt:    time.Now().UTC(),
	}
	_, err := s.artifacts.InsertOne(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("insert artifact: %w", err)
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
