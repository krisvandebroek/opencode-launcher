package opencodestorage

import (
	"context"
	"database/sql"
	"net/url"
	"path/filepath"
	"testing"
)

func createTestSQLiteDB(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dbPath := filepath.Join(root, "opencode.db")

	u := &url.URL{Scheme: "file", Path: dbPath}
	q := url.Values{}
	q.Set("mode", "rwc")
	u.RawQuery = q.Encode()

	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE "project" (id TEXT PRIMARY KEY, worktree TEXT NOT NULL, time_updated INTEGER NOT NULL);`,
		`CREATE TABLE "session" (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, title TEXT NOT NULL, directory TEXT NOT NULL, time_updated INTEGER NOT NULL);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}

	return dbPath
}

func TestSQLiteStore_LoadsProjectsAndSessionsAndNormalizesTimestamps(t *testing.T) {
	dbPath := createTestSQLiteDB(t)

	// Insert timestamps in seconds.
	{
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		if _, err := db.Exec(`INSERT INTO "project" (id, worktree, time_updated) VALUES (?, ?, ?)`, "p1", "/p1", int64(2)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO "project" (id, worktree, time_updated) VALUES (?, ?, ?)`, "p2", "/p2", int64(3)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO "session" (id, project_id, title, directory, time_updated) VALUES (?, ?, ?, ?, ?)`, "s1", "p1", "", "/", int64(2)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO "session" (id, project_id, title, directory, time_updated) VALUES (?, ?, ?, ?, ?)`, "s2", "p1", "Hello", "/x", int64(3)); err != nil {
			t.Fatal(err)
		}
	}

	st, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	projects, err := st.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].ID != "p2" || projects[1].ID != "p1" {
		t.Fatalf("unexpected project order: %+v", projects)
	}
	if projects[0].Updated != 3000 || projects[1].Updated != 2000 {
		t.Fatalf("expected unix millis normalization, got %+v", projects)
	}

	sessions, err := st.Sessions(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].ID != "s2" || sessions[1].ID != "s1" {
		t.Fatalf("unexpected session order: %+v", sessions)
	}
	if sessions[1].Title != "untitled" {
		t.Fatalf("expected title fallback to untitled: %+v", sessions)
	}
	if sessions[1].Directory != "/" {
		t.Fatalf("expected directory to be parsed: %+v", sessions[1])
	}
}
