# Specification: Focus-Only Narrow TUI Layout

## Goal
Make the `oc` TUI usable on narrow terminals by switching to a focus-only layout that preserves the speed-first, keyboard-first flow.

## User Stories
- As an experienced OpenCode user, I want the picker to remain readable on small terminals so that I can switch context quickly without resizing my window.
- As an experienced OpenCode user, I want to keep using `tab`/`shift+tab` to move between columns so that the workflow stays consistent across terminal sizes.

## Specific Requirements

**Narrow Mode: Focus-Only Layout**
- When terminal width is below the computed breakpoint, render only the focused column as a full panel (no stacked vertical multi-panel view).
- `tab`/`shift+tab` changes focus and swaps the single visible panel to the newly focused column.

**Non-Active Columns (Header-Only)**
- In narrow mode, represent non-active columns as header-only with their selected value (Option B).
- Header-only text includes the column title and the current selection (truncated as needed).

**Context Summary Line**
- In narrow mode, display a one-line context summary above the panel showing selected Project / Session / Model.
- Context summary remains visible regardless of focus.

**Filter Behavior (Persist Across Focus)**
- Filtering remains scoped to the focused column.
- Filter values persist when moving focus between columns.

**Responsive Breakpoint Calculation**
- Replace the hardcoded `width < 110` threshold with a computed breakpoint derived from minimum column widths and margins.
- Include terminal safety slack (including `OC_TUI_SAFETY_SLACK` and Ghostty slack behavior) in the breakpoint computation.

**Minimum Supported Size**
- The layout remains usable at 80x20.

## Non-Functional Requirements (Optional)
- **Performance:** Layout switching and focus changes remain instantaneous and do not introduce noticeable UI lag.

## Visual Design
No visuals provided.

## Existing Code to Leverage

**TUI layout + rendering**
- Path: `internal/tui/tui.go`
- Contains the current narrow behavior (`width < 110`) and the rendering pipeline (`View()`, `resize()`, `layout()`, `panelW()`).
- Reuse the existing sizing policy and rendering pipeline; replace stacked mode with the focus-only narrow mode.

**Focus + keyboard routing**
- Path: `internal/tui/tui.go`
- Contains focus state machine (`focusProjects`/`focusSessions`/`focusModels`), `tab`/`shift+tab` handling, and model-lock focus constraints.
- Reuse the existing focus behavior and key routing; narrow mode should only change what is rendered.

**Terminal safety slack + row clearing**
- Path: `internal/tui/tui.go`
- Contains `safetySlack()` and `inset()` logic used to avoid terminal cropping/leftovers.
- Reuse safety slack when computing the breakpoint and preserve inset trimming behavior.

## Notes & Open Questions
- Confirm the exact header-only format (e.g., `Projects: <selected>`) and truncation rules for long selections.
- Confirm whether narrow mode should also reduce two-line list items (name + description) to a single-line view or leave list rendering as-is.

## Out of Scope
- Changing project/session ordering, selection defaults, or launch behavior.
- Adding additional focus keys beyond `tab`/`shift+tab`.
- Any changes to storage parsing, model configuration, or OpenCode invocation.
