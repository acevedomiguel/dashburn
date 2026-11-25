package ui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// HelpContent is the Help document: what the panels mean, the key bindings
// rendered from the same KeyMap the handlers use, and the current warnings.
func HelpContent(st Styles, width int, version string, warnings []string) []string {
	textWidth := maxInt(width, 20)
	if version == "" {
		version = "dev"
	}
	lines := []string{
		st.Title.Render("What the panels mean"),
		"DashBurn shows two independent providers side by side. It only reads data; it never sends inference requests and never stores a cache.",
		"",
		st.Title.Render("Kimi Code"),
		"5-hour quota and monthly membership are separate windows; the monthly Code allocation is included in the shared membership, never added to it.",
		"Runway and Burn are estimated locally from retained in-memory readings and are bounded by reset time.",
		"Usage is a sparkline of the readings retained for the current reset period (this process only).",
		"The timeline runs from now to the reset: ✕ marks the estimated depletion, ✓ means the quota lasts past the reset.",
		"",
		st.Title.Render("OpenRouter"),
		"Credits, spend, tokens, and cost-by-model come from the management key; day buckets are UTC to match OpenRouter.",
		"Runway is balance divided by the average daily spend over the previous 30 complete UTC days.",
		"The daily table lists one row per UTC day, newest first; Tab focuses it and j/k scroll it.",
		"",
		st.Title.Render("Keys"),
	}
	h := NewHelp(st, textWidth)
	h.ShowAll = true
	lines = append(lines, strings.Split(h.View(NewKeyMap()), "\n")...)
	lines = append(lines, "", st.Title.Render("Warnings"))
	if len(warnings) == 0 {
		lines = append(lines, "None")
	}
	for _, warning := range warnings {
		lines = append(lines, st.Warning.Render(warning))
	}
	lines = append(lines, "",
		st.Title.Render("About DashBurn"),
		"DashBurn is an independent, read-only dashboard available under the MIT license.",
		"Version: "+version,
		"It reads the credentials kimiburn and routeburn already store.",
	)

	var out []string
	for _, line := range lines {
		out = append(out, strings.Split(lipgloss.Wrap(line, textWidth, " "), "\n")...)
	}
	return out
}

// HelpTextWidth is the wrap width of the Help document for a terminal width.
func HelpTextWidth(width int) int {
	_, _, textWidth := helpGeometry(width, 0)
	return textWidth
}

// helpGeometry sizes the popup for a terminal: at most 88 columns wide and
// leaving the title row and footer row of the dashboard visible.
func helpGeometry(width, height int) (popupWidth, popupHeight, textWidth int) {
	popupWidth = minInt(88, width-4)
	if popupWidth < 20 {
		popupWidth = maxInt(width, 20)
	}
	popupHeight = maxInt(height-4, 5)
	textWidth = maxInt(popupWidth-4, 1)
	return popupWidth, popupHeight, textWidth
}

// HelpScrollLimit reports the largest useful popup offset for this terminal.
func HelpScrollLimit(opts ViewOptions) int {
	_, popupHeight, textWidth := helpGeometry(opts.Width, opts.Height)
	lines := HelpContent(opts.Styles, textWidth, opts.Version, opts.Warnings)
	return maxInt(len(lines)-(popupHeight-2), 0)
}

// HelpOverlay centres a bordered, scrollable Help popup above the dashboard
// frame with the lipgloss compositor. The canvas keeps the terminal size, so
// the dashboard and its pinned footer stay on the same rows underneath.
func HelpOverlay(background string, opts ViewOptions) string {
	st := opts.Styles
	popupWidth, popupHeight, textWidth := helpGeometry(opts.Width, opts.Height)
	content := HelpContent(st, textWidth, opts.Version, opts.Warnings)
	popupHeight = minInt(popupHeight, len(content)+2)
	popupHeight = maxInt(popupHeight, 3)

	vp := viewport.New(viewport.WithWidth(textWidth), viewport.WithHeight(maxInt(popupHeight-2, 1)))
	vp.SetContentLines(content)
	vp.SetYOffset(opts.HelpScroll)

	caption := ""
	if !vp.AtTop() || !vp.AtBottom() {
		caption = intToString(int64(vp.ScrollPercent()*100+0.5)) + "%"
	}
	popup := strings.Join(panelWithCaption("DASHBURN · Help", caption, strings.Split(vp.View(), "\n"), popupWidth, popupHeight, st), "\n")

	x := maxInt((opts.Width-popupWidth)/2, 0)
	y := maxInt((opts.Height-popupHeight)/2, 0)
	compositor := lipgloss.NewCompositor(
		lipgloss.NewLayer(background),
		lipgloss.NewLayer(popup).X(x).Y(y).Z(1),
	)
	// The canvas trims trailing blanks; pad back so every row spans the
	// terminal exactly like the dashboard frame underneath.
	lines := strings.Split(lipgloss.NewCanvas(opts.Width, opts.Height).Compose(compositor).Render(), "\n")
	for i, line := range lines {
		lines[i] = padRight(line, opts.Width)
	}
	return strings.Join(lines, "\n")
}
