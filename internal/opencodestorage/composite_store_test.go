package opencodestorage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func writeJSONProject(t *testing.T, root, name, json string) {
	t.Helper()
	dir := filepath.Join(root, "storage", "project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJSONSession(t *testing.T, root, projectID, name, json string) {
	t.Helper()
	dir := filepath.Join(root, "storage", "session", projectID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(json), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompositeStore_MergesAndPrefersSQLiteOnDuplicateIDs(t *testing.T) {
	root := t.TempDir()
	dbPath := createTestSQLiteDB(t)

	// JSON has p1 and session s1.
	writeJSONProject(t, root, "p1.json", `{"id":"p1","worktree":"/json","time":{"updated":1}}`)
	writeJSONSession(t, root, "p1", "s1.json", `{"id":"s1","title":"from-json","directory":"/","time":{"updated":1}}`)

	// SQLite has same IDs but should win.
	{
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.Exec(`INSERT INTO "project" (id, worktree, time_updated) VALUES (?, ?, ?)`, "p1", "/db", int64(10)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO "session" (id, project_id, title, directory, time_updated) VALUES (?, ?, ?, ?, ?)`, "s1", "p1", "from-db", "/", int64(10)); err != nil {
			t.Fatal(err)
		}
	}

	st, err := OpenStore(OpenOptions{StorageRoot: root, DBPath: dbPath, UseLegacy: true})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	projects, err := st.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d: %+v", len(projects), projects)
	}
	if projects[0].ID != "p1" || projects[0].Worktree != "/db" {
		t.Fatalf("expected sqlite project to win: %+v", projects[0])
	}

	sessions, err := st.Sessions(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d: %+v", len(sessions), sessions)
	}
	if sessions[0].ID != "s1" || sessions[0].Title != "from-db" {
		t.Fatalf("expected sqlite session to win: %+v", sessions[0])
	}
}

func TestCompositeStore_GlobalCollisionProducesCanonicalAndAliasProjects(t *testing.T) {
	root := t.TempDir()
	dbPath := createTestSQLiteDB(t)

	// Legacy JSON has canonical global (worktree="/").
	writeJSONProject(t, root, "global.json", `{"id":"global","worktree":"/","time":{"updated":1}}`)

	// SQLite has an id=global project pointing at a real path.
	wt := "/Users/alice/work/test-project"
	{
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.Exec(`INSERT INTO "project" (id, worktree, time_updated) VALUES (?, ?, ?)`, "global", wt, int64(10)); err != nil {
			t.Fatal(err)
		}
	}

	st, err := OpenStore(OpenOptions{StorageRoot: root, DBPath: dbPath, UseLegacy: true})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	projects, err := st.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d: %+v", len(projects), projects)
	}

	var haveCanonical bool
	var haveAlias bool
	aliasID := syntheticGlobalProjectID(wt)
	for _, p := range projects {
		if p.ID == "global" && p.Worktree == "/" {
			haveCanonical = true
		}
		if p.ID == aliasID && p.Worktree == wt {
			haveAlias = true
		}
	}
	if !haveCanonical {
		t.Fatalf("expected canonical global project to exist: %+v", projects)
	}
	if !haveAlias {
		t.Fatalf("expected alias project for global collision to exist: %+v", projects)
	}
}

func TestCompositeStore_SyntheticGlobalSessionsFilteredByDirectoryPrefix(t *testing.T) {
	root := t.TempDir()
	dbPath := createTestSQLiteDB(t)

	// Provide canonical global and a collision alias.
	writeJSONProject(t, root, "global.json", `{"id":"global","worktree":"/","time":{"updated":1}}`)
	wt := "/Users/alice/work/test-project"
	{
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.Exec(`INSERT INTO "project" (id, worktree, time_updated) VALUES (?, ?, ?)`, "global", wt, int64(10)); err != nil {
			t.Fatal(err)
		}
	}

	// Sessions live under project_id "global".
	writeJSONSession(t, root, "global", "s1.json", `{"id":"s1","title":"in","directory":"/Users/alice/work/test-project","time":{"updated":2}}`)
	writeJSONSession(t, root, "global", "s2.json", `{"id":"s2","title":"in-sub","directory":"/Users/alice/work/test-project/subdir","time":{"updated":3}}`)
	writeJSONSession(t, root, "global", "s3.json", `{"id":"s3","title":"out","directory":"/Users/alice/work/other","time":{"updated":4}}`)

	st, err := OpenStore(OpenOptions{StorageRoot: root, DBPath: dbPath, UseLegacy: true})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// Build alias mapping.
	_, err = st.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	aliasID := syntheticGlobalProjectID(wt)
	filtered, err := st.Sessions(context.Background(), aliasID)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered sessions, got %d: %+v", len(filtered), filtered)
	}
	if filtered[0].ID != "s2" || filtered[1].ID != "s1" {
		t.Fatalf("expected filtered sessions sorted by updated desc: %+v", filtered)
	}

	all, err := st.Sessions(context.Background(), "global")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 global sessions, got %d: %+v", len(all), all)
	}
}

func TestCompositeStore_FallsBackToJSONWhenSQLiteCannotOpen(t *testing.T) {
	root := t.TempDir()
	writeJSONProject(t, root, "p1.json", `{"id":"p1","worktree":"/json","time":{"updated":1}}`)

	st, err := OpenStore(OpenOptions{StorageRoot: root, DBPath: filepath.Join(root, "does-not-exist.db"), UseLegacy: true})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	projects, err := st.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Worktree != "/json" {
		t.Fatalf("expected json-only projects, got %+v", projects)
	}
}

func TestCheckStorageReadable_OKIfDBReadableEvenWhenJSONMissing(t *testing.T) {
	storageRoot := t.TempDir()
	dbPath := createTestSQLiteDB(t)

	// Make JSON unreadable by not creating storage/project.
	if err := CheckStorageReadable(storageRoot, dbPath, false, false); err != nil {
		t.Fatalf("expected readable via sqlite, got %v", err)
	}
}

func TestCheckStorageReadable_DisableSQLiteRequiresJSON(t *testing.T) {
	storageRoot := t.TempDir()
	dbPath := createTestSQLiteDB(t)
	if err := CheckStorageReadable(storageRoot, dbPath, true, true); err == nil {
		t.Fatalf("expected error when sqlite disabled and json unreadable")
	}
}

func TestCheckStorageReadable_DefaultSQLiteOnlyErrorsWhenDBMissing(t *testing.T) {
	storageRoot := t.TempDir()
	dbPath := filepath.Join(storageRoot, "missing.db")
	if err := CheckStorageReadable(storageRoot, dbPath, false, false); err == nil {
		t.Fatalf("expected error when sqlite missing in sqlite-only mode")
	}
}
