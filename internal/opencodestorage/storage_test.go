package opencodestorage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjects_SortsByUpdatedDesc(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "storage", "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"a.json": `{"id":"a","worktree":"/a","time":{"updated":2}}`,
		"b.json": `{"id":"b","worktree":"/b","updated":3}`,
		"c.json": `{"id":"c","worktree":"/c","time":{"updated":1}}`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(projectDir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	projects, err := LoadProjects(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}
	if projects[0].ID != "b" || projects[1].ID != "a" || projects[2].ID != "c" {
		t.Fatalf("unexpected order: %+v", projects)
	}
}

func TestLoadSessions_SortsByUpdatedDescAndTitleFallback(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, "storage", "session", "p1")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"s1.json": `{"id":"s1","title":"","time":{"updated":2}}`,
		"s2.json": `{"id":"s2","title":"Hello","updated":3}`,
		"s3.json": `{"id":"s3","time":{"updated":1}}`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(sessionDir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sessions, err := LoadSessions(root, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
	if sessions[0].ID != "s2" || sessions[1].ID != "s1" || sessions[2].ID != "s3" {
		t.Fatalf("unexpected order: %+v", sessions)
	}
	if sessions[1].Title != "untitled" || sessions[2].Title != "untitled" {
		t.Fatalf("expected title fallback to untitled: %+v", sessions)
	}
}
