# SQLite support migration plan (OpenCode `opencode.db` + legacy JSON)

**Repo:** `oc` (opencode-launcher)  
**Goal:** show *all* OpenCode projects + sessions by reading **both**:
- legacy JSON storage under `<storageRoot>/storage/{project,session}/...`
- new SQLite DB at `~/.local/share/opencode/opencode.db` (path relative to `storageRoot`)

**Critical constraint (for this change request):** no CGO; cross-platform (darwin/linux amd64/arm64); startup time matters; keep binary size reasonable.

---

## Recommendation

### Driver
Use **`modernc.org/sqlite`** as the SQLite driver.

**Why it best fits the constraints**
- **No CGO:** pure Go SQLite (`database/sql` driver).
- **Cross-platform:** explicitly supports darwin/linux amd64/arm64 (and many more).
- **Startup/runtime performance:** generally faster and lower overhead than Wasm-based approaches for short-lived CLI reads.
- **Operational simplicity:** `database/sql` driver, straightforward DSN, stable behavior in read-only mode.

**Why not the main alternatives**
- **`github.com/ncruces/go-sqlite3`** (wazero + embedded Wasm): also CGO-free and very portable, but has **higher per-process initialization overhead** (Wasm runtime/module instantiation) and typically **higher memory overhead**. It is a good fallback option if modernc causes problems, but not my first pick for a “lightning-fast” launcher.
- **`github.com/glebarez/go-sqlite`**: pure Go, but comparatively less active and essentially another facade over the same “port-of-C” lineage; less attractive than using modernc directly.
- **`zombiezen.com/go/sqlite`**: solid, CGO-free, but **not** a `database/sql` driver; we would write more plumbing for no meaningful win here (we only need a couple queries).

### Schema target (what we will support)
The DB contains (at least):

- table **`project`** (singular)
  - `id` (TEXT PK)
  - `worktree` (TEXT NOT NULL)
  - `time_updated` (INTEGER NOT NULL) — seconds or milliseconds
- table **`session`** (singular)
  - `id` (TEXT PK)
  - `project_id` (TEXT NOT NULL)
  - `title` (TEXT NOT NULL)
  - `directory` (TEXT NOT NULL)
  - `time_updated` (INTEGER NOT NULL) — seconds or milliseconds
  - `time_archived` (INTEGER, nullable) — optional hiding

Implementation should be tolerant to:
- timestamps in seconds vs milliseconds
- missing optional columns (e.g., `time_archived`) by feature-detecting via `PRAGMA table_info`.

---

## Impact assessment

### Binary size increase (estimate)
Adding a pure-Go SQLite engine is the biggest cost of this migration.

- **modernc.org/sqlite**: expect **~+6 MB to +15 MB** on stripped, static, `CGO_ENABLED=0` binaries (varies by Go version/linker trimming and exact dependency versions).
- For comparison, **ncruces/go-sqlite3** often lands in a similar ballpark, sometimes smaller, sometimes larger, but tends to have higher runtime init overhead.

Mitigation: keep the SQLite integration minimal (no ORM), and keep using release flags `-trimpath -ldflags "-s -w"`.

### Compile time impact (estimate)
- modernc builds will add noticeable compile time in CI and for contributors: typically **+10s to +60s** depending on cache warmth and machine.
- Mitigate by keeping the integration small and relying on Go build cache.

### Operational overhead
- Use read-only DB access (`mode=ro`) and `PRAGMA query_only=ON`.
- Handle lock contention with `busy_timeout`.
- Expect schema changes over time; keep queries centralized and feature-detect columns.

---

## Data strategy

### DB location rules
Assuming `oc` already computes `storageRoot` (default `~/.local/share/opencode`, override via `OC_STORAGE_ROOT` or `--storage`). Add DB path resolution:

1. If `OC_DB_PATH` is set (new env var), use it.
2. Else if `--db` is provided (new flag), use it.
3. Else default to `filepath.Join(storageRoot, "opencode.db")`.

**Fallbacks**
- If DB does not exist/unreadable: proceed with legacy JSON only.
- If DB exists but is corrupt/unopenable: proceed with legacy JSON only unless JSON yields zero projects, in which case return an actionable error.

Escape hatch:
- `OC_DISABLE_SQLITE=1` skips DB entirely.

### Merging JSON + SQLite results

