package ui

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/acevedomiguel/kimiburn"
	"github.com/acevedomiguel/routeburn"
)

// --- Kimi Code formatting (kimiburn's helpers are not part of its public
// library surface, so DashBurn re-derives the same presentation). ---

// FormatPercent renders a valid ratio at one decimal place. Missing and invalid
// values never become a plausible-looking zero.
func FormatPercent(ratio *float64) string {
	if ratio == nil || math.IsNaN(*ratio) || math.IsInf(*ratio, 0) || *ratio < 0 || *ratio > 1 {
		if ratio == nil {
			return "Not reported"
		}
		return "Unavailable"
	}
	return fmt.Sprintf("%.1f%%", *ratio*100)
}

// FormatCount renders only an actually reported absolute count.
func FormatCount(count *float64) string {
	if count == nil || math.IsNaN(*count) || math.IsInf(*count, 0) || *count < 0 {
		return "Not reported"
	}
	return strconv.FormatFloat(*count, 'f', -1, 64)
}

// FormatReset describes the reset relative to the reading's timestamp.
func FormatReset(reset *time.Time, now time.Time) string {
	if reset == nil {
		return "Not reported"
	}
	if !reset.After(now) {
		return "Passed · awaiting API update"
	}
	return "in " + formatDuration(reset.Sub(now), false)
}

// FormatRunway renders the local Kimi estimate without implying exactness.
func FormatRunway(runway kimiburn.RunwayEstimate, key kimiburn.QuotaKey) string {
	switch runway.Status {
	case "depleted":
		return "Depleted"
	case "beyond_reset":
		return "Beyond this reset"
	case "no_consumption":
		return "No consumption observed"
	case "finite":
		if runway.Seconds == nil || math.IsNaN(*runway.Seconds) || math.IsInf(*runway.Seconds, 0) {
			return "Unavailable · non finite estimate"
		}
		return formatDuration(time.Duration(*runway.Seconds*float64(time.Second)), true)
	default:
		reason := strings.ReplaceAll(runway.Reason, "_", " ")
		if reason == "" {
			reason = "insufficient samples"
		}
		return "Unavailable · " + reason
	}
}

// FormatBurn converts the canonical ratio/second burn into the panel's period.
func FormatBurn(runway kimiburn.RunwayEstimate, key kimiburn.QuotaKey) string {
	if runway.BurnRatioPerSecond == nil || math.IsNaN(*runway.BurnRatioPerSecond) || math.IsInf(*runway.BurnRatioPerSecond, 0) {
		return "Not reported"
	}
	factor := 8640000.0
	unit := "%/day"
	if key == kimiburn.QuotaLimit5h {
		factor, unit = 360000, "%/h"
	}
	value := *runway.BurnRatioPerSecond * factor
	if value >= 10 {
		return fmt.Sprintf("%.1f%s", value, unit)
	}
	return fmt.Sprintf("%.2f%s", value, unit)
}

