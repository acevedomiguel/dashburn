package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/acevedomiguel/burnblocks/openrouter"
)

// minTableHeight is the smallest daily table panel worth drawing: two borders,
// a column header, one day, a separator, and the pinned total row.
const minTableHeight = 6

// maxTableHeight caps how tall the daily table grows on a very tall terminal.
const maxTableHeight = 40

// openRouterSection renders the OpenRouter half of the dashboard.
func openRouterSection(opts ViewOptions, width, bodyHeight int) []string {
	st := opts.Styles
	out := []string{sectionHeader("OPENROUTER", openRouterStatus(opts), width, st)}

	if !opts.OpenRouter.Configured {
		return append(out, hintBlock(opts.OpenRouter.Hint, width, st)...)
	}
	if opts.OpenRouter.Error != "" {
		out = append(out, st.Warning.Render(truncate("! "+opts.OpenRouter.Error, width)))
	}
	snap := opts.OpenRouter.Snapshot
	if snap == nil {
		out = append(out, st.Muted.Render("Loading OpenRouter credits and usage…"))
		return out
	}

	n := 2
	if width < 76 {
		n = 1
	}
	blocks := []blockFunc{
		func(w int) []string { return creditsPanel(snap, opts, w) },
		func(w int) []string { return last30Panel(snap, opts, w) },
	}
	if hasTokenSplit(snap) {
		blocks = append(blocks, func(w int) []string { return tokensPanel(snap, opts, w) })
	}
	if len(snap.Models) > 0 {
		blocks = append(blocks, func(w int) []string { return modelsPanel(snap, opts, w) })
	}
	summary := layOut(width, n, blocks)
	out = append(out, summary...)

	// The daily table takes the remaining height so it stays readable without
	// pushing the summary off the top of the screen. len(out) already counts
	// the section header, any error line, and the summary rows; the extra row
	// is the blank line above the table.
	tableHeight := clampInt(bodyHeight-len(out)-1, minTableHeight, maxTableHeight)
	tableHeight = minInt(tableHeight, maxInt(len(snap.Daily)+5, minTableHeight))
	out = append(out, "")
	out = append(out, dailyTablePanel(snap.Daily, opts, width, tableHeight)...)
	return out
}

// creditsPanel renders the account credit panel. Freshness lives in the
// section header, so the panel itself carries no caption.
func creditsPanel(snap *openrouter.Snapshot, opts ViewOptions, width int) []string {
	t := opts.Styles
	inner := maxInt(width-4, 1)

	rw := snap.Runway
	runway := FormatDaysRemaining(rw)
	if d := FormatDepletionDate(rw); d != "" {
		runway += " " + d
	}
	runwayStyle := t.Value
	if rw.LowConfidence {
		runwayStyle = t.Warning
	}

	rows := []string{
		leader("Remaining", FormatCredits(snap.Account.Remaining), inner, t),
		leaderValue("Runway", runway, inner, t, runwayStyle),
	}
	if rw.Basis == openrouter.BasisUnavailable {
		rows = append(rows, leader("Daily burn", "analytics unavailable", inner, t))
	} else if rw.DailyBurn > 0 {
		rows = append(rows, leader("Daily burn", FormatCredits(rw.DailyBurn)+"/day", inner, t))
	}
	return panel("Credits", rows, width, len(rows)+2, t)
}

// last30Panel renders the 30-day summary.
func last30Panel(snap *openrouter.Snapshot, opts ViewOptions, width int) []string {
	t := opts.Styles
	inner := maxInt(width-4, 1)

	spend := 0.0
	for _, d := range snap.Daily {
		spend += d.CreditSpend
	}
	rows := []string{leader("Spend", FormatCredits(spend), inner, t)}
	if snap.Requests > 0 {
		rows = append(rows, leader("Requests", FormatInt(snap.Requests), inner, t))
	}
	rows = append(rows, leader("Tokens", FormatTokens(snap.TotalTokens), inner, t))
	if snap.CacheHitRate != nil {
		rows = append(rows, leader("Cache hit", fmt.Sprintf("%.1f%%", *snap.CacheHitRate*100), inner, t))
	}
	return panel("Last 30 days", rows, width, len(rows)+2, t)
}

// hasTokenSplit reports whether the API reported any part of the token split.
func hasTokenSplit(snap *openrouter.Snapshot) bool {
	return snap.TokensPrompt > 0 || snap.TokensCompletion > 0 || snap.CachedTokens > 0 || snap.ReasoningTokens > 0
}

