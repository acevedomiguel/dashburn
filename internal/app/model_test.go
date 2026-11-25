package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/acevedomiguel/dashburn/internal/provider"
	"github.com/acevedomiguel/dashburn/internal/ui"
	"github.com/acevedomiguel/kimiburn"
	"github.com/acevedomiguel/routeburn"
)

// clock is a settable clock for deterministic scheduling tests.
type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

type fakeKimi struct {
	result provider.KimiResult
	err    error
	calls  int
	next   time.Time
}

func (f *fakeKimi) Fetch(context.Context, time.Time) (provider.KimiResult, error) {
	f.calls++
	return f.result, f.err
}

func (f *fakeKimi) NextRefreshAllowed() time.Time { return f.next }

type fakeOR struct {
	result routeburn.Snapshot
	err    error
	calls  int
	next   time.Time
}

func (f *fakeOR) Fetch(context.Context, time.Time) (routeburn.Snapshot, error) {
	f.calls++
	return f.result, f.err
}

func (f *fakeOR) NextRefreshAllowed() time.Time { return f.next }

func startTime() time.Time { return time.Date(2026, 11, 25, 15, 42, 0, 0, time.UTC) }

func kimiFixture(now time.Time) provider.KimiResult {
	used := 0.25
	reset := now.Add(3 * time.Hour)
	return provider.KimiResult{
		Snapshot: kimiburn.Snapshot{
			GeneratedAt: now,
			Quotas: map[kimiburn.QuotaKey]kimiburn.QuotaReading{
				kimiburn.QuotaLimit5h: {UsedRatio: &used, ResetAt: &reset},
			},
			Runways: map[kimiburn.QuotaKey]kimiburn.RunwayEstimate{},
		},
	}
}

func orFixture(now time.Time) routeburn.Snapshot {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	daily := make([]routeburn.DailyUsage, 0, 30)
	for i := 0; i < 30; i++ {
		daily = append(daily, routeburn.DailyUsage{Day: today.AddDate(0, 0, i-29), CreditSpend: 0.4})
	}
	days := 100.0
	return routeburn.Snapshot{
		GeneratedAt: now,
		Account:     routeburn.AccountCredits{Remaining: 42.10},
		Daily:       daily,
		TotalTokens: 1000,
		Runway:      routeburn.RunwayEstimate{Basis: routeburn.BasisFinite, DaysRemaining: &days},
	}
}

func newModel(t *testing.T, c *clock, k *fakeKimi, o *fakeOR) *Model {
	t.Helper()
	return New(Deps{
		Config: Config{
			KimiInterval:       time.Minute,
			OpenRouterInterval: time.Minute,
			ManualFloor:        30 * time.Second,
			Version:            "test",
		},
		Kimi:       k,
		OpenRouter: o,
		Now:        c.now,
	})
}

func TestInitStartsBothProviders(t *testing.T) {
	c := &clock{startTime()}
	m := newModel(t, c, &fakeKimi{}, &fakeOR{})
	m.Update(initMsg{})
	if !m.kimiState.refreshing || !m.orState.refreshing {
		t.Fatalf("both providers should be refreshing: kimi=%t or=%t", m.kimiState.refreshing, m.orState.refreshing)
	}
}

func TestFetchedResultsAreStored(t *testing.T) {
	c := &clock{startTime()}
	m := newModel(t, c, &fakeKimi{}, &fakeOR{})
	m.Update(initMsg{})
	m.applyKimi(kimiFetchedMsg{result: kimiFixture(c.now())})
	m.applyOR(orFetchedMsg{snapshot: orFixture(c.now())})

	if m.kimiSnap == nil {
		t.Fatal("Kimi snapshot was not stored")
	}
	if m.orSnap == nil {
		t.Fatal("OpenRouter snapshot was not stored")
	}
	if m.kimiState.refreshing || m.orState.refreshing {
		t.Fatal("providers should no longer be refreshing")
	}
}

func TestProviderFailureKeepsTheOtherProvider(t *testing.T) {
	c := &clock{startTime()}
	m := newModel(t, c, &fakeKimi{}, &fakeOR{})
	m.Update(initMsg{})
	m.applyOR(orFetchedMsg{snapshot: orFixture(c.now())})
	m.applyKimi(kimiFetchedMsg{err: errors.New("Kimi Code is unreachable")})

	if m.kimiSnap != nil {
		t.Fatal("a failed Kimi read must not invent a snapshot")
	}
	if m.orSnap == nil {
		t.Fatal("an OpenRouter snapshot must survive a Kimi failure")
	}
	frame := ui.Render(m.viewOptions())
	if !strings.Contains(frame, "Kimi Code is unreachable") {
		t.Errorf("the Kimi error is missing from the frame:\n%s", frame)
	}
	if !strings.Contains(frame, "Credits") || !strings.Contains(frame, "Daily spend") {
		t.Errorf("the OpenRouter section was lost:\n%s", frame)
	}
}

