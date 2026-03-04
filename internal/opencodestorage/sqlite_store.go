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

func (s *SQLiteStore) RecentSessions(ctx context.Context, limit int) ([]SessionSearchResult, error) {
	if limit <= 0 {
		return []SessionSearchResult{}, nil
	}

	_ = s.ensureSessionColumns(ctx)

	query := `
		SELECT s.id, s.project_id, s.title, s.directory, s.time_updated, p.worktree
		FROM "session" s
		JOIN "project" p ON p.id = s.project_id
	`
	if s.sessionHasArchived {
		query += ` WHERE IFNULL(s.time_archived, 0) = 0`
	}
	query += ` ORDER BY s.time_updated DESC LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	capHint := limit
	if capHint > 64 {
		capHint = 64
	}
	out := make([]SessionSearchResult, 0, capHint)
	for rows.Next() {
		var sesID, projectID, title, dir, worktree string
		var updated int64
		if err := rows.Scan(&sesID, &projectID, &title, &dir, &updated, &worktree); err != nil {
			return nil, err
		}
		sesID = strings.TrimSpace(sesID)
		projectID = strings.TrimSpace(projectID)
		if sesID == "" || projectID == "" {
			continue
		}
		title = strings.TrimSpace(title)
		if title == "" {
			title = "untitled"
		}
		dir = strings.TrimSpace(dir)
		worktree = strings.TrimSpace(worktree)
		out = append(out, SessionSearchResult{
			ProjectID:       projectID,
			ProjectWorktree: worktree,
			Session:         Session{ID: sesID, Title: title, Directory: dir, Updated: normalizeUnixMillisFromSQLite(updated)},
			MatchText:       "",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SQLiteStore) SearchSessions(ctx context.Context, query string, limit int) ([]SessionSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return []SessionSearchResult{}, nil
	}

	like := "%" + EscapeLikePattern(query) + "%"

	// Avoid scanning the entire DB on each keystroke: search within a window of
	// most-recently-updated sessions.
	candidateLimit := limit * 500
	if candidateLimit < 1000 {
		candidateLimit = 1000
	}
	if candidateLimit > 5000 {
		candidateLimit = 5000
	}

	rows, err := s.db.QueryContext(ctx, `
		WITH candidates AS (
			SELECT id, project_id, title, directory, time_updated
			FROM "session"
			ORDER BY time_updated DESC
			LIMIT ?
		)
		SELECT s.id, s.project_id, s.title, s.directory, s.time_updated, p.worktree,
			(
				SELECT substr(json_extract(pt.data, '$.text'), 1, 20000)
				FROM "part" pt
				WHERE pt.session_id = s.id
				  AND json_extract(pt.data, '$.type') = 'text'
				  AND json_extract(pt.data, '$.text') LIKE ? ESCAPE '\'
				ORDER BY pt.time_created DESC
				LIMIT 1
			) AS match_text
		FROM candidates s
		JOIN "project" p ON p.id = s.project_id
		WHERE EXISTS (
			SELECT 1
			FROM "part" px
			WHERE px.session_id = s.id
			  AND json_extract(px.data, '$.type') = 'text'
			  AND json_extract(px.data, '$.text') LIKE ? ESCAPE '\'
		)
		ORDER BY s.time_updated DESC
		LIMIT ?
	`, candidateLimit, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	capHint := limit
	if capHint > 64 {
		capHint = 64
	}
	out := make([]SessionSearchResult, 0, capHint)
	for rows.Next() {
		var sesID, projectID, title, dir, worktree string
		var updated int64
		var match sql.NullString
		if err := rows.Scan(&sesID, &projectID, &title, &dir, &updated, &worktree, &match); err != nil {
			return nil, err
		}
		sesID = strings.TrimSpace(sesID)
		projectID = strings.TrimSpace(projectID)
		if sesID == "" || projectID == "" {
			continue
		}
		title = strings.TrimSpace(title)
		if title == "" {
			title = "untitled"
		}
		dir = strings.TrimSpace(dir)
		worktree = strings.TrimSpace(worktree)

		matchText := strings.TrimSpace(match.String)
		if matchText == "" {
			continue
		}
		out = append(out, SessionSearchResult{
			ProjectID:       projectID,
			ProjectWorktree: worktree,
			Session:         Session{ID: sesID, Title: title, Directory: dir, Updated: normalizeUnixMillisFromSQLite(updated)},
			MatchText:       matchText,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
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
	// Ensure LIKE is case-insensitive (used by session search).
	q.Add("_pragma", "case_sensitive_like(0)")
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
