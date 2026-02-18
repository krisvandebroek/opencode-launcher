package opencodestorage

import "context"

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

func (s *JSONStore) Close() error { return nil }
