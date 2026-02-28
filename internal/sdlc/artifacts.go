package sdlc

import "context"

type ArtifactStore interface {
	SaveArtifact(ctx context.Context, workflowID string, artifactType string, content []byte) (string, error)
	LoadArtifact(ctx context.Context, artifactID string) ([]byte, error)
}
