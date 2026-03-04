package opencodestorage

import (
	"context"
	"fmt"
)

type JSONStore struct {
	StorageRoot string
}

func NewJSONStore(storageRoot string) *JSONStore {
	return &JSONStore{StorageRoot: storageRoot}
}

func (s *JSONStore) Projects(ctx context.Context) ([]Project, error) {
	_ = ctx
	return LoadProjects(s.StorageRoot)
}

func (s *JSONStore) Sessions(ctx context.Context, projectID string) ([]Session, error) {
	_ = ctx
	return LoadSessions(s.StorageRoot, projectID)
}

func (s *JSONStore) SearchSessions(ctx context.Context, query string, limit int) ([]SessionSearchResult, error) {
	_ = ctx
	_ = query
	_ = limit
	return nil, fmt.Errorf("session text search requires SQLite (opencode.db)")
}

func (s *JSONStore) Close() error { return nil }
