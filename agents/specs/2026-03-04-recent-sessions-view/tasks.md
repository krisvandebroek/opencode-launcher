# Implementation Plan: Recent Sessions View (Cross-Project)

## Overview
- **Primary deliverable:** A toggleable “Recent Sessions” list showing session title + project, newest-first, launching a selected session immediately.
- **User-visible behavior change:** New view + keys; existing project-centric flow remains the default.

## Global Guardrails
- No drive-by refactors.
- Preserve existing keyboard-first behavior and current launch semantics.
- Keep startup fast: do not eagerly load every session for every project.

## Task Groups

### Task Group 1: Storage API for Recent Sessions
**Goal:** Provide a single bounded, cross-project “recent sessions” list.

**Planned write touchpoints (expected files/modules):**
- `internal/opencodestorage/store.go` — add a `RecentSessions(ctx, limit)` method (or equivalent) to the Store interface.
- `internal/opencodestorage/sqlite_store.go` — implement `RecentSessions` via a single query joining `session` + `project`, ordered by `time_updated DESC`, `LIMIT ?`, excluding archived when `time_archived` exists.
- `internal/opencodestorage/json_store.go` — implement `RecentSessions` by iterating projects + session files and keeping a bounded top-N by `Updated` (heap) to avoid unbounded memory.
- `internal/opencodestorage/composite_store.go` — implement `RecentSessions` by:
  - calling primary store and (optionally) secondary store
  - de-duping by `(ProjectID, SessionID)`
  - sorting by `Updated DESC`
  - returning the top `limit`

**Implementation notes:**
- Return type: reuse `opencodestorage.SessionSearchResult` (with empty `MatchText`) or introduce a dedicated `RecentSession` struct; prefer reuse to minimize new types.
- Default limit: define at the TUI layer (e.g. 200); storage just respects caller-provided limit.

**Verification**
- `go test ./...`

**Acceptance Criteria**
- Works with SQLite-only, JSON-only, and composite (SQLite+legacy) modes.
- Archived sessions are excluded in SQLite when `time_archived` exists.
- Results are sorted newest-first and capped to `limit`.

### Task Group 2: TUI Mode + List Rendering
**Goal:** Add a toggleable Recent Sessions view and display the list.

**Planned write touchpoints (expected files/modules):**
- `internal/tui/tui.go` — add a view mode toggle and a list component for recent sessions.
- `internal/tui/search.go` — reuse the existing session list item formatting patterns where applicable (title, project path, relative updated time).

**Implementation notes:**
- Add a `viewMode` (e.g. `viewProjects`, `viewRecentSessions`).
- When entering Recent Sessions mode:
  - load recent sessions via `Store.RecentSessions(ctx, limit)`
  - populate list items showing Title + ProjectWorktree + relative Updated.
- Provide keybindings:
  - `r` -> Recent Sessions
  - `p` -> Projects
  - (optional) `tab` toggles between the two modes

**Verification**
- `go test ./...`
- Manual: `go build -o dist/oc ./cmd/oc` then run demo: `OC_STORAGE_ROOT="$PWD/demo/opencode-storage" OC_CONFIG_PATH="$PWD/demo/oc-config.yaml" ./dist/oc`

**Acceptance Criteria**
- Recent Sessions view renders a single list of sessions across all projects.
- Each row includes session title, project path, and relative time.
- Ordering is newest-first.

### Task Group 3: Launch Immediately From Recent Sessions
**Goal:** Pressing Enter on a recent session launches `opencode` with that project dir + `--session <id>`.

**Planned write touchpoints (expected files/modules):**
- `internal/tui/tui.go` — handle Enter key in Recent Sessions mode by returning a plan immediately.

**Implementation notes:**
- Returned plan should include:
  - `ProjectDir`: the selected session’s project worktree
  - `SessionID`: selected session id
  - `Model`: set to `DefaultModel` (model selection is not part of continuing a session, but the current CLI invocation includes `--model`)

**Verification**
- Manual smoke:
  1. Enter Recent Sessions view.
  2. Select a session and press Enter.
  3. Run with `--dry-run` to confirm args include `--session` and the correct project dir.

**Acceptance Criteria**
- Enter on a Recent Sessions item exits the TUI and launches that session.

### Task Group 4: Tests
**Goal:** Prevent regressions in ordering/capping and archived filtering.

**Planned write touchpoints (expected files/modules):**
- `internal/opencodestorage/sqlite_store_test.go` — add a test that `RecentSessions` returns newest-first and excludes archived when the column exists.
- `internal/opencodestorage/storage_test.go` (or new test file) — add a JSON-only test that `RecentSessions` returns newest-first and respects limit.

**Verification**
- `go test ./...`

**Acceptance Criteria**
- Tests cover ordering and limit behavior; SQLite archived filtering is covered when supported.

## Execution Order
1. Task Group 1: Storage API for Recent Sessions
2. Task Group 2: TUI Mode + List Rendering
3. Task Group 3: Launch Immediately From Recent Sessions
4. Task Group 4: Tests
