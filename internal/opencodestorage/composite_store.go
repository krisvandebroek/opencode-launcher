package opencodestorage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type CompositeStore struct {
	json   Store
	sqlite Store

	mu      sync.RWMutex
	aliases map[string]projectAlias
}

type projectAlias struct {
	baseProjectID string
	dirPrefix     string
}

type OpenOptions struct {
	StorageRoot   string
	DBPath        string
	UseLegacy     bool
	DisableSQLite bool
}

// OpenStore opens the appropriate store for the configured data sources.
//
// Default behavior (UseLegacy=false): SQLite-only.
// Legacy behavior (UseLegacy=true): JSON + (optionally) SQLite merged.
func OpenStore(opts OpenOptions) (Store, error) {
	if !opts.UseLegacy {
		if opts.DisableSQLite {
			return nil, fmt.Errorf("sqlite disabled and legacy disabled")
		}
		st, err := OpenSQLiteStore(opts.DBPath)
		if err != nil {
			return nil, err
		}
		return st, nil
	}

	jsonStore := Store(NewJSONStore(opts.StorageRoot))
	var sqliteStore Store
	if !opts.DisableSQLite {
		if s, err := OpenSQLiteStore(opts.DBPath); err == nil {
			sqliteStore = s
		}
	}
	return &CompositeStore{json: jsonStore, sqlite: sqliteStore, aliases: map[string]projectAlias{}}, nil
}

func (s *CompositeStore) Projects(ctx context.Context) ([]Project, error) {
	haveSQLite := s.sqlite != nil
	haveJSON := s.json != nil
	if !haveSQLite && !haveJSON {
		return nil, fmt.Errorf("no storage sources configured")
	}

	var dbProjects []Project
	var jsonProjects []Project
	var dbErr error
	var jsonErr error

	if haveSQLite {
		dbProjects, dbErr = s.sqlite.Projects(ctx)
	}
	if haveJSON {
		jsonProjects, jsonErr = s.json.Projects(ctx)
	}

	if haveSQLite && dbErr == nil && (!haveJSON || jsonErr != nil) {
		return dbProjects, nil
	}
	if haveJSON && jsonErr == nil && (!haveSQLite || dbErr != nil) {
		return jsonProjects, nil
	}
	if haveSQLite && haveJSON && dbErr == nil && jsonErr == nil {
		merged, aliases := mergeProjectsWithGlobalAliases(dbProjects, jsonProjects)
		s.mu.Lock()
		s.aliases = aliases
		s.mu.Unlock()
		sort.SliceStable(merged, func(i, j int) bool { return merged[i].Updated > merged[j].Updated })
		return merged, nil
	}
	if haveSQLite && haveJSON {
		return nil, fmt.Errorf("sqlite: %v; json: %v", dbErr, jsonErr)
	}
	if haveSQLite {
		return nil, dbErr
	}
	return nil, jsonErr
}

func (s *CompositeStore) Sessions(ctx context.Context, projectID string) ([]Session, error) {
	if baseID, prefix, ok := s.resolveAlias(projectID); ok {
		all, err := s.sessionsBase(ctx, baseID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(prefix) == "" {
			return all, nil
		}
		filtered := make([]Session, 0, len(all))
		for _, ses := range all {
			if dirWithinPrefix(ses.Directory, prefix) {
				filtered = append(filtered, ses)
			}
		}
		return filtered, nil
	}

	return s.sessionsBase(ctx, projectID)
}

func (s *CompositeStore) sessionsBase(ctx context.Context, projectID string) ([]Session, error) {
	haveSQLite := s.sqlite != nil
	haveJSON := s.json != nil
	if !haveSQLite && !haveJSON {
		return nil, fmt.Errorf("no storage sources configured")
	}

	var dbSessions []Session
	var jsonSessions []Session
	var dbErr error
	var jsonErr error

	if haveSQLite {
		dbSessions, dbErr = s.sqlite.Sessions(ctx, projectID)
	}
	if haveJSON {
		jsonSessions, jsonErr = s.json.Sessions(ctx, projectID)
	}

	if haveSQLite && dbErr == nil && (!haveJSON || jsonErr != nil) {
		return dbSessions, nil
	}
	if haveJSON && jsonErr == nil && (!haveSQLite || dbErr != nil) {
		return jsonSessions, nil
	}
	if haveSQLite && haveJSON && dbErr == nil && jsonErr == nil {
		merged := mergeSessionsPreferFirst(dbSessions, jsonSessions)
		sort.SliceStable(merged, func(i, j int) bool { return merged[i].Updated > merged[j].Updated })
		return merged, nil
	}
	if haveSQLite && haveJSON {
		return nil, fmt.Errorf("sqlite: %v; json: %v", dbErr, jsonErr)
	}
	if haveSQLite {
		return nil, dbErr
	}
	return nil, jsonErr
}

func (s *CompositeStore) resolveAlias(projectID string) (baseID, prefix string, ok bool) {
	s.mu.RLock()
	a, ok := s.aliases[projectID]
	s.mu.RUnlock()
	if !ok {
		return "", "", false
	}
	return a.baseProjectID, a.dirPrefix, true
}

func mergeProjectsWithGlobalAliases(primary, secondary []Project) ([]Project, map[string]projectAlias) {
	aliases := map[string]projectAlias{}
	// Only apply special Global collision handling when we can identify the
	// canonical Global project (id="global", worktree="/") from at least one
	// source.
	if _, ok := pickCanonicalGlobal(primary, secondary); !ok {
		return mergeProjectsPreferFirst(primary, secondary), aliases
	}

	// Merge all non-global projects by ID (primary wins).
	byID := make(map[string]Project, len(primary)+len(secondary))
	order := make([]string, 0, len(primary)+len(secondary))
	add := func(p Project) {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return
		}
		if _, ok := byID[id]; ok {
			return
		}
		byID[id] = p
		order = append(order, id)
	}

	for _, p := range primary {
		if strings.TrimSpace(p.ID) == "global" {
			continue
		}
		add(p)
	}
	for _, p := range secondary {
		if strings.TrimSpace(p.ID) == "global" {
			continue
		}
		add(p)
	}

	out := make([]Project, 0, len(byID)+2)
	for _, id := range order {
		out = append(out, byID[id])
	}

	// Ensure canonical Global exists: id=global, worktree="/".
	canonical, _ := pickCanonicalGlobal(primary, secondary)
	out = append(out, canonical)

	// Handle collisions: id=global but worktree != "/".
	collisions := collectGlobalCollisionsPreferFirst(primary, secondary)
	for _, p := range collisions {
		worktree := strings.TrimSpace(p.Worktree)
		if worktree == "" || worktree == "/" {
			continue
		}
		synthID := syntheticGlobalProjectID(worktree)
		out = append(out, Project{ID: synthID, Worktree: worktree, Updated: p.Updated})
		aliases[synthID] = projectAlias{baseProjectID: "global", dirPrefix: worktree}
	}

	return out, aliases
}

