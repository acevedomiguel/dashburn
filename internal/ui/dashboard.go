package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/acevedomiguel/kimiburn"
	"github.com/acevedomiguel/routeburn"
)

// KimiView is the Kimi Code rendering state for one frame.
type KimiView struct {
	Snapshot    *kimiburn.Snapshot
	History     map[kimiburn.QuotaKey][]kimiburn.Sample
	Refreshing  bool
	Spinner     string
	Error       string
	NextRefresh time.Time
	Configured  bool
	Hint        string
}

// OpenRouterView is the OpenRouter rendering state for one frame.
type OpenRouterView struct {
	Snapshot     *routeburn.Snapshot
	Refreshing   bool
	Spinner      string
	Error        string
	NextRefresh  time.Time
	Configured   bool
	Hint         string
	TableOffset  int
	TableFocused bool
}

// ViewOptions is the complete, pure description of one dashboard frame.
type ViewOptions struct {
	Width  int
	Height int
	Styles Styles
	Now    time.Time

	Scroll int

	Version     string
	HelpVisible bool
	HelpScroll  int
	Warnings    []string
	// Notice is a transient one-line hint shown in the footer, such as a
	// throttle message.
	Notice string

	Kimi       KimiView
	OpenRouter OpenRouterView
}

// titleText is the centred dashboard title.
const titleText = "DASHBURN · Kimi Code + OpenRouter · live"

// Render returns an exact-height frame with the footer pinned to the last row.
func Render(opts ViewOptions) string {
	if opts.Width <= 0 {
		opts.Width = 80
	}
	if opts.Height <= 0 {
		opts.Height = 24
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	lay := Classify(opts.Width, opts.Height)
	if lay.Class == ClassTooSmall {
		return renderTooSmall(opts)
	}

	bodyHeight := maxInt(opts.Height-2, 1)
	body := composeBody(opts, lay, bodyHeight)
	maxOffset := maxInt(len(body)-bodyHeight, 0)
	offset := clampInt(opts.Scroll, 0, maxOffset)
	window := fitLines(body[offset:minInt(offset+bodyHeight, len(body))], bodyHeight)

	lines := make([]string, 0, opts.Height)
	lines = append(lines, titleRow(opts))
	for _, line := range window {
		lines = append(lines, padLine(line, lay))
	}
	lines = fitLines(lines, opts.Height-1)

	right := opts.Notice
	if right == "" && maxOffset > 0 {
		right = fmt.Sprintf("line %d/%d", offset+1, len(body))
	}
	lines = append(lines, footerBar(opts, right))
	return strings.Join(lines[:opts.Height], "\n")
}

// MaxScroll reports the largest useful scroll offset for the current frame.
func MaxScroll(opts ViewOptions) int {
	if opts.Width <= 0 {
		opts.Width = 80
	}
	if opts.Height <= 0 {
		opts.Height = 24
	}
	lay := Classify(opts.Width, opts.Height)
	if lay.Class == ClassTooSmall {
		return 0
	}
	bodyHeight := maxInt(opts.Height-2, 1)
	body := composeBody(opts, lay, bodyHeight)
	return maxInt(len(body)-bodyHeight, 0)
}

// composeBody builds the whole scrollable body of the dashboard.
func composeBody(opts ViewOptions, lay Layout, bodyHeight int) []string {
	width := lay.ContentWidth
	if lay.TwoColumn {
		widths := splitWidths(width, 2)
		left := kimiSection(opts, widths[0], bodyHeight)
		right := openRouterSection(opts, widths[1], bodyHeight)
		return joinColumns(left, right)
	}
	left := kimiSection(opts, width, bodyHeight)
	right := openRouterSection(opts, width, bodyHeight)
	return stackBlocks([][]string{left, right})
}

// titleRow centres the app title across the terminal width.
func titleRow(opts ViewOptions) string {
	return lipgloss.PlaceHorizontal(opts.Width, lipgloss.Center, opts.Styles.Title.Render(truncate(titleText, opts.Width)))
}

// padLine applies the layout padding so every body row spans the terminal.
func padLine(line string, lay Layout) string {
	content := padRight(line, lay.ContentWidth)
	if lay.Padding > 0 {
		content = strings.Repeat(" ", lay.Padding) + content + strings.Repeat(" ", lay.Padding)
	}
	return padRight(content, lay.Width)
}

// Footer renders the pinned bottom bar with no right-hand hint.
func Footer(opts ViewOptions) string { return footerBar(opts, "") }

// footerBar renders the pinned bottom bar: the help bubble's key hints on the
// left and an optional status hint on the right.
func footerBar(opts ViewOptions, right string) string {
	width := maxInt(opts.Width, 1)
	h := NewHelp(opts.Styles, maxInt(width-2, 1))
	h.ShowAll = false
	var left string
	if opts.HelpVisible {
		left = h.View(NewOverlayKeyMap())
	} else {
		left = h.View(NewKeyMap())
	}

	contentWidth := maxInt(width-2, 1)
	bar := left
	if right != "" {
		gap := contentWidth - lipgloss.Width(left) - lipgloss.Width(right)
		if gap < 1 {
			gap = 1
		}
		bar = left + strings.Repeat(" ", gap) + opts.Styles.Muted.Render(right)
	}
	// The help bubble does not always fit its hints, so clip the composed bar
	// escape-safely before the status bar paints its background.
	bar = ansi.Truncate(bar, contentWidth, "…")
	bar = padRight(bar, contentWidth)
	out := opts.Styles.StatusBar.Render(bar)
	return padRight(out, width)
}

// renderTooSmall fills the screen with the resize hint.
func renderTooSmall(opts ViewOptions) string {
	width, height := maxInt(opts.Width, 1), maxInt(opts.Height, 1)
	lines := strings.Split(lipgloss.Wrap(TooSmallMessage, width, " "), "\n")
	lines = fitLines(lines, height)
	for i := range lines {
		lines[i] = padRight(lines[i], width)
	}
	return strings.Join(lines, "\n")
}