// tokensPanel renders the prompt/completion/cached/reasoning split.
func tokensPanel(snap *openrouter.Snapshot, opts ViewOptions, width int) []string {
	t := opts.Styles
	inner := maxInt(width-4, 1)

	pairs := make([][2]string, 0, 4)
	if snap.TokensPrompt > 0 {
		pairs = append(pairs, [2]string{"Input", FormatTokens(snap.TokensPrompt)})
	}
	if snap.TokensCompletion > 0 {
		pairs = append(pairs, [2]string{"Output", FormatTokens(snap.TokensCompletion)})
	}
	if snap.CachedTokens > 0 {
		pairs = append(pairs, [2]string{"Cached", FormatTokens(snap.CachedTokens)})
	}
	if snap.ReasoningTokens > 0 {
		pairs = append(pairs, [2]string{"Reasoning", FormatTokens(snap.ReasoningTokens)})
	}

	colWidth := (inner - 2) / 2
	twoUp := colWidth >= 12
	for _, p := range pairs {
		if lipgloss.Width(p[0])+lipgloss.Width(p[1])+3 > colWidth {
			twoUp = false
			break
		}
	}

	rows := make([]string, 0, (len(pairs)+1)/2)
	if twoUp {
		for i := 0; i < len(pairs); i += 2 {
			if i+1 < len(pairs) {
				rows = append(rows, leader(pairs[i][0], pairs[i][1], colWidth, t)+"  "+leader(pairs[i+1][0], pairs[i+1][1], colWidth, t))
			} else {
				rows = append(rows, leader(pairs[i][0], pairs[i][1], inner, t))
			}
		}
	} else {
		for _, p := range pairs {
			rows = append(rows, leader(p[0], p[1], inner, t))
		}
	}
	return panel("Tokens", rows, width, len(rows)+2, t)
}

// modelsPanel renders the model spend breakdown.
func modelsPanel(snap *openrouter.Snapshot, opts ViewOptions, width int) []string {
	t := opts.Styles
	inner := maxInt(width-4, 1)

	sorted := SortModels(snap.Models)
	if len(sorted) == 0 {
		return panel("Cost by model", []string{t.Muted.Render("No model spend recorded.")}, width, 3, t)
	}
	const limit = 5
	models := sorted
	if len(sorted) > limit {
		models = TopModels(sorted, limit)
	}
	total := 0.0
	for _, m := range models {
		total += m.CreditSpend
	}
	rows := make([]string, 0, len(models))
	for _, m := range models {
		rows = append(rows, modelRow(m, total, inner, t))
	}
	return panel("Cost by model", rows, width, len(rows)+2, t)
}

// modelRow renders one model as "name ······· $2.61 ▇▇▇▇ 52%".
func modelRow(m openrouter.ModelUsage, total float64, width int, t Styles) string {
	name := ShortModelName(m.Model)
	spend := FormatCredits(m.CreditSpend)
	ratio := 0.0
	if total > 0 {
		ratio = m.CreditSpend / total
	}
	pct := fmt.Sprintf("%.0f%%", ratio*100)

	meterWidth := 10
	if width < 40 {
		meterWidth = 6
	}
	if width < 30 {
		meterWidth = 4
	}
	tail := spend + " " + meter(ratio, meterWidth, t) + " " + pct
	tailWidth := lipgloss.Width(tail)
	if tailWidth+12 > width {
		tail = spend + " " + pct
		tailWidth = lipgloss.Width(tail)
	}

	nameWidth := width - tailWidth - 3
	if nameWidth < 6 {
		nameWidth = 6
	}
	name = truncate(name, nameWidth)
	dots := width - lipgloss.Width(name) - tailWidth - 2
	if dots < 1 {
		return t.Label.Render(truncate(name+" "+spend, width))
	}
	return t.Label.Render(name) + " " + t.Muted.Render(strings.Repeat(boxDot, dots)) + " " + tail
}

// meter renders a small proportional bar.
func meter(ratio float64, width int, st Styles) string {
	if width < 1 {
		return ""
	}
	fill := int(clamp01(ratio)*float64(width) + 0.5)
	if fill == 0 && ratio > 0 {
		fill = 1
	}
	bar := strings.Repeat("▇", fill)
	track := strings.Repeat("░", width-fill)
	if !st.Enabled {
		return bar + track
	}
	return st.Accent.Render(bar) + lipgloss.NewStyle().Foreground(st.MeterEmpty).Render(track)
}

// --- daily table ---

// dailyColumn is one column of the daily table.
type dailyColumn struct {
	title string
	width int
	right bool
}

