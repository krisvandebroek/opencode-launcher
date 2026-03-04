package tui

import (
	"fmt"
	"strings"
	"time"

	"oc/internal/opencodestorage"
)

type recentSessionItem struct {
	res opencodestorage.SessionSearchResult
}

func (it recentSessionItem) Title() string {
	t := strings.TrimSpace(it.res.Session.Title)
	if t == "" {
		t = "(untitled)"
	}
	return t
}

func (it recentSessionItem) Description() string {
	updated := formatUpdatedRelative(it.res.Session.Updated)
	proj := shortenPath(it.res.ProjectWorktree, 60)
	dir := shortenPath(it.res.Session.Directory, 60)

	parts := make([]string, 0, 3)
	if updated != "" {
		parts = append(parts, updated)
	}
	if proj != "" {
		parts = append(parts, proj)
	}
	if dir != "" && dir != proj {
		parts = append(parts, dir)
	}
	return strings.Join(parts, "  ")
}

func (it recentSessionItem) FilterValue() string { return it.Title() + " " + it.Description() }

func formatUpdatedRelative(ms int64) string {
	if ms <= 0 {
		return ""
	}

	t := time.UnixMilli(ms).Local()
	d := time.Since(t)
	if d < 0 {
		d = -d
	}

	if d < 45*time.Second {
		return "just now"
	}
	if d < 90*time.Second {
		return "1m ago"
	}
	if d < 60*time.Minute {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 90*time.Minute {
		return "1h ago"
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	if d < 48*time.Hour {
		return "1d ago"
	}
	if d < 14*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Format("2006-01-02")
}
