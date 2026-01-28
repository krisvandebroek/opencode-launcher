# Spec Requirements: 2026-01-27-tui-focus-only-layout

## Initial Description
Review the UX, keeping in mind the @agents/product/mission.md.

The first screenshot is a failed UX when the screen size is limited.
In this case it's probably better that the non-active columns only contain the title, reducing the width and allowing it to be on a small screen.
Tab will then make the next column visible and hide previous.

## Requirements Discussion

### First Round Questions

**Q1:** I assume the “narrow terminal” behavior should switch from the current stacked-vertical 3-panels layout to a focus-only layout (only the active column is fully visible). Is that correct, or do you want stacked mode kept as an option?
**Answer:** yes

**Q2:** For non-active columns in narrow mode, I’m assuming we should either (A) fully hide them, or (B) show header-only with the selected value (e.g., `Projects: oc`). Which do you prefer?
**Answer:** B

**Q3:** I assume `tab` / `shift+tab` should continue to change focus, and in narrow mode it should “swap” the single visible panel to the newly-focused column. Any additional keys (e.g., left/right) you want to support for focus switching?
**Answer:** tab/shift+tab

**Q4:** I’m assuming we should keep “type to filter” scoped to the focused column only (current behavior). When you tab away, should the previous column’s filter persist or be cleared?
**Answer:** yes and persist

**Q5:** In narrow mode, should we add a one-line “current context” summary (Project / Session / Model) at the top so state is always visible without tabbing back?
**Answer:** yes

**Q6:** Breakpoint: I’m assuming we should compute the threshold from min column widths + margins (and include `OC_TUI_SAFETY_SLACK` / Ghostty slack) rather than a hardcoded `width < 110`. Is that correct?
**Answer:** yes

**Q7:** What’s the minimum terminal width/height you want to explicitly support (e.g., 80x24)? Any specific terminal(s) you care about beyond Ghostty?
**Answer:** 80x20

**Q8:** Exclusions: I assume this spec should not change sorting, selection defaults, or the launch plan—only layout/rendering and related UX hints. Anything else explicitly out of scope?
**Answer:** no extra scope

### Existing Code to Reference

**User-Provided Similar Features (if any):**
None provided by user.

**Agent-Suggested Candidates (from quick codebase scan):**
- TUI layout + rendering - Path: `internal/tui/tui.go`
  - Why it seems relevant: contains `resize()`, `layout()`, `panelW()`, and the current `width < 110` stacked behavior.
  - What to potentially reuse: existing sizing policy and rendering pipeline, replacing stacked mode with focus-only behavior.
- Focus/navigation handling - Path: `internal/tui/tui.go`
  - Why it seems relevant: `tab`/`shift+tab` focus cycling, model-lock focus constraints, and key routing between list vs filter.
  - What to potentially reuse: existing focus state machine and key handling; keep behavior consistent while changing rendering.
- Width safety behavior - Path: `internal/tui/tui.go`
  - Why it seems relevant: `safetySlack()` and inset/trim logic that affects narrow terminal rendering.
  - What to potentially reuse: existing safety slack mechanism when computing the responsive breakpoint.

### Follow-up Questions
None.

## Visual Assets

### Files Provided:
No visual assets provided.

## Requirements Summary

### Functional Requirements
- Introduce a narrow-terminal layout mode that uses a focus-only flow instead of stacking all panels vertically.
- In narrow mode, show the active column fully and render non-active columns as header-only (Option B).
- Keep keyboard behavior: `tab` / `shift+tab` cycles focus; focus change swaps the visible column.
- Keep filters scoped to the focused column and persist filter values across focus changes.
- Add a one-line context summary (Project / Session / Model) at the top in narrow mode.
- Compute the breakpoint from min widths + margins and include terminal safety slack rather than using a hardcoded width.
- Explicitly support terminals down to 80x20.

### Reusability Opportunities
- Agent-suggested candidates: `internal/tui/tui.go` for layout, sizing, focus handling, and safety slack behavior.

### Scope Boundaries
**In Scope:**
- Layout/rendering changes and UX hints needed to support narrow terminals, preserving keyboard-first flow.

**Out of Scope:**
- Changes to sorting, selection defaults, model/session logic, or the launch plan.

### Technical Considerations
- Replace the current `width < 110` stacked-vertical behavior with a computed-breakpoint focus-only mode.
- Ensure the layout remains usable at 80x20 without horizontal clipping.
- Preserve existing focus state machine and model-locked behavior while changing rendering.
