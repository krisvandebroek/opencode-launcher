package opencodestorage

import "context"

// Store provides access to OpenCode projects and sessions.
// Implementations should keep session loading lazy.
type Store interface {
	Projects(ctx context.Context) ([]Project, error)
	Sessions(ctx context.Context, projectID string) ([]Session, error)
	// RecentSessions returns a cross-project, newest-first list of sessions.
	// MatchText is empty.
	RecentSessions(ctx context.Context, limit int) ([]SessionSearchResult, error)
	SearchSessions(ctx context.Context, query string, limit int) ([]SessionSearchResult, error)
	Close() error
}

// WindowSearchStore optionally supports searching within a bounded window of
// the most recently updated sessions.
//
// candidateLimit controls how many sessions are considered (newest-first).
// If candidateLimit <= 0, the implementation should pick a sensible default.
type WindowSearchStore interface {
	SearchSessionsWindow(ctx context.Context, query string, limit int, candidateLimit int) ([]SessionSearchResult, error)
}
