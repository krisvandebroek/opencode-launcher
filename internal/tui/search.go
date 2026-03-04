package tui

import (
	"strings"
	"unicode"

	"oc/internal/opencodestorage"
)

type searchResultsMsg struct {
	query   string
	results []opencodestorage.SessionSearchResult
	err     error
}

type searchTickMsg struct {
	seq   int
	query string
}

type sessionSearchItem struct {
	res     opencodestorage.SessionSearchResult
	queryLC string
}

func (it sessionSearchItem) Title() string {
	t := strings.TrimSpace(it.res.Session.Title)
	if t == "" {
		t = "(untitled)"
	}
	return t
}

func (it sessionSearchItem) Description() string {
	updated := formatUpdated(it.res.Session.Updated)
	proj := shortenPath(it.res.ProjectWorktree, 50)
	dir := shortenPath(it.res.Session.Directory, 40)
	snippet := excerptMatch(it.res.MatchText, it.queryLC, 90)

	parts := make([]string, 0, 4)
	if updated != "" {
		parts = append(parts, updated)
	}
	if proj != "" {
		parts = append(parts, proj)
	}
	if dir != "" && dir != proj {
		parts = append(parts, dir)
	}
	if snippet != "" {
		parts = append(parts, snippet)
	}
	return strings.Join(parts, "  ")
}

func (it sessionSearchItem) FilterValue() string { return it.Title() + " " + it.Description() }

func excerptMatch(text string, queryLower string, maxLen int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 80
	}

	// Normalize to a single line for list rendering.
	text = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, text)
	text = strings.Join(strings.Fields(text), " ")

	q := strings.TrimSpace(queryLower)
	if q == "" {
		return truncatePlain(text, maxLen)
	}
	idx := strings.Index(strings.ToLower(text), q)
	if idx < 0 {
		return truncatePlain(text, maxLen)
	}

	before := 28
	after := maxLen - before - 1
	if after < 16 {
		after = 16
	}
	start := idx - before
	if start < 0 {
		start = 0
	}
	end := idx + len(q) + after
	if end > len(text) {
		end = len(text)
	}
	chunk := text[start:end]
	if start > 0 {
		chunk = "..." + strings.TrimLeft(chunk, " ")
	}
	if end < len(text) {
		chunk = strings.TrimRight(chunk, " ") + "..."
	}
	return truncatePlain(chunk, maxLen)
}

func truncatePlain(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
