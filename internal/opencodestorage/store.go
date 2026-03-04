package opencodestorage

import "context"

// Store provides access to OpenCode projects and sessions.
// Implementations should keep session loading lazy.
type Store interface {
	Projects(ctx context.Context) ([]Project, error)
	Sessions(ctx context.Context, projectID string) ([]Session, error)
	SearchSessions(ctx context.Context, query string, limit int) ([]SessionSearchResult, error)
	Close() error
}
