# Specification: Session Text Search (Ctrl+F)

## Goal
Enable a global, keyboard-first session search inside the `oc` TUI: press `ctrl+f`, type a keyword, and get sessions (newest first) where that keyword appears in the session transcript. Each result shows the session folder, session title, and a short transcript snippet.

## User Stories
- As an OpenCode user, I want to press `ctrl+f` to quickly find a past session by a phrase I remember from the conversation.
- As an OpenCode user, I want results ordered by most recently updated so I can jump to the most relevant recent session first.
- As an OpenCode user, I want to see a snippet so I can confirm I found the right session before launching it.

## Specific Requirements

**Keybinding**
- `ctrl+f` opens a search overlay (or dedicated search panel) from anywhere in the TUI.
- `esc` closes search and returns to the previous focus/selection state.

**Search Input**
- Search query is a case-insensitive substring match.
- Empty query shows no results (or a short hint line) and does not run DB search.
- While search is open, normal list filtering for Projects/Sessions/Model is suspended; input goes to the search box.

**Search Scope**
- Search runs across sessions for all projects (including Global).

**Results**
- Results are ordered by `session.time_updated` descending.
- Each result displays:
  - Project folder/worktree (shortened path)
  - Session title
  - Session directory (where available; especially useful for Global)
  - A short snippet of transcript text containing the match
- Selecting a result and pressing `enter` launches OpenCode into that session immediately.

**Launch Behavior**
- Launch uses the same invocation path as the existing picker: `opencode <worktree> --model <selected-model> --session <session-id>`.

**Storage + Querying**
- Transcript search is backed by SQLite (`opencode.db`) only.
- The DB is opened read-only and must remain read-only.
- Use the existing `part_session_idx` index to keep searches responsive:
  - Filter candidate sessions by recent update time (ordered by `time_updated DESC`)
  - For each candidate session, search `part` rows for `type='text'` where `text` contains the query
  - Return the first (most recent) matching text part as the snippet

**Loading + Errors**
- While a search is running, show a lightweight loading state (e.g. "Searching...").
- If SQLite is disabled/unavailable, show a short error in the search UI and no results.

## Existing Code to Leverage

**TUI model + key handling**
- Path: `internal/tui/tui.go`
- Extend the key handler (`tea.KeyMsg`) to open/close a search overlay and to route keys to a search textinput + results list.

**SQLite access**
- Path: `internal/opencodestorage/sqlite_store.go`
- Add a query method that returns session search results, including worktree + snippet.

## Notes
- SQLite schema discovery on a real DB shows:
  - `project(id, worktree, time_updated)`
  - `session(id, project_id, directory, title, time_updated, ...)`
  - `part(session_id, data TEXT JSON, time_created, ...)` with `data.type='text'` and `data.text` holding transcript text.

## Out of Scope
- Regex search, fuzzy matching, highlighting, or advanced query syntax.
- Creating/searching an FTS index (DB must remain read-only).
- Searching legacy JSON-only transcripts.
