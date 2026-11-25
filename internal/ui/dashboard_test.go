package ui

import (
	"strings"
	"testing"

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
