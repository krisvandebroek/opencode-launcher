# Task Breakdown: Focus-Only Narrow TUI Layout

## Overview
- **Total task groups:** 3
- **Primary deliverable:** Narrow terminals render a focus-only picker with header-only non-active columns, a context summary line, and persistent per-column filters.
- **Single source of truth:** This `tasks.md` defines the intended scope boundaries for implementation (touchpoints, standards, verification).

## Global Guardrails
- No drive-by refactors. Only changes required to satisfy checked-off tasks.
- Implementation must stay within the touchpoints listed per task group. If new touchpoints are needed, STOP and get explicit approval before editing additional files.
- Keep behavior keyboard-first (no mouse interactions).

## Task Groups

### Task Group 1: Define Narrow-Mode Contract + Breakpoint
**Dependencies:** None

**Standards to follow (selected after reading `agents/standards/`):**
- None found in `agents/standards/**/*` in this repo.

**Planned write touchpoints (expected files/modules):**
- `internal/tui/tui.go` — replace the hardcoded `width < 110` checks with a computed breakpoint; introduce/centralize a “layout mode” decision.

**Planned new files (if any):**
- None

**Verification**
- **Commands (repo-documented):**
  - `go test ./...`

- [x] 1.0 Narrow-mode contract and breakpoint is defined
  - [x] 1.1 Decide header-only format + truncation rules for selected values (keep it single-line and stable)
  - [x] 1.2 Add a computed breakpoint helper based on min column widths + margins + `safetySlack()`
  - [x] 1.3 Ensure breakpoint logic accounts for Ghostty and `OC_TUI_SAFETY_SLACK`
  - [x] 1.4 Verification
    - [x] Run `go test ./...`

**Acceptance Criteria:**
- No remaining `width < 110` hardcoded layout threshold checks.
- Breakpoint calculation is derived from `minColWProjects`, `minColWSessions`, `minColWModel`, margins/gaps, and `safetySlack()`.
- Breakpoint logic is used consistently wherever layout mode is decided.

### Task Group 2: Implement Focus-Only Rendering + Header-Only Non-Active Columns
**Dependencies:** Task Group 1

**Standards to follow (selected after reading `agents/standards/`):**
- None found in `agents/standards/**/*` in this repo.

**Planned write touchpoints (expected files/modules):**
- `internal/tui/tui.go` — update `View()`, `resize()`, and `layout()` to render focus-only mode; add context summary line; preserve keyboard routing and filter persistence.

**Planned new files (if any):**
- None

**Verification**
- **Commands (repo-documented):**
  - `go test ./...`
  - `go build -o dist/oc ./cmd/oc`
- **Manual smoke checks (needed for TUI layout):**
  1. Run the demo: `OC_STORAGE_ROOT="$PWD/demo/opencode-storage" OC_CONFIG_PATH="$PWD/demo/oc-config.yaml" ./dist/oc`.
  2. Resize the terminal to ~80x20.
  3. Confirm narrow mode shows only one full panel (focused column) and shows header-only selections for the other columns.
  4. Confirm a one-line Project/Session/Model context summary is visible above the panel.
  5. Type a filter in Projects, `tab` to Sessions, then `shift+tab` back; confirm the Projects filter persists.
  6. `tab`/`shift+tab` cycles focus and swaps the visible panel accordingly.

- [ ] 2.0 Focus-only mode renders correctly on narrow terminals
  - [x] 2.1 Replace stacked vertical multi-panel rendering with focus-only rendering when below breakpoint
  - [x] 2.2 Render non-active columns as header-only with selected value (Option B)
  - [x] 2.3 Add a one-line context summary (Project / Session / Model) in narrow mode
  - [x] 2.4 Preserve existing focus rules (including model-locked behavior) and persist filters across focus changes
  - [x] 2.5 Ensure layout is usable at 80x20 (no horizontal clipping; stable refresh)
  - [ ] 2.6 Verification
    - [x] Run `go test ./...`
    - [x] Run `go build -o dist/oc ./cmd/oc`
    - [ ] Perform the manual smoke checks

**Acceptance Criteria:**
- At ~80x20, the UI does not stack all three panels vertically.
- Only the focused column renders as a full panel; switching focus swaps which column is shown.
- Non-active columns appear as header-only with selected values.
- A one-line context summary shows Project/Session/Model and remains visible during focus changes.
- Filters are scoped to the focused column and persist when tabbing between columns.

### Task Group 3: Add Targeted Tests for Breakpoint + Mode Selection
**Dependencies:** Task Group 2

**Standards to follow (selected after reading `agents/standards/`):**
- None found in `agents/standards/**/*` in this repo.

**Planned write touchpoints (expected files/modules):**
- `internal/tui/tui.go` — add small helpers (if needed) to make breakpoint/mode selection testable without snapshotting the whole TUI.

**Planned new files (if any):**
- `internal/tui/tui_test.go` — unit tests covering breakpoint computation and layout mode selection boundaries.

**Verification**
- **Commands (repo-documented):**
  - `go test ./...`

- [x] 3.0 Breakpoint and layout mode behavior is covered by tests
  - [x] 3.1 Add unit tests for: below breakpoint => narrow mode; at/above breakpoint => three-column mode
  - [x] 3.2 Add unit tests for: safety slack influences breakpoint as expected (including Ghostty slack)
  - [x] 3.3 Verification
    - [x] Run `go test ./...`

**Acceptance Criteria:**
- Tests exist for layout mode selection boundaries (narrow vs non-narrow).
- Tests exercise safety slack influence on breakpoint/mode.
- `go test ./...` passes.

## Execution Order
1. Task Group 1: Define Narrow-Mode Contract + Breakpoint
2. Task Group 2: Implement Focus-Only Rendering + Header-Only Non-Active Columns
3. Task Group 3: Add Targeted Tests for Breakpoint + Mode Selection
