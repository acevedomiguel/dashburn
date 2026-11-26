package ui

import (
	"testing"
	"time"

	"github.com/acevedomiguel/burnblocks/kimi"
	"github.com/acevedomiguel/burnblocks/openrouter"
)

func TestFormatPercentDistinguishesMissingFromZero(t *testing.T) {
	if got := FormatPercent(nil); got != "Not reported" {
		t.Errorf("nil ratio = %q, want Not reported", got)
	}
	zero := 0.0
	if got := FormatPercent(&zero); got != "0.0%" {
		t.Errorf("zero ratio = %q, want 0.0%%", got)
	}
	bad := 1.5
	if got := FormatPercent(&bad); got != "Unavailable" {
		t.Errorf("out-of-range ratio = %q, want Unavailable", got)
	}
}

func TestFormatCountNeverInventsZero(t *testing.T) {
	if got := FormatCount(nil); got != "Not reported" {
		t.Errorf("nil count = %q, want Not reported", got)
	}
	v := 1200.0
	if got := FormatCount(&v); got != "1200" {
		t.Errorf("count = %q, want 1200", got)
	}
}

func TestFormatRunwayStates(t *testing.T) {
	cases := []struct {
		est  kimi.RunwayEstimate
		want string
	}{
		{kimi.RunwayEstimate{Status: "depleted"}, "Depleted"},
		{kimi.RunwayEstimate{Status: "beyond_reset"}, "Beyond this reset"},
		{kimi.RunwayEstimate{Status: "no_consumption"}, "No consumption observed"},
		{kimi.RunwayEstimate{Status: "finite", Seconds: floatPtr(5400)}, "1h 30m"},
		{kimi.RunwayEstimate{Status: "unavailable", Reason: "insufficient_samples"}, "Unavailable · insufficient samples"},
	}
	for _, c := range cases {
		if got := FormatRunway(c.est, kimi.QuotaLimit5h); got != c.want {
			t.Errorf("FormatRunway(%v) = %q, want %q", c.est.Status, got, c.want)
		}
	}
}

func TestFormatCreditsAndTokens(t *testing.T) {
	if got := FormatCredits(42.1); got != "$42.10" {
		t.Errorf("FormatCredits(42.1) = %q", got)
	}
	if got := FormatCredits(-3.5); got != "-$3.50" {
		t.Errorf("FormatCredits(-3.5) = %q", got)
	}
	if got := FormatTokens(8420000); got != "8.4M" {
		t.Errorf("FormatTokens(8420000) = %q", got)
	}
	if got := FormatInt(1240); got != "1,240" {
		t.Errorf("FormatInt(1240) = %q", got)
	}
}

func TestFormatDaysRemainingBases(t *testing.T) {
	if got := FormatDaysRemaining(openrouter.RunwayEstimate{Basis: openrouter.BasisUnavailable}); got != "Unavailable" {
		t.Errorf("unavailable = %q", got)
	}
	days := 102.0
	if got := FormatDaysRemaining(openrouter.RunwayEstimate{Basis: openrouter.BasisFinite, DaysRemaining: &days}); got != "102 days" {
		t.Errorf("finite = %q", got)
	}
}

func TestShortModelName(t *testing.T) {
	if got := ShortModelName("deepseek/deepseek-v4.1-flash-20260910"); got != "deepseek-v4.1-flash" {
		t.Errorf("ShortModelName = %q", got)
	}
	if got := ShortModelName("openai/gpt-5"); got != "gpt-5" {
		t.Errorf("ShortModelName = %q", got)
	}
}

func TestTimelineRowMarksSafeWhenBeyondReset(t *testing.T) {
	now := fixedNow()
	reset := now.Add(2 * time.Hour)
	quota := kimi.QuotaReading{ResetAt: timePtr(reset)}
	row, ok := timelineRow(quota, kimi.RunwayEstimate{Status: "beyond_reset"}, 60, now, plainStyles())
	if !ok {
		t.Fatal("expected a timeline row")
	}
	if !contains(row, "✓") {
		t.Errorf("beyond-reset timeline should be marked safe: %q", row)
	}
}

func TestCurrentPeriodFiltersOtherResets(t *testing.T) {
	now := fixedNow()
	thisReset := now.Add(time.Hour)
	otherReset := now.Add(48 * time.Hour)
	samples := []kimi.Sample{
		{ObservedAt: now.Add(-time.Minute), UsedRatio: 0.1, ResetAt: thisReset},
		{ObservedAt: now, UsedRatio: 0.2, ResetAt: otherReset},
	}
	got := currentPeriod(samples, timePtr(thisReset))
	if len(got) != 1 || !got[0].ResetAt.Equal(thisReset) {
		t.Fatalf("currentPeriod kept the wrong samples: %+v", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
