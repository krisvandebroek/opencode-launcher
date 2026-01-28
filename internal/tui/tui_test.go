package tui

import "testing"

func TestChooseLayoutMode_BreakpointBoundary(t *testing.T) {
	bp := narrowBreakpointWidth(0)
	if bp <= 0 {
		t.Fatalf("expected breakpoint > 0, got %d", bp)
	}

	if got := chooseLayoutMode(bp-1, 0); got != layoutModeNarrow {
		t.Fatalf("expected narrow below breakpoint, got %v", got)
	}
	if got := chooseLayoutMode(bp, 0); got != layoutModeWide {
		t.Fatalf("expected wide at breakpoint, got %v", got)
	}
}

func TestNarrowBreakpointWidth_IncludesSafetySlack(t *testing.T) {
	base := narrowBreakpointWidth(0)
	withSlack := narrowBreakpointWidth(10)
	if withSlack-base != 10 {
		t.Fatalf("expected slack to increase breakpoint by 10, got %d", withSlack-base)
	}
}

func TestSafetySlack_EnvAndGhostty(t *testing.T) {
	var m model

	t.Run("ghostty default", func(t *testing.T) {
		t.Setenv("OC_TUI_SAFETY_SLACK", "")
		t.Setenv("TERM_PROGRAM", "ghostty")
		if got := m.safetySlack(); got != ghosttySafetySlack {
			t.Fatalf("expected ghostty slack %d, got %d", ghosttySafetySlack, got)
		}
	})

	t.Run("env overrides ghostty", func(t *testing.T) {
		t.Setenv("OC_TUI_SAFETY_SLACK", "3")
		t.Setenv("TERM_PROGRAM", "ghostty")
		if got := m.safetySlack(); got != 3 {
			t.Fatalf("expected env slack 3, got %d", got)
		}
	})

	t.Run("negative env clamps to zero", func(t *testing.T) {
		t.Setenv("OC_TUI_SAFETY_SLACK", "-5")
		t.Setenv("TERM_PROGRAM", "")
		if got := m.safetySlack(); got != 0 {
			t.Fatalf("expected env slack 0, got %d", got)
		}
	})
}