func TestFailureKeepsLastGoodSnapshot(t *testing.T) {
	c := &clock{startTime()}
	m := newModel(t, c, &fakeKimi{}, &fakeOR{})
	m.Update(initMsg{})
	m.applyKimi(kimiFetchedMsg{result: kimiFixture(c.now())})

	c.advance(2 * time.Minute)
	m.Update(kimiTickMsg{})
	m.applyKimi(kimiFetchedMsg{err: errors.New("temporary failure")})

	if m.kimiSnap == nil {
		t.Fatal("the last good Kimi snapshot should remain visible")
	}
	if m.kimiState.err == "" {
		t.Fatal("the failure should be recorded as an error")
	}
}

func TestManualRefreshIsThrottled(t *testing.T) {
	c := &clock{startTime()}
	m := newModel(t, c, &fakeKimi{}, &fakeOR{})
	m.Update(initMsg{})
	m.applyKimi(kimiFetchedMsg{result: kimiFixture(c.now())})

	cmd := m.startKimi(true)
	if cmd == nil {
		t.Fatal("expected a command")
	}
	msg := cmd()
	notice, ok := msg.(noticeMsg)
	if !ok {
		t.Fatalf("expected a throttle notice, got %T", msg)
	}
	if !strings.Contains(notice.text, "Kimi refresh in") {
		t.Errorf("notice = %q", notice.text)
	}
}

func TestManualRefreshAllowedAfterFloor(t *testing.T) {
	c := &clock{startTime()}
	kimi := &fakeKimi{result: kimiFixture(c.now())}
	m := newModel(t, c, kimi, &fakeOR{})
	m.Update(initMsg{})
	m.applyKimi(kimiFetchedMsg{result: kimiFixture(c.now())})

	c.advance(31 * time.Second)
	cmd := m.startKimi(true)
	if cmd == nil {
		t.Fatal("expected a command")
	}
	if _, ok := cmd().(kimiFetchedMsg); !ok {
		t.Fatalf("expected a fetch, got %T", cmd())
	}
}

func TestServerThrottleDelaysNextRead(t *testing.T) {
	c := &clock{startTime()}
	kimi := &fakeKimi{result: kimiFixture(c.now())}
	m := newModel(t, c, kimi, &fakeOR{})
	m.Update(initMsg{})

	serverNext := c.t.Add(5 * time.Minute)
	m.applyKimi(kimiFetchedMsg{result: kimiFixture(c.now()), nextAllowed: serverNext})

	if delay := m.kimiState.nextDelay(c.now(), time.Minute); delay < 5*time.Minute {
		t.Fatalf("next delay = %s, want at least the server window", delay)
	}
}

func TestQuitKey(t *testing.T) {
	c := &clock{startTime()}
	m := newModel(t, c, &fakeKimi{}, &fakeOR{})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestHelpToggles(t *testing.T) {
	c := &clock{startTime()}
	m := newModel(t, c, &fakeKimi{}, &fakeOR{})
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !m.helpVisible {
		t.Fatal("? should open help")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.helpVisible {
		t.Fatal("esc should close help")
	}
}

func TestTabFocusesDailyTable(t *testing.T) {
	c := &clock{startTime()}
	m := newModel(t, c, &fakeKimi{}, &fakeOR{})
	m.applyOR(orFetchedMsg{snapshot: orFixture(c.now())})

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.tableFocused {
		t.Fatal("tab should focus the daily table")
	}
	before := m.tableOffset
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.tableOffset != before+1 {
		t.Fatalf("table offset = %d, want %d", m.tableOffset, before+1)
	}
}

func TestBodyScrollClampsAtTop(t *testing.T) {
	c := &clock{startTime()}
	m := newModel(t, c, &fakeKimi{}, &fakeOR{})
	m.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.scroll != 0 {
		t.Fatalf("scroll = %d, want 0 (clamped)", m.scroll)
	}
}

func TestAppHandleIsTeaModel(t *testing.T) {
	var _ tea.Model = New(Deps{})
}
