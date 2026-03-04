# Specification: Recent Sessions View (Cross-Project)

## Goal
Add a TUI view that shows all sessions across all projects in reverse-chronological order (most recent first) so recent activity is visible at a glance.

## User Stories
- As an OpenCode user, I want a single “recent sessions” list across all projects so I can quickly jump back into whatever I worked on most recently.
- As an OpenCode user, I want each recent session entry to show both the project and the session title so I can disambiguate similarly named sessions.

## Specific Requirements

**Entry Point (Top-Level Toggle)**
- Add a top-level toggle between the existing project-centric picker and a new “Recent Sessions” view.
- Default startup view remains the current project-centric picker.

**Ordering**
- Recent Sessions list is sorted by session `Updated` descending (last session first).

**Row Content**
- Each row shows:
  - Session title
  - Project identifier (worktree/path; optionally shortened for display)
  - Relative updated time (e.g. `5m ago`, `2h ago`, `3d ago`)

**Archived Sessions**
- Archived sessions are hidden by default.
- SQLite: if `time_archived` column exists, treat non-null/non-zero as archived and exclude from Recent Sessions.
- JSON legacy storage: no archived concept; therefore nothing to exclude.

**Selection Behavior**
- Pressing Enter on a Recent Sessions item launches immediately.
- Model is not chosen in this flow; we are continuing an existing session.
  - Implementation detail: the returned plan should still carry a model value (use `DefaultModel`) to satisfy current invocation shape.

**Keyboard Behavior**
- Provide explicit keys to switch views (suggested: `r` for Recent Sessions, `p` for Projects; optionally `tab` toggles).
- Existing keys and flows remain unchanged in the project-centric picker.

**Performance / Limits**
- Recent Sessions should not require loading every session for every project into memory.
- Fetch a bounded list (suggested default limit: 200) ordered by updated time.

## Existing Code to Leverage

**TUI**
- `internal/tui/tui.go`:
  - existing focus, list rendering, key routing, and the returned execution plan.
- `internal/tui/search.go`:
  - existing list item formatting for a session with project context.
  - `formatUpdated()` and `shortenPath()` helpers (used for relative time and project display).

**Storage**
- `internal/opencodestorage/store.go`: Store interface.
- `internal/opencodestorage/sqlite_store.go`: already queries sessions ordered by `time_updated` and feature-detects `time_archived`.
- `internal/opencodestorage/composite_store.go`: merge logic patterns for cross-source results.
- `internal/opencodestorage/storage.go`: JSON legacy loaders.

## Out of Scope
- Persisting per-user view preference across runs.
- Adding session transcript previews/snippets to the Recent Sessions list.
- Changing existing project/session ordering in the project-centric flow.