Identity + de-dupe keys:
- **Projects:** key by `Project.ID`. If ID appears in both, **prefer SQLite**.
- **Sessions:** key by `Session.ID` (scoped to project). If ID appears in both, **prefer SQLite**.

If IDs differ but worktree matches: keep both (avoid accidental merges).

Sort order:
- Projects sorted by updated desc.
- Sessions sorted by updated desc.

Timestamp normalization:
- If `time_updated > 99_999_999_999` treat as ms.
- Else treat as seconds and multiply by 1000.

Query strategy:
- Startup: one query to list projects.
- On-demand: sessions are lazy-loaded per project; keep this behavior.

---

## Refactor plan

### Introduce a Store interface
To open SQLite once and keep session loading lazy:

```go
type Store interface {
    Projects(ctx context.Context) ([]Project, error)
    Sessions(ctx context.Context, projectID string) ([]Session, error)
    Close() error
}
```

Implementations:
- `JSONStore` (wrap existing logic)
- `SQLiteStore` (new)
- `CompositeStore` (new; merges JSON + SQLite)

Update call sites:
- `cmd/oc/main.go` constructs a `CompositeStore` and passes it into the TUI.
- `internal/tui` uses `store.Sessions(ctx, projectID)` instead of calling JSON loader directly.

### SQLiteStore implementation details
- Open read-only DSN (modernc):
  - `file:/path/to/opencode.db?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(2000)`
- Feature-detect `time_archived` via `PRAGMA table_info(session)`.

SQL:

Projects:

```sql
SELECT id, worktree, time_updated
FROM "project"
ORDER BY time_updated DESC;
```

Sessions:

```sql
SELECT id, title, directory, time_updated
FROM "session"
WHERE project_id = ?
ORDER BY time_updated DESC;
```

### Update CheckStorageReadable
Semantics: OK if JSON is readable OR DB is readable. Error only if both missing/unreadable.

---

## Risks + mitigations

| Risk | Impact | Mitigation |
| ---- | ------ | ---------- |
| Binary grows noticeably | Larger downloads; slightly slower cold start | Keep integration minimal; keep stripping; consider optional build tag only if necessary |
| DB locked / SQLITE_BUSY | Projects/sessions intermittently missing | `busy_timeout`; retry once with small backoff |
| Schema changes | Breaks DB reading | Centralize SQL; `PRAGMA table_info` / feature detection |
| Archived session semantics unclear | Either clutter or hiding too much | Start by showing all; later add filtering behind a flag once confirmed |
| Exec replaces process so defers do not run | DB not cleanly closed | Open read-only; rely on OS close on exec |

---

## Milestones

### 0) Confirm schema + define contract
- Document supported schema: `project`, `session` and key columns.
- Decide initial policy (recommend: show all sessions).
- Define knobs: `OC_DB_PATH`, `--db`, `OC_DISABLE_SQLITE=1`.

### 1) Add dependency + minimal SQLite reader
- Add `modernc.org/sqlite` dependency.
- Implement `SQLiteStore` with read-only open, queries, timestamp normalization.
- Unit tests create a temp DB file and validate results.

### 2) Add Store interface + JSONStore wrapper
- Wrap existing JSON functions in `JSONStore`.
- Keep legacy functions temporarily if helpful.

### 3) CompositeStore merge + update call sites
- Implement merge logic (DB wins on duplicate IDs).
- Update `cmd/oc/main.go` and `internal/tui` to use the store.

### 4) Robustness
- Fallback behavior; optional debug logging.
- Busy retry/backoff.
- Disable-sqlite switch.

### 5) Verification + release readiness
- CI tests for JSON-only, DB-only, merged.
- Optional: add a demo DB (or generate during tests) for manual validation.

---

## Test / verification plan

CI tests in `internal/opencodestorage`:
- SQLite timestamp normalization.
- SQLite projects and sessions load/sort.
- Composite merge: DB wins on duplicates.
- CheckStorageReadable DB-only behavior.
- Fallback to JSON when DB open fails.

Local manual:
- With real storage, ensure DB-only projects show up and sessions load.
- `OC_DISABLE_SQLITE=1` reproduces legacy behavior.

---

## Appendix: mapping

Projects: `id`, `worktree`, `time_updated` -> `Project{ID, Worktree, Updated(ms)}`

Sessions: `id`, `title`, `directory`, `time_updated` -> `Session{ID, Title (fallback untitled), Directory, Updated(ms)}`