// dailyTablePanel renders the daily credit spend as a bordered table, newest
// day first, with a pinned total row. It returns exactly height lines.
func dailyTablePanel(daily []openrouter.DailyUsage, opts ViewOptions, width, height int) []string {
	st := opts.Styles
	height = maxInt(height, minTableHeight)
	inner := maxInt(width-4, 1)

	if len(daily) == 0 {
		return panel("Daily spend", []string{st.Muted.Render("No daily data available.")}, width, height, st)
	}

	cols := dailyColumns(inner)
	dataRows := maxInt(height-5, 1)

	newest := make([]openrouter.DailyUsage, 0, len(daily))
	for i := len(daily) - 1; i >= 0; i-- {
		newest = append(newest, daily[i])
	}
	maxOffset := maxInt(len(newest)-dataRows, 0)
	offset := clampInt(opts.OpenRouter.TableOffset, 0, maxOffset)
	end := minInt(offset+dataRows, len(newest))
	window := newest[offset:end]
	peak := peakSpend(daily)

	body := make([]string, 0, dataRows+3)
	body = append(body, st.CardHead.Render(dailyRow(columnTitles(cols), cols)))
	for _, d := range window {
		cells := dayCells(d)
		if len(cols) > 1 && peak > 0 && d.CreditSpend > 0 {
			cells[1] = heatStyle(st, d.CreditSpend/peak).Render(cells[1])
		}
		body = append(body, dailyRow(cells, cols))
	}
	for len(body) < dataRows+1 {
		body = append(body, "")
	}
	body = append(body, st.Muted.Render(strings.Repeat(boxHorizontal, minInt(colBudget(cols), inner))))
	body = append(body, totalRow(daily, cols, st))

	caption := ""
	if len(newest) > dataRows {
		caption = fmt.Sprintf("%d-%d/%d", offset+1, end, len(newest))
	}
	return panelWithCaption("Daily spend", caption, body, width, dataRows+5, st)
}

// dailyColumns picks the visible columns. Dropped columns are the ones the
// panel cannot fit, from the right.
func dailyColumns(inner int) []dailyColumn {
	cols := []dailyColumn{
		{title: "DATE", width: 11},
		{title: "SPEND", width: 8, right: true},
		{title: "TOKENS", width: 7, right: true},
		{title: "REQS", width: 6, right: true},
		{title: "CACHE", width: 6, right: true},
	}
	for len(cols) > 1 && colBudget(cols) > inner {
		cols = cols[:len(cols)-1]
	}
	return cols
}

// colBudget is the row width the columns occupy, the gaps included.
func colBudget(cols []dailyColumn) int {
	sum := len(cols) - 1
	for _, c := range cols {
		sum += c.width
	}
	return sum
}

func columnTitles(cols []dailyColumn) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.title
	}
	return out
}

// dailyRow renders one row of cells, padded to each column.
func dailyRow(cells []string, cols []dailyColumn) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		value := ""
		if i < len(cells) {
			value = cells[i]
		}
		parts[i] = padCell(value, c.width, c.right)
	}
	return strings.Join(parts, " ")
}

// dayCells renders one day's values in column order.
func dayCells(d openrouter.DailyUsage) []string {
	return []string{
		d.Day.Format("Jan 02"),
		FormatCredits(d.CreditSpend),
		FormatTokens(d.Tokens),
		FormatInt(d.Requests),
		formatCacheRate(d.CachedTokens, d.TokensPrompt),
	}
}

// formatCacheRate derives a day's cache hit rate. A day that reported no prompt
// tokens reads "-", never "0.0%".
func formatCacheRate(cached, prompt int64) string {
	if prompt <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", clamp01(float64(cached)/float64(prompt))*100)
}

// totalRow pins the all-days totals under the visible window.
func totalRow(daily []openrouter.DailyUsage, cols []dailyColumn, st Styles) string {
	var spend float64
	var tokens, reqs, cached, prompt int64
	for _, d := range daily {
		spend += d.CreditSpend
		tokens += d.Tokens
		reqs += d.Requests
		cached += d.CachedTokens
		prompt += d.TokensPrompt
	}
	cells := []string{
		fmt.Sprintf("%d days", len(daily)),
		FormatCredits(spend),
		FormatTokens(tokens),
		FormatInt(reqs),
		formatCacheRate(cached, prompt),
	}
	return st.Value.Render(dailyRow(cells, cols))
}

// padCell pads a cell to its column width, right-aligning on request.
func padCell(s string, width int, right bool) string {
	if w := lipgloss.Width(s); w > width {
		s = truncate(s, width)
	}
	pad := width - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	if right {
		return strings.Repeat(" ", pad) + s
	}
	return s + strings.Repeat(" ", pad)
}

// peakSpend is the largest single-day spend in the series.
func peakSpend(daily []openrouter.DailyUsage) float64 {
	peak := 0.0
	for _, d := range daily {
		if d.CreditSpend > peak {
			peak = d.CreditSpend
		}
	}
	return peak
}

// heatColor maps a day's share of the peak spend onto a green-yellow-red ramp.
func heatColor(t float64) color.Color {
	t = clamp01(t)
	type rgb struct{ r, g, b uint8 }
	green, yellow, red := rgb{63, 185, 80}, rgb{210, 153, 34}, rgb{248, 81, 73}
	a, b := green, red
	u := t
	if t < 0.5 {
		b = yellow
		u = t * 2
	} else {
		a = yellow
		u = (t - 0.5) * 2
	}
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*u + 0.5)
	}
	return color.RGBA{R: mix(a.r, b.r), G: mix(a.g, b.g), B: mix(a.b, b.b), A: 255}
}

// heatStyle colours a value by its share of the peak.
func heatStyle(st Styles, t float64) lipgloss.Style {
	if !st.Enabled {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(heatColor(t))
}
