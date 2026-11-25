package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
)

func TestRenderNeverPanicsAcrossWidths(t *testing.T) {
	for w := 20; w <= 240; w += 4 {
		for _, h := range []int{10, 16, 24, 40, 80} {
			out := Render(fullOptions(w, h, plainStyles()))
			if strings.TrimSpace(out) == "" {
				t.Fatalf("empty render at %dx%d", w, h)
			}
		}
	}
}

func TestRenderTooSmallShowsResizeHint(t *testing.T) {
	out := Render(fullOptions(40, 10, plainStyles()))
	if !strings.Contains(out, "Terminal too small") {
		t.Fatalf("missing resize hint: %q", out)
	}
}

func TestRenderLinesFitWidth(t *testing.T) {
	for _, styles := range []Styles{plainStyles(), NewStyles(true)} {
		for _, w := range []int{60, 72, 80, 96, 110, 140, 200} {
			out := Render(fullOptions(w, 44, styles))
			for i, line := range strings.Split(out, "\n") {
				if got := lipgloss.Width(line); got > w {
					t.Fatalf("width %d line %d is %d columns wide: %q", w, i, got, line)
				}
			}
		}
	}
}

func TestRenderHonoursHeight(t *testing.T) {
	for _, h := range []int{16, 24, 40, 60} {
		out := Render(fullOptions(120, h, plainStyles()))
		if got := len(strings.Split(out, "\n")); got != h {
			t.Fatalf("height %d rendered %d lines", h, got)
		}
	}
}

func TestRenderNoANSIWhenColorsDisabled(t *testing.T) {
	out := Render(fullOptions(140, 44, plainStyles()))
	if strings.Contains(out, "\x1b[") {
		t.Fatal("no-color render contains ANSI escapes")
	}
}

func TestRenderDeterministic(t *testing.T) {
	opts := fullOptions(140, 44, plainStyles())
	first := Render(opts)
	for i := 0; i < 5; i++ {
		if got := Render(opts); got != first {
			t.Fatalf("render is not deterministic at iteration %d", i)
		}
	}
}

func TestRenderShowsCoreFacts(t *testing.T) {
	out := Render(fullOptions(150, 46, plainStyles()))
	for _, want := range []string{
		"KIMI CODE", "OPENROUTER",
		"5-hour quota", "Monthly membership", "Monthly Code allocation",
		"Credits", "$42.10", "Runway", "102 days",
		"Last 30 days", "Cost by model", "claude-sonnet-4",
		"Tokens", "Daily spend", "Remaining",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render is missing %q", want)
		}
	}
}

func TestUpdatedStampIsSingleMinuteAndRightAligned(t *testing.T) {
	const width = 150
	out := Render(fullOptions(width, 46, plainStyles()))

	if got := strings.Count(out, "updated:"); got != 1 {
		t.Fatalf("want exactly one updated stamp, got %d:\n%s", got, out)
	}
	if !strings.Contains(out, "updated: 15:42") {
		t.Errorf("the stamp should have minute resolution:\n%s", out)
	}
	if strings.Contains(out, "15:42:00") {
		t.Error("the stamp must not show seconds")
	}
	if strings.Contains(out, "next ") {
		t.Error("the next-read countdown must be gone")
	}

	lines := strings.Split(out, "\n")
	row, col := findText(lines, "updated:")
	if row < 0 {
		t.Fatal("updated stamp is missing")
	}
	if col < width/2 {
		t.Errorf("the updated stamp should sit on the right, found at column %d", col)
	}
	if row != 0 {
		t.Errorf("the updated stamp belongs in the title row, found on row %d", row)
	}
}

func TestUpdatedStampUsesTheLatestRead(t *testing.T) {
	opts := fullOptions(120, 40, plainStyles())
	opts.Kimi.Snapshot.GeneratedAt = fixedNow()
	opts.OpenRouter.Snapshot.GeneratedAt = fixedNow().Add(2 * time.Minute)
	out := Render(opts)
	if !strings.Contains(out, "updated: 15:44") {
		t.Errorf("the stamp should track the most recent read:\n%s", out)
	}
}

