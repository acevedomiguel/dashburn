package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/acevedomiguel/kimiburn"
)

// kimiPanelSpec names one Kimi quota panel.
type kimiPanelSpec struct {
	title      string
	key        kimiburn.QuotaKey
	allocation bool
}

// kimiSection renders the Kimi Code half of the dashboard.
func kimiSection(opts ViewOptions, width, bodyHeight int) []string {
	st := opts.Styles
	out := []string{sectionHeader("KIMI CODE", kimiStatus(opts), width, st)}

	if !opts.Kimi.Configured {
		return append(out, hintBlock(opts.Kimi.Hint, width, st)...)
	}
	if opts.Kimi.Error != "" {
		out = append(out, st.Warning.Render(truncate("! "+opts.Kimi.Error, width)))
	}
	snap := opts.Kimi.Snapshot
	if snap == nil {
		out = append(out, st.Muted.Render("Loading Kimi Code usage…"))
		return out
	}
	return append(out, kimiPanels(opts, snap, width)...)
}

// kimiPanels renders the reported quota panels, two per row when the width
// allows.
func kimiPanels(opts ViewOptions, snap *kimiburn.Snapshot, width int) []string {
	specs := []kimiPanelSpec{
		{title: "5-hour quota", key: kimiburn.QuotaLimit5h},
		{title: "Monthly membership", key: kimiburn.QuotaLimitMonthTotal},
	}
	if _, ok := snap.Quotas[kimiburn.QuotaLimitMonthCode]; ok {
		specs = append(specs, kimiPanelSpec{title: "Monthly Code allocation", key: kimiburn.QuotaLimitMonthCode, allocation: true})
	}
	if _, ok := snap.Quotas[kimiburn.QuotaLimit7d]; ok {
		specs = append(specs, kimiPanelSpec{title: "7-day quota · legacy", key: kimiburn.QuotaLimit7d})
	}

	n := 2
	if width < 76 {
		n = 1
	}
	blocks := make([]blockFunc, len(specs))
	for i, spec := range specs {
		spec := spec
		blocks[i] = func(w int) []string { return kimiPanel(spec, opts, w) }
	}
	return layOut(width, n, blocks)
}

// kimiPanel renders one quota panel at an explicit width.
func kimiPanel(spec kimiPanelSpec, opts ViewOptions, width int) []string {
	st := opts.Styles
	inner := width - 4
	if inner < 1 {
		inner = 1
	}
	snap := opts.Kimi.Snapshot
	quota, ok := snap.Quotas[spec.key]
	if !ok {
		return panel(spec.title, []string{st.Muted.Render("Not reported by this account/API response")}, width, 3, st)
	}
	runway := snap.Runways[spec.key]

	rows := make([]string, 0, 8)
	if validRatio(quota.UsedRatio) {
		rows = append(rows, remainingMeter(1-*quota.UsedRatio, inner, st))
	} else {
		rows = append(rows, st.Muted.Render("Meter unavailable · ratio not reported"))
	}
	if row, ok := timelineRow(quota, runway, inner, opts.Now, st); ok {
		rows = append(rows, row)
	}
	rows = append(rows,
		leader("Reset", FormatReset(quota.ResetAt, opts.Now), inner, st),
		leaderValue("Runway", FormatRunway(runway, spec.key), inner, st, runwayStyle(runway, st)),
		leader("Burn", FormatBurn(runway, spec.key), inner, st),
	)
	if row, ok := trendRow(opts.Kimi.History[spec.key], quota.ResetAt, inner, opts.Now, st); ok {
		rows = append(rows, row)
	}
	if quota.Used != nil && quota.Limit != nil {
		counts := FormatCount(quota.Used) + "/" + FormatCount(quota.Limit) + " used"
		if quota.Remaining != nil {
			counts += " · " + FormatCount(quota.Remaining) + " left"
		}
		rows = append(rows, leader("Requests", counts, inner, st))
	}
	if spec.allocation {
		rows = append(rows, st.Muted.Render("Included in membership"))
	}
	return panel(spec.title, rows, width, len(rows)+2, st)
}

// validRatio accepts only a reported fraction in [0,1]. The self-comparison
// rejects NaN, and the range check rejects an infinity.
func validRatio(ratio *float64) bool {
	if ratio == nil {
		return false
	}
	v := *ratio
	return v == v && v >= 0 && v <= 1
}

func runwayStyle(runway kimiburn.RunwayEstimate, st Styles) lipgloss.Style {
	if runway.Status == "finite" || runway.Status == "depleted" {
		return st.Error.Bold(st.Enabled)
	}
	return st.Value
}

// kimiStatus is the compact right-hand marker for the Kimi section: only the
// provider's own exceptional state. The frame's timestamp is shown once, in the
// title row, so no freshness or countdown text belongs here.
func kimiStatus(opts ViewOptions) string {
	k := opts.Kimi
	switch {
	case !k.Configured:
		return "not configured"
	case k.Refreshing:
		return strings.TrimSpace(k.Spinner + " refreshing")
	default:
		return ""
	}
}

// openRouterStatus is the compact right-hand marker for the OpenRouter section.
func openRouterStatus(opts ViewOptions) string {
	o := opts.OpenRouter
	switch {
	case !o.Configured:
		return "not configured"
	case o.Refreshing:
		return strings.TrimSpace(o.Spinner + " refreshing")
	default:
		return ""
	}
}

// sectionHeader renders a provider heading with its state marker tucked right
// after the title. The marker is deliberately not flush right: the only thing
// that belongs on the right-hand edge is the frame's single "updated" stamp, so
// a section marker must sit with the section it belongs to.
func sectionHeader(title, status string, width int, st Styles) string {
	head := st.Title.Render(title)
	if status == "" {
		return head
	}
	available := width - lipgloss.Width(head) - 2
	if available < 1 {
		return head
	}
	return head + "  " + st.Muted.Render(truncate(status, available))
}

// hintBlock renders a provider's "not configured" guidance, wrapped to width.
func hintBlock(hint string, width int, st Styles) []string {
	if hint == "" {
		hint = "Not configured."
	}
	lines := wrapPlain(hint, maxInt(width, 1))
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, st.Muted.Render(l))
	}
	return out
}
