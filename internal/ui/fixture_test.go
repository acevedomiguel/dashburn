package ui

import (
	"time"

	"github.com/acevedomiguel/burnblocks/kimi"
	"github.com/acevedomiguel/burnblocks/openrouter"
)

// fixedNow is the frozen clock every render test uses.
func fixedNow() time.Time {
	return time.Date(2026, 11, 25, 15, 42, 0, 0, time.UTC)
}

func plainStyles() Styles { return NewStyles(false) }

func floatPtr(v float64) *float64 { return &v }
func timePtr(t time.Time) *time.Time {
	return &t
}

// syntheticKimi is a sanitized Kimi fixture. It never contains real account
// data.
func syntheticKimi(now time.Time) *kimi.Snapshot {
	used := 0.25
	reset5 := now.Add(3 * time.Hour)
	resetMonth := now.AddDate(0, 0, 12)
	limit := 1000.0
	return &kimi.Snapshot{
		SchemaVersion: kimi.SnapshotSchemaVersion,
		GeneratedAt:   now,
		Quotas: map[kimi.QuotaKey]kimi.QuotaReading{
			kimi.QuotaLimit5h:         {Source: kimi.SourceRatio, UsedRatio: &used, ResetAt: timePtr(reset5)},
			kimi.QuotaLimitMonthTotal: {Source: kimi.SourceRatio, UsedRatio: &used, ResetAt: timePtr(resetMonth), Used: floatPtr(250), Limit: &limit, Remaining: floatPtr(750)},
			kimi.QuotaLimitMonthCode:  {Source: kimi.SourceRatio, UsedRatio: &used, ResetAt: timePtr(resetMonth)},
			kimi.QuotaLimit7d:         {Source: kimi.SourceRatio, UsedRatio: &used, ResetAt: timePtr(resetMonth)},
		},
		Runways: map[kimi.QuotaKey]kimi.RunwayEstimate{
			kimi.QuotaLimit5h: {
				Status:             "finite",
				Seconds:            floatPtr(3600),
				BurnRatioPerSecond: floatPtr(0.00002),
			},
			kimi.QuotaLimitMonthTotal: {Status: "beyond_reset"},
		},
	}
}

// syntheticHistory is a short two-sample trend for the 5-hour quota.
func syntheticHistory(now time.Time) map[kimi.QuotaKey][]kimi.Sample {
	reset := now.Add(3 * time.Hour)
	return map[kimi.QuotaKey][]kimi.Sample{
		kimi.QuotaLimit5h: {
			{ObservedAt: now.Add(-10 * time.Minute), UsedRatio: 0.20, ResetAt: reset},
			{ObservedAt: now, UsedRatio: 0.25, ResetAt: reset},
		},
	}
}

// syntheticOpenRouter is a sanitized RouteBurn fixture.
func syntheticOpenRouter(now time.Time) *openrouter.Snapshot {
	spend := []float64{0, 0.12, 0.31, 0.44, 0.2, 0.05, 0, 0.61, 0.88, 0.42, 0.31, 0.27, 0.9, 1.2, 0.77, 0.31, 0.0, 0.18, 0.24, 0.55, 0.4, 0.35, 0.2, 0.6, 0.45, 0.3, 0.15, 0.25, 0.5, 0.33}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	daily := make([]openrouter.DailyUsage, 0, 30)
	for i := 0; i < 30; i++ {
		v := 0.0
		if i < len(spend) {
			v = spend[i]
		}
		daily = append(daily, openrouter.DailyUsage{
			Day:          today.AddDate(0, 0, i-29),
			CreditSpend:  v,
			Tokens:       int64(v * 700000),
			Requests:     int64(v * 70),
			TokensPrompt: int64(v * 500000),
			CachedTokens: int64(v * 90000),
		})
	}
	days := 102.0
	depletion := time.Date(2027, 3, 3, 0, 0, 0, 0, time.UTC)
	cacheHit := 0.184
	return &openrouter.Snapshot{
		SchemaVersion: openrouter.SchemaVersion,
		GeneratedAt:   now,
		Range:         openrouter.TimeRange{Start: today.AddDate(0, 0, -29), End: today.AddDate(0, 0, 1)},
		Account:       openrouter.AccountCredits{TotalPurchased: 100, TotalUsed: 57.90, Remaining: 42.10},
		Daily:         daily,
		Models: []openrouter.ModelUsage{
			{Model: "anthropic/claude-sonnet-4", CreditSpend: 7.21, Tokens: 4100000},
			{Model: "openai/gpt-5", CreditSpend: 3.82, Tokens: 2900000},
			{Model: "meta/llama-4-70b", CreditSpend: 1.31, Tokens: 1420000},
		},
		TotalTokens:      8420000,
		CacheHitRate:     &cacheHit,
		Requests:         1240,
		TokensPrompt:     7100000,
		TokensCompletion: 1320000,
		CachedTokens:     1512000,
		ReasoningTokens:  480000,
		Runway: openrouter.RunwayEstimate{
			WindowDays:    30,
			WindowSpend:   12.34,
			DailyBurn:     0.4113,
			DaysRemaining: &days,
			DepletionDate: &depletion,
			Basis:         openrouter.BasisFinite,
		},
	}
}

// fullOptions is a fully populated frame at the requested size.
func fullOptions(width, height int, styles Styles) ViewOptions {
	now := fixedNow()
	return ViewOptions{
		Width:  width,
		Height: height,
		Styles: styles,
		Now:    now,
		Kimi: KimiView{
			Snapshot:   syntheticKimi(now),
			History:    syntheticHistory(now),
			Configured: true,
		},
		OpenRouter: OpenRouterView{
			Snapshot:   syntheticOpenRouter(now),
			Configured: true,
		},
	}
}