func TestRefreshingProviderIsMarkedWithoutATimestamp(t *testing.T) {
	opts := fullOptions(120, 40, plainStyles())
	opts.Kimi.Refreshing = true
	opts.Kimi.Spinner = "⠋"
	out := Render(opts)
	if !strings.Contains(out, "refreshing") {
		t.Error("a refreshing provider should still be marked")
	}
	if got := strings.Count(out, "updated:"); got != 1 {
		t.Errorf("want exactly one updated stamp while refreshing, got %d", got)
	}
}

func TestRenderWidePutsProvidersSideBySide(t *testing.T) {
	out := Render(fullOptions(150, 46, plainStyles()))
	lines := strings.Split(out, "\n")
	kimiRow, kimiCol := findText(lines, "KIMI CODE")
	orRow, orCol := findText(lines, "OPENROUTER")
	if kimiRow < 0 || orRow < 0 {
		t.Fatalf("missing a section header:\n%s", out)
	}
	if kimiRow != orRow {
		t.Errorf("wide layout should put the section headers on one row: kimi=%d openrouter=%d", kimiRow, orRow)
	}
	if kimiCol >= orCol {
		t.Errorf("Kimi column (%d) must sit left of OpenRouter (%d)", kimiCol, orCol)
	}
}

func TestRenderNarrowStacksProviders(t *testing.T) {
	out := Render(fullOptions(70, 40, plainStyles()))
	lines := strings.Split(out, "\n")
	kimiRow, _ := findText(lines, "KIMI CODE")
	orRow, _ := findText(lines, "OPENROUTER")
	if kimiRow < 0 || orRow < 0 {
		t.Fatalf("missing a section header:\n%s", out)
	}
	if kimiRow >= orRow {
		t.Errorf("narrow layout must stack Kimi above OpenRouter: kimi=%d openrouter=%d", kimiRow, orRow)
	}
}

