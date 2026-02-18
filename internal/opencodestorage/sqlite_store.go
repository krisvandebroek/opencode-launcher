package opencodestorage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db     *sql.DB
	dbPath string

	colsOnce           sync.Once
	sessionHasArchived bool
	colsErr            error
}

func OpenSQLiteStore(dbPath string) (*SQLiteStore, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("empty db path")
	}
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		abs = dbPath
	}

	u := &url.URL{Scheme: "file", Path: abs}
	q := url.Values{}
	q.Set("mode", "ro")
	// Keep reads safe and avoid accidental writes.
	q.Add("_pragma", "query_only(1)")
	// Avoid transient SQLITE_BUSY failures when OpenCode has it open.
	q.Add("_pragma", "busy_timeout(2000)")
	u.RawQuery = q.Encode()

	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	// Validate the handle early (helps catch missing/corrupt DB).
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	// This CLI does a couple small queries; keep the pool minimal.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return &SQLiteStore{db: db, dbPath: abs}, nil
}

func (s *SQLiteStore) Projects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, worktree, time_updated FROM "project" ORDER BY time_updated DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var id, worktree string
		var updated int64
		if err := rows.Scan(&id, &worktree, &updated); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		worktree = strings.TrimSpace(worktree)
		if id == "" || worktree == "" {
			continue
		}
		projects = append(projects, Project{ID: id, Worktree: worktree, Updated: normalizeUnixMillisFromSQLite(updated)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return projects, nil
}

func (s *SQLiteStore) Sessions(ctx context.Context, projectID string) ([]Session, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("empty project id")
	}

	if err := s.ensureSessionColumns(ctx); err != nil {
		// If PRAGMA fails, continue without archived column support.
		// (We currently don't filter on archived; this is just schema tolerance.)
	}

	query := `SELECT id, title, directory, time_updated FROM "session" WHERE project_id = ? ORDER BY time_updated DESC`
	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []Session{}
	for rows.Next() {
		var id, title, dir string
		var updated int64
		if err := rows.Scan(&id, &title, &dir, &updated); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		title = strings.TrimSpace(title)
		if title == "" {
			title = "untitled"
		}
		dir = strings.TrimSpace(dir)
		sessions = append(sessions, Session{ID: id, Title: title, Directory: dir, Updated: normalizeUnixMillisFromSQLite(updated)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (s *SQLiteStore) ensureSessionColumns(ctx context.Context) error {
	s.colsOnce.Do(func() {
		rows, err := s.db.QueryContext(ctx, `PRAGMA table_info("session")`)
		if err != nil {
			s.colsErr = err
			return
		}
		defer rows.Close()
		for rows.Next() {
			// cid, name, type, notnull, dflt_value, pk
			var cid int
			var name, ctype string
			var notnull int
			var dflt sql.NullString
			var pk int
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				s.colsErr = err
				return
			}
			if strings.EqualFold(strings.TrimSpace(name), "time_archived") {
				s.sessionHasArchived = true
			}
		}
		s.colsErr = rows.Err()
	})
	return s.colsErr
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