func pickCanonicalGlobal(primary, secondary []Project) (Project, bool) {
	for _, p := range primary {
		if strings.TrimSpace(p.ID) == "global" && strings.TrimSpace(p.Worktree) == "/" {
			return p, true
		}
	}
	for _, p := range secondary {
		if strings.TrimSpace(p.ID) == "global" && strings.TrimSpace(p.Worktree) == "/" {
			return p, true
		}
	}
	return Project{}, false
}

func maxUpdatedForGlobal(primary, secondary []Project) int64 {
	max := int64(0)
	for _, p := range primary {
		if strings.TrimSpace(p.ID) == "global" && p.Updated > max {
			max = p.Updated
		}
	}
	for _, p := range secondary {
		if strings.TrimSpace(p.ID) == "global" && p.Updated > max {
			max = p.Updated
		}
	}
	return max
}

func collectGlobalCollisionsPreferFirst(primary, secondary []Project) []Project {
	byWorktree := map[string]Project{}
	order := []string{}
	add := func(p Project) {
		if strings.TrimSpace(p.ID) != "global" {
			return
		}
		wt := strings.TrimSpace(p.Worktree)
		if wt == "" || wt == "/" {
			return
		}
		if _, ok := byWorktree[wt]; ok {
			return
		}
		byWorktree[wt] = p
		order = append(order, wt)
	}
	for _, p := range primary {
		add(p)
	}
	for _, p := range secondary {
		add(p)
	}

	out := make([]Project, 0, len(order))
	for _, wt := range order {
		out = append(out, byWorktree[wt])
	}
	return out
}

func syntheticGlobalProjectID(worktree string) string {
	// Stable, deterministic ID that won't collide with the canonical global.
	// Keep it short for display/debugging.
	sum := sha256.Sum256([]byte(strings.TrimSpace(worktree)))
	return "global-alias-" + hex.EncodeToString(sum[:6])
}

func dirWithinPrefix(dir, prefix string) bool {
	dir = strings.TrimSpace(dir)
	prefix = strings.TrimSpace(prefix)
	if dir == "" || prefix == "" {
		return false
	}
	// Normalize both paths without touching the filesystem.
	dir = filepath.Clean(dir)
	prefix = filepath.Clean(prefix)
	if dir == prefix {
		return true
	}
	if !strings.HasPrefix(dir, prefix) {
		return false
	}
	// Require a path boundary so "/foo/bar" doesn't match "/foo/barista".
	next := dir[len(prefix)]
	return next == '/' || next == '\\'
}

func mergeProjectsPreferFirst(primary, secondary []Project) []Project {
	out := make([]Project, 0, len(primary)+len(secondary))
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	for _, p := range primary {
		out = append(out, p)
		seen[p.ID] = struct{}{}
	}
	for _, p := range secondary {
		if _, ok := seen[p.ID]; ok {
			continue
		}
		out = append(out, p)
		seen[p.ID] = struct{}{}
	}
	return out
}

func mergeSessionsPreferFirst(primary, secondary []Session) []Session {
	out := make([]Session, 0, len(primary)+len(secondary))
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	for _, s := range primary {
		out = append(out, s)
		seen[s.ID] = struct{}{}
	}
	for _, s := range secondary {
		if _, ok := seen[s.ID]; ok {
			continue
		}
		out = append(out, s)
		seen[s.ID] = struct{}{}
	}
	return out
}

func (s *CompositeStore) Close() error {
	var err error
	if s.sqlite != nil {
		err = s.sqlite.Close()
	}
	if s.json != nil {
		_ = s.json.Close()
	}
	return err
}
