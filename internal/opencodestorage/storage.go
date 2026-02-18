package opencodestorage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Project struct {
	ID       string
	Worktree string
	Updated  int64
}

type Session struct {
	ID        string
	Title     string
	Directory string
	Updated   int64
}

func LoadProjects(storageRoot string) ([]Project, error) {
	projectDir := filepath.Join(storageRoot, "storage", "project")
	ents, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	projects := make([]Project, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if strings.HasPrefix(name, ".") {
			// Ignore editor swap/hidden files.
			continue
		}

		b, err := os.ReadFile(filepath.Join(projectDir, name))
		if err != nil {
			return nil, err
		}

		var raw struct {
			ID       string `json:"id"`
			Worktree string `json:"worktree"`
			Updated  int64  `json:"updated"`
			Time     struct {
				Updated int64 `json:"updated"`
			} `json:"time"`
		}
		if err := json.Unmarshal(b, &raw); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if strings.TrimSpace(raw.ID) == "" || strings.TrimSpace(raw.Worktree) == "" {
			continue
		}
		updated := raw.Updated
		if updated == 0 {
			updated = raw.Time.Updated
		}
		projects = append(projects, Project{ID: raw.ID, Worktree: raw.Worktree, Updated: updated})
	}

	sort.SliceStable(projects, func(i, j int) bool {
		return projects[i].Updated > projects[j].Updated
	})
	return projects, nil
}

func LoadSessions(storageRoot, projectID string) ([]Session, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("empty project id")
	}
	sessionDir := filepath.Join(storageRoot, "storage", "session", projectID)
	ents, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Session{}, nil
		}
		return nil, err
	}

	sessions := make([]Session, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}

		b, err := os.ReadFile(filepath.Join(sessionDir, name))
		if err != nil {
			return nil, err
		}

		var raw struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Directory string `json:"directory"`
			Updated   int64  `json:"updated"`
			Time      struct {
				Updated int64 `json:"updated"`
			} `json:"time"`
		}
		if err := json.Unmarshal(b, &raw); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if strings.TrimSpace(raw.ID) == "" {
			continue
		}
		title := strings.TrimSpace(raw.Title)
		if title == "" {
			title = "untitled"
		}
		dir := strings.TrimSpace(raw.Directory)
		updated := raw.Updated
		if updated == 0 {
			updated = raw.Time.Updated
		}
		sessions = append(sessions, Session{ID: raw.ID, Title: title, Directory: dir, Updated: updated})
	}

	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].Updated > sessions[j].Updated
	})
	return sessions, nil
}
