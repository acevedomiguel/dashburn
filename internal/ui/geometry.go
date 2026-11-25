// Package ui renders the DashBurn dashboard from provider snapshots. It
// performs no I/O and never fetches: every value it draws is passed in.
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// truncate cuts plain text to the terminal cell width, appending an ellipsis
// when it is cut. It is escape-aware: it measures the rendered width.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		var b strings.Builder
		used := 0
		for _, r := range s {
			rw := lipgloss.Width(string(r))
			if used+rw > width {
				break
			}
			b.WriteRune(r)
			used += rw
		}
		return b.String()
	}
	target := width - 3
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > target {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + "..."
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp01(v float64) float64 {
	if v != v || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// fitLines pads or trims a body so the frame and footer always land on the
// expected rows.
func fitLines(lines []string, height int) []string {
	if height < 1 {
		height = 1
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

// wrapPlain hard-wraps plain text without splitting escape sequences; callers
// use it only for syntax-neutral text.
func wrapPlain(s string, width int) []string {
	if width < 1 {
		return []string{""}
	}
	var lines []string
	for _, logical := range strings.Split(s, "\n") {
		if logical == "" {
			lines = append(lines, "")
			continue
		}
		var b strings.Builder
		used := 0
		for _, r := range logical {
			rw := lipgloss.Width(string(r))
			if used > 0 && used+rw > width {
				lines = append(lines, b.String())
				b.Reset()
				used = 0
			}
			b.WriteRune(r)
			used += rw
		}
		lines = append(lines, b.String())
	}
	return lines
}