func TestRenderUnconfiguredProviderShowsHint(t *testing.T) {
	opts := fullOptions(120, 40, plainStyles())
	opts.Kimi = KimiView{Configured: false, Hint: "Run kimiburn setup to add a Kimi Code token."}
	opts.OpenRouter = OpenRouterView{Configured: false, Hint: "Run routeburn setup to add an OpenRouter key."}
	out := Render(opts)
	for _, want := range []string{"kimiburn setup", "routeburn setup", "not configured"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderProviderErrorKeepsOtherProvider(t *testing.T) {
	opts := fullOptions(120, 44, plainStyles())
	opts.Kimi = KimiView{Configured: true, Error: "Kimi Code is unreachable"}
	out := Render(opts)
	if !strings.Contains(out, "Kimi Code is unreachable") {
		t.Errorf("missing Kimi error banner:\n%s", out)
	}
	if !strings.Contains(out, "Credits") || !strings.Contains(out, "Daily spend") {
		t.Errorf("OpenRouter section lost after a Kimi error:\n%s", out)
	}
}

func TestMaxScrollGrowsWithContent(t *testing.T) {
	short := MaxScroll(fullOptions(120, 60, plainStyles()))
	tall := MaxScroll(fullOptions(70, 20, plainStyles()))
	if short != 0 {
		t.Errorf("a tall wide terminal should not need scrolling, got %d", short)
	}
	if tall <= 0 {
		t.Errorf("a short narrow terminal should need scrolling, got %d", tall)
	}
}

func TestHelpOverlayPreservesGeometry(t *testing.T) {
	for _, styles := range []Styles{plainStyles(), NewStyles(true)} {
		for _, size := range []struct{ width, height int }{{60, 16}, {72, 24}, {110, 40}, {140, 60}} {
			opts := fullOptions(size.width, size.height, styles)
			opts.HelpVisible = true
			opts.Version = "1.2.3"
			opts.Warnings = []string{"Kimi Code · Cache: sample warning"}
			background := Render(opts)
			out := HelpOverlay(background, opts)
			lines := strings.Split(out, "\n")
			if got := len(lines); got != size.height {
				t.Errorf("%dx%d overlay rendered %d lines, want %d", size.width, size.height, got, size.height)
			}
			for i, line := range lines {
				if got := lipgloss.Width(line); got > size.width {
					t.Errorf("%dx%d overlay line %d is %d columns wide", size.width, size.height, i, got)
				}
			}
			if !strings.Contains(out, "DASHBURN · Help") {
				t.Errorf("%dx%d overlay is missing its title", size.width, size.height)
			}
		}
	}
}

func TestHelpContentIncludesWarningsAndAbout(t *testing.T) {
	content := strings.Join(HelpContent(plainStyles(), 80, "1.2.3", []string{"Kimi Code · Cache: sample warning"}), "\n")
	if !strings.Contains(content, "Warnings") {
		t.Error("help content is missing the warnings section")
	}
	if !strings.Contains(content, "sample warning") {
		t.Error("help content is missing the warning text")
	}
	for _, want := range []string{"5-hour quota", "OpenRouter", "Runway", "About DashBurn", "Version: 1.2.3"} {
		if !strings.Contains(content, want) {
			t.Errorf("help content is missing %q", want)
		}
	}
}

func TestHelpScrollLimitGrowsWithTerminal(t *testing.T) {
	small := fullOptions(60, 16, plainStyles())
	large := fullOptions(140, 60, plainStyles())
	if HelpScrollLimit(small) <= HelpScrollLimit(large) {
		t.Errorf("a smaller terminal should need more Help scrolling: small=%d large=%d",
			HelpScrollLimit(small), HelpScrollLimit(large))
	}
}

func TestFooterAdvertisesImplementedKeys(t *testing.T) {
	footer := Footer(fullOptions(120, 40, plainStyles()))
	for _, want := range []string{"[r]", "[q]", "[?]", "[tab]"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer missing %q: %q", want, footer)
		}
	}
}

func TestSplitWidths(t *testing.T) {
	cases := []struct {
		total, n int
		want     []int
	}{
		{100, 1, []int{100}},
		{101, 2, []int{50, 50}},
		{100, 2, []int{49, 50}},
	}
	for _, c := range cases {
		got := splitWidths(c.total, c.n)
		if len(got) != len(c.want) {
			t.Fatalf("splitWidths(%d,%d) = %v", c.total, c.n, got)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("splitWidths(%d,%d) = %v, want %v", c.total, c.n, got, c.want)
			}
		}
		if sum := got[0] + (c.n - 1) + got[len(got)-1]; c.n == 2 && sum != c.total {
			t.Fatalf("splitWidths(%d,%d) = %v does not fill %d", c.total, c.n, got, c.total)
		}
	}
}

func TestJoinColumnsPadsToTheWidestLine(t *testing.T) {
	// The left block's first line is the short one, exactly like a section
	// heading above a full-width panel; both right-hand rows must still start
	// in the same column.
	left := []string{"short", "a much longer line"}
	out := joinColumns(left, []string{"R", "S"})
	first := strings.Index(out[0], "R")
	second := strings.Index(out[1], "S")
	if first < 0 || second < 0 {
		t.Fatalf("markers missing:\n%q\n%q", out[0], out[1])
	}
	if first != second {
		t.Fatalf("right block starts at column %d then %d:\n%q\n%q", first, second, out[0], out[1])
	}
}

func TestProviderColumnsStayAlignedToTheBottom(t *testing.T) {
	// The Kimi column is shorter than the OpenRouter one, so the tail rows are
	// padded blanks: the table's left border must not drift left.
	out := Render(fullOptions(150, 40, plainStyles()))
	borders := map[int]bool{}
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, "│ DATE"); i >= 0 {
			borders[lipgloss.Width(line[:i])] = true
		}
	}
	// Every row of the table shares one left edge; a single column value means
	// no row drifted.
	if len(borders) > 1 {
		t.Fatalf("the daily table's left edge is inconsistent: %v", borders)
	}
}

// findText returns the row and display column of the first occurrence of
// needle, or (-1, -1).
func findText(lines []string, needle string) (int, int) {
	for i, line := range lines {
		if idx := strings.Index(line, needle); idx >= 0 {
			return i, lipgloss.Width(line[:idx])
		}
	}
	return -1, -1
}