func formatDuration(d time.Duration, round bool) string {
	if d < 0 {
		d = 0
	}
	minutes := int(d / time.Minute)
	if round && d%time.Minute >= 30*time.Second {
		minutes++
	}
	if minutes < 1 {
		return "<1m"
	}
	hours := minutes / 60
	minutes %= 60
	if hours < 48 {
		if hours == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := hours / 24
	hours %= 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// --- OpenRouter formatting (routeburn's domain helpers are not part of its
// public library surface either). ---

// FormatCredits renders a USD amount with two decimals.
func FormatCredits(v float64) string {
	if v > 0 && v < 0.005 {
		return "<$0.01"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	whole := math.Floor(v)
	frac := int(math.Round((v - whole) * 100))
	if frac == 100 {
		whole++
		frac = 0
	}
	out := "$" + groupThousands(int64(whole)) + "." + twoDigits(frac)
	if neg {
		out = "-" + out
	}
	return out
}

// FormatTokens renders token counts with K/M/B suffixes for display.
func FormatTokens(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	var out string
	switch {
	case v >= 1_000_000_000:
		out = trimZero(float64(v)/1e9) + "B"
	case v >= 1_000_000:
		out = trimZero(float64(v)/1e6) + "M"
	case v >= 1_000:
		out = trimZero(float64(v)/1e3) + "K"
	default:
		out = groupThousands(v)
	}
	if neg {
		return "-" + out
	}
	return out
}

// FormatInt renders a plain count with thousands separators.
func FormatInt(v int64) string { return groupThousands(v) }

// ShortModelName strips the vendor prefix and a trailing release date.
func ShortModelName(model string) string {
	name := model
	if i := strings.Index(name, "/"); i >= 0 {
		if i < len(name)-1 {
			name = name[i+1:]
		} else {
			name = name[:i]
		}
	}
	if len(name) > 9 && name[len(name)-9] == '-' && isAllDigits(name[len(name)-8:]) {
		name = name[:len(name)-9]
	}
	if name == "" {
		return model
	}
	return name
}

// FormatDaysRemaining renders the headline OpenRouter runway value.
func FormatDaysRemaining(est routeburn.RunwayEstimate) string {
	switch est.Basis {
	case routeburn.BasisUnavailable:
		return "Unavailable"
	case routeburn.BasisNoRecentSpend:
		return "No recent spend"
	case routeburn.BasisBeyondHorizon:
		return "Beyond horizon"
	case routeburn.BasisDepleted:
		return "0 days"
	}
	if est.DaysRemaining == nil {
		return "Unavailable"
	}
	return groupThousands(int64(math.Floor(*est.DaysRemaining))) + " days"
}

// FormatDepletionDate renders the approximate depletion date.
func FormatDepletionDate(est routeburn.RunwayEstimate) string {
	if est.Basis != routeburn.BasisFinite || est.DepletionDate == nil {
		return ""
	}
	return "~" + est.DepletionDate.Format("Jan 02")
}

// SortModels returns rows ordered by spend descending, ties broken by name.
func SortModels(models []routeburn.ModelUsage) []routeburn.ModelUsage {
	out := make([]routeburn.ModelUsage, len(models))
	copy(out, models)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreditSpend != out[j].CreditSpend {
			return out[i].CreditSpend > out[j].CreditSpend
		}
		return out[i].Model < out[j].Model
	})
	return out
}

// TopModels limits rows to the top n by spend, folding the remainder into an
// "Other (remaining)" row.
func TopModels(models []routeburn.ModelUsage, n int) []routeburn.ModelUsage {
	if n <= 0 {
		return nil
	}
	sorted := SortModels(models)
	if len(sorted) <= n {
		return sorted
	}
	top := make([]routeburn.ModelUsage, 0, n)
	top = append(top, sorted[:n-1]...)
	rest := routeburn.ModelUsage{Model: "Other (remaining)"}
	for _, m := range sorted[n-1:] {
		rest.CreditSpend += m.CreditSpend
		rest.Tokens += m.Tokens
	}
	return append(top, rest)
}

// FormatFreshness renders "updated 15:04:05" for a live reading.
func FormatFreshness(generatedAt, now time.Time) string {
	if generatedAt.IsZero() {
		return ""
	}
	return "updated " + generatedAt.In(now.Location()).Format("15:04:05")
}

// FormatCountdown renders a short, second-resolution wait.
func FormatCountdown(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 60 {
		return strconv.Itoa(seconds) + "s"
	}
	return strconv.Itoa(seconds/60) + "m " + strconv.Itoa(seconds%60) + "s"
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func groupThousands(v int64) string {
	if v < 0 {
		return "-" + groupThousands(-v)
	}
	digits := intToString(v)
	var b strings.Builder
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(digits[i])
	}
	return b.String()
}

func intToString(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func twoDigits(v int) string {
	if v < 10 {
		return "0" + intToString(int64(v))
	}
	return intToString(int64(v))
}

func trimZero(v float64) string {
	s := formatOneDecimal(v)
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}

func formatOneDecimal(v float64) string {
	whole := int64(v)
	frac := int(math.Round((v - float64(whole)) * 10))
	if frac == 10 {
		whole++
		frac = 0
	}
	return groupThousands(whole) + "." + intToString(int64(frac))
}
