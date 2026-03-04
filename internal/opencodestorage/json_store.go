package opencodestorage

import (
	"container/heap"
	"context"
	"fmt"
	"sort"
	"strings"
)

type JSONStore struct {
	StorageRoot string
}

func NewJSONStore(storageRoot string) *JSONStore {
	return &JSONStore{StorageRoot: storageRoot}
}

func (s *JSONStore) Projects(ctx context.Context) ([]Project, error) {
	_ = ctx
	return LoadProjects(s.StorageRoot)
}

func (s *JSONStore) Sessions(ctx context.Context, projectID string) ([]Session, error) {
	_ = ctx
	return LoadSessions(s.StorageRoot, projectID)
}

type recentSessionsMinHeap []SessionSearchResult

func (h recentSessionsMinHeap) Len() int { return len(h) }
func (h recentSessionsMinHeap) Less(i, j int) bool {
	// Min-heap by Updated.
	return h[i].Session.Updated < h[j].Session.Updated
}
func (h recentSessionsMinHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *recentSessionsMinHeap) Push(x any)   { *h = append(*h, x.(SessionSearchResult)) }
func (h *recentSessionsMinHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func (s *JSONStore) RecentSessions(ctx context.Context, limit int) ([]SessionSearchResult, error) {
	_ = ctx
	if limit <= 0 {
		return []SessionSearchResult{}, nil
	}

	projects, err := LoadProjects(s.StorageRoot)
	if err != nil {
		return nil, err
	}

	h := make(recentSessionsMinHeap, 0, minInt(limit, 64))
	heap.Init(&h)

	for _, p := range projects {
		pid := strings.TrimSpace(p.ID)
		wt := strings.TrimSpace(p.Worktree)
		if pid == "" || wt == "" {
			continue
		}
		sessions, err := LoadSessions(s.StorageRoot, pid)
		if err != nil {
			return nil, err
		}
		for _, ses := range sessions {
			if strings.TrimSpace(ses.ID) == "" {
				continue
			}
			r := SessionSearchResult{ProjectID: pid, ProjectWorktree: wt, Session: ses, MatchText: ""}
			if h.Len() < limit {
				heap.Push(&h, r)
				continue
			}
			if h.Len() > 0 && r.Session.Updated > h[0].Session.Updated {
				_ = heap.Pop(&h)
				heap.Push(&h, r)
			}
		}
	}

	out := make([]SessionSearchResult, 0, h.Len())
	for h.Len() > 0 {
		out = append(out, heap.Pop(&h).(SessionSearchResult))
	}
	// Heap pops oldest-first; sort newest-first.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Session.Updated > out[j].Session.Updated })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *JSONStore) SearchSessions(ctx context.Context, query string, limit int) ([]SessionSearchResult, error) {
	_ = ctx
	_ = query
	_ = limit
	return nil, fmt.Errorf("session text search requires SQLite (opencode.db)")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *JSONStore) Close() error { return nil }
