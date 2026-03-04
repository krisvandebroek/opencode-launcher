package opencodestorage

import "strings"

// SessionSearchResult is a session plus minimal context for global search.
//
// MatchText is a small chunk of transcript text (typically a single message
// part) that contains the query; the TUI is responsible for formatting it into
// a one-line snippet.
type SessionSearchResult struct {
	ProjectID       string
	ProjectWorktree string
	Session         Session
	MatchText       string
}

func EscapeLikePattern(s string) string {
	// Escape characters that have special meaning in SQL LIKE patterns.
	// We use backslash as the escape character.
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
