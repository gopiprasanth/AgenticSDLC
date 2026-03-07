package mongo

import (
	"context"
	"fmt"
	"time"

	"agenticsdlc/internal/sdlc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AuditStore struct {
	events *mongo.Collection
}

type auditEventDocument struct {
	EventID    string    `bson:"eventId"`
	WorkflowID string    `bson:"workflowId"`
	Type       string    `bson:"type"`
	Stage      string    `bson:"stage,omitempty"`
	Status     string    `bson:"status,omitempty"`
	Detail     string    `bson:"detail,omitempty"`
	Goal       string    `bson:"goal,omitempty"`
	ProjectID  string    `bson:"projectId,omitempty"`
	CreatedAt  time.Time `bson:"createdAt"`
}

func NewAuditStore(db *mongo.Database) *AuditStore {
	return &AuditStore{events: db.Collection("audit_events")}
}

func (s *AuditStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.events.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "eventId", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("ux_audit_events_eventId"),
	})
	if err != nil {
		return fmt.Errorf("ensure audit_events indexes: %w", err)
	}
	return nil
}

func (s *AuditStore) WriteStartEvent(ctx context.Context, workflowID string, runID string, req sdlc.SDLCRequest) error {
	doc := auditEventDocument{
		EventID:    fmt.Sprintf("start-%s-%s", workflowID, runID),
		WorkflowID: workflowID,
		Type:       "workflow.start",
		Goal:       req.Goal,
		ProjectID:  req.ProjectID,
		CreatedAt:  time.Now().UTC(),
	}
	_, err := s.events.UpdateOne(
		ctx,
		bson.M{"eventId": doc.EventID},
		bson.M{"$setOnInsert": doc},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("write start event: %w", err)
	}
	return nil
}

func (s *AuditStore) WriteActivityEvent(ctx context.Context, event sdlc.ActivityAuditEvent) error {
	doc := auditEventDocument{
		EventID:    event.EventID,
		WorkflowID: event.WorkflowID,
		Type:       "activity." + string(event.Stage),
		Stage:      string(event.Stage),
		Status:     event.Status,
		Detail:     event.Detail,
		CreatedAt:  time.Now().UTC(),
	}
	_, err := s.events.UpdateOne(
		ctx,
		bson.M{"eventId": doc.EventID},
		bson.M{"$setOnInsert": doc},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("write activity event: %w", err)
	}
	return nil
}
