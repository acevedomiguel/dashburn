package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Box-drawing characters. Every section is a box whose title is inlaid in the
// top border, so the frame does the grouping work.
const (
	boxTopLeft     = "╭"
	boxTopRight    = "╮"
	boxBottomLeft  = "╰"
	boxBottomRight = "╯"
	boxHorizontal  = "─"
	boxVertical    = "│"
	boxDot         = "·"
)

// panel draws one bordered box exactly width columns wide and height rows tall,
// with title inlaid in the top border. Callers size rows to width-4.
func panel(title string, rows []string, width, height int, st Styles) []string {
	return panelWithCaption(title, "", rows, width, height, st)
}

// panelWithCaption draws a panel whose title sits in the top border and whose
// caption sits flush right in the same border.
func panelWithCaption(title, caption string, rows []string, width, height int, st Styles) []string {
	if width < 4 {
		width = 4
	}
	if height < 2 {
		height = 2
	}
	inner := width - 2
	textWidth := inner - 2
	if textWidth < 1 {
		textWidth = 1
	}

	title = truncate(title, maxInt(inner-5, 1))
	head := st.Accent.Render(boxHorizontal+" ") + st.CardHead.Render(title) + st.Accent.Render(" "+boxHorizontal)
	tail := ""
	if caption != "" {
		if room := inner - lipgloss.Width(head) - 5; room > 0 {
			tail = " " + st.Muted.Render(truncate(caption, room)) + " " + boxHorizontal
		}
	}
	fill := inner - lipgloss.Width(head) - lipgloss.Width(tail)
	if fill < 0 {
		fill = 0
	}

	out := make([]string, 0, height)
	out = append(out, st.Accent.Render(boxTopLeft)+head+st.Accent.Render(strings.Repeat(boxHorizontal, fill)+tail+boxTopRight))

	for i := 0; i < height-2; i++ {
		row := ""
		if i < len(rows) {
			row = rows[i]
		}
		if lipgloss.Width(row) > textWidth {
			if !strings.ContainsRune(row, '\x1b') {
				row = truncate(row, textWidth)
			}
		}
		row = padRight(row, textWidth)
		out = append(out, st.Accent.Render(boxVertical)+" "+row+" "+st.Accent.Render(boxVertical))
	}

	out = append(out, st.Accent.Render(boxBottomLeft+strings.Repeat(boxHorizontal, inner)+boxBottomRight))
	return out
}

// leader renders "label ······ value" filling exactly width columns with the
// value flush right. Both inputs are plain text so the geometry is exact.
func leader(label, value string, width int, st Styles) string {
	return leaderValue(label, value, width, st, st.Value)
}

// leaderValue is leader with an explicit style for the value.
func leaderValue(label, value string, width int, st Styles, valueStyle lipgloss.Style) string {
	label = truncate(label, maxInt(width, 1))
	value = truncate(value, maxInt(width, 1))
	lw, vw := lipgloss.Width(label), lipgloss.Width(value)
	dots := width - lw - vw - 2
	if dots < 1 {
		return st.Muted.Render(truncate(label+" "+value, width))
	}
	return st.Label.Render(label) + " " + st.Muted.Render(strings.Repeat(boxDot, dots)) + " " + valueStyle.Render(value)
}

// remainingMeter renders a labelled row whose proportional bar and value both
// describe the remaining quota.
func remainingMeter(remaining float64, width int, st Styles) string {
	remaining = clamp01(remaining)
	value := FormatPercent(&remaining)
	const label = "Remaining"
	barWidth := width - lipgloss.Width(label) - lipgloss.Width(value) - 2
	if barWidth < 1 {
		return leader(label, value, width, st)
	}
	return st.Label.Render(label) + " " + meterBar(remaining, barWidth, st) + " " +
		st.gaugeStyle(remaining).Bold(st.Enabled).Render(value)
}

// meterBar renders the remaining-quota bar in the gauge colour for remaining.
func meterBar(remaining float64, width int, st Styles) string {
	if width < 1 {
		return ""
	}
	fill := int(clamp01(remaining)*float64(width) + 0.5)
	bar := strings.Repeat("▇", fill)
	track := strings.Repeat("░", width-fill)
	if !st.Enabled {
		return bar + track
	}
	return st.gaugeStyle(remaining).Render(bar) + lipgloss.NewStyle().Foreground(st.MeterEmpty).Render(track)
}

// joinColumns places rendered blocks side by side, one space apart, padding
// shorter blocks with blank lines so every row lines up.
func joinColumns(blocks ...[]string) []string {
	height := 0
	widths := make([]int, len(blocks))
	for i, b := range blocks {
		if len(b) > height {
			height = len(b)
		}
		if len(b) > 0 {
			widths[i] = lipgloss.Width(b[0])
		}
	}

	out := make([]string, 0, height)
	for row := 0; row < height; row++ {
		var b strings.Builder
		for i, blk := range blocks {
			if i > 0 {
				b.WriteString(" ")
			}
			line := ""
			if row < len(blk) {
				line = blk[row]
			}
			b.WriteString(padRight(line, widths[i]))
		}
		out = append(out, strings.TrimRight(b.String(), " "))
	}
	return out
}

// splitWidths divides total into n equal columns separated by a one-column
// gutter, giving any remainder to the last column.
func splitWidths(total, n int) []int {
	if n <= 1 {
		return []int{total}
	}
	avail := total - (n - 1)
	if avail < n {
		avail = n
	}
	base := avail / n
	widths := make([]int, n)
	for i := range widths {
		widths[i] = base
	}
	widths[n-1] += avail - base*n
	return widths
}

// blockFunc renders one block at an explicit width. It is how a panel is built
// for a column whose width is only known after the grid is divided.
type blockFunc func(width int) []string

// layOut arranges blocks n across, wrapping to a new row with a blank line
// between rows. A final row with a single block spans the full width.
func layOut(width, n int, blocks []blockFunc) []string {
	if n < 1 {
		n = 1
	}
	var out []string
	for i := 0; i < len(blocks); i += n {
		end := minInt(i+n, len(blocks))
		row := blocks[i:end]
		widths := splitWidths(width, len(row))
		rendered := make([][]string, len(row))
		for j := range row {
			rendered[j] = row[j](widths[j])
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, joinColumns(rendered...)...)
	}
	return out
}

// stackBlocks concatenates blocks vertically with a blank row between them.
func stackBlocks(blocks [][]string) []string {
	var out []string
	for i, b := range blocks {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, b...)
	}
	return out
}
