// Package app holds the DashBurn Bubble Tea model: two independent provider
// refreshes, their scheduling, and the terminal lifecycle. Rendering lives in
// internal/ui, and provider access lives in internal/provider.
package app

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/acevedomiguel/dashburn/internal/provider"
	"github.com/acevedomiguel/dashburn/internal/ui"
	"github.com/acevedomiguel/kimiburn"
	"github.com/acevedomiguel/routeburn"
)

// Config is the resolved, non-secret application configuration.
type Config struct {
	// KimiInterval and OpenRouterInterval are the automatic polling intervals.
	// Zero disables automatic refresh for that provider.
	KimiInterval       time.Duration
	OpenRouterInterval time.Duration
	// ManualFloor is the shortest gap between two manual refreshes.
	ManualFloor time.Duration
	NoColor     bool
	Theme       string
	Version     string
}

// Deps are the model dependencies. Everything is injectable for tests.
type Deps struct {
	Config         Config
	Kimi           provider.KimiSource
	OpenRouter     provider.OpenRouterSource
	KimiHint       string
	OpenRouterHint string
	Warnings       []string
	Now            func() time.Time
	Redact         func(string) string
	Timeout        time.Duration
}

// sourceState is the per-provider refresh bookkeeping.
type sourceState struct {
	refreshing  bool
	err         string
	lastAttempt time.Time
	lastDone    time.Time
	nextAllowed time.Time
}

// nextDelay is the wait before the next automatic read: one interval after the
// previous read completed, or later when the provider asked for a pause.
func (s *sourceState) nextDelay(now time.Time, interval time.Duration) time.Duration {
	delay := interval
	if !s.lastDone.IsZero() {
		if d := s.lastDone.Add(interval).Sub(now); d > delay {
			delay = d
		}
	}
	if s.nextAllowed.After(now) {
		if d := s.nextAllowed.Sub(now); d > delay {
			delay = d
		}
	}
	if delay < 0 {
		return 0
	}
	return delay
}

// Model is the complete Bubble Tea state for one DashBurn process.
type Model struct {
	cfg     Config
	now     func() time.Time
	redact  func(string) string
	timeout time.Duration

	styles  ui.Styles
	spinner spinner.Model

	kimiSource provider.KimiSource
	orSource   provider.OpenRouterSource
	kimiHint   string
	orHint     string

	kimiSnap    *kimiburn.Snapshot
	kimiHistory map[kimiburn.QuotaKey][]kimiburn.Sample
	kimiState   sourceState

	orSnap  *routeburn.Snapshot
	orState sourceState

	warnings []string

	width  int
	height int
	scroll int

	helpVisible bool
	helpScroll  int

	tableOffset  int
	tableFocused bool

	notice  string
	started bool
}

// New builds the initial model.
func New(deps Deps) *Model {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Timeout <= 0 {
		deps.Timeout = 30 * time.Second
	}
	if deps.Config.ManualFloor <= 0 {
		deps.Config.ManualFloor = 30 * time.Second
	}
	if deps.Redact == nil {
		deps.Redact = func(s string) string { return s }
	}
	styles := ui.NewStylesFor(!deps.Config.NoColor, deps.Config.Theme != "light")
	return &Model{
		cfg:         deps.Config,
		now:         deps.Now,
		redact:      deps.Redact,
		timeout:     deps.Timeout,
		styles:      styles,
		spinner:     ui.NewSpinner(styles),
		kimiSource:  deps.Kimi,
		orSource:    deps.OpenRouter,
		kimiHint:    deps.KimiHint,
		orHint:      deps.OpenRouterHint,
		warnings:    append([]string(nil), deps.Warnings...),
		width:       80,
		height:      24,
		kimiHistory: nil,
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return func() tea.Msg { return initMsg{} }
}

// viewOptions is the single description of the current frame, shared by the
// renderer and the scroll-limit measurement.
func (m *Model) viewOptions() ui.ViewOptions {
	spinnerFrame := m.spinner.View()
	return ui.ViewOptions{
		Width:       m.width,
		Height:      m.height,
		Styles:      m.styles,
		Now:         m.now(),
		Scroll:      m.scroll,
		Version:     m.cfg.Version,
		HelpVisible: m.helpVisible,
		HelpScroll:  m.helpScroll,
		Warnings:    m.warningsList(),
		Notice:      m.notice,
		Kimi: ui.KimiView{
			Snapshot:   m.kimiSnap,
			History:    m.kimiHistory,
			Refreshing: m.kimiState.refreshing,
			Spinner:    spinnerFrame,
			Error:      m.kimiState.err,
			Configured: m.kimiSource != nil,
			Hint:       m.kimiHint,
		},
		OpenRouter: ui.OpenRouterView{
			Snapshot:     m.orSnap,
			Refreshing:   m.orState.refreshing,
			Spinner:      spinnerFrame,
			Error:        m.orState.err,
			Configured:   m.orSource != nil,
			Hint:         m.orHint,
			TableOffset:  m.tableOffset,
			TableFocused: m.tableFocused,
		},
	}
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	if m.width <= 0 || m.height <= 0 {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}
	background := ui.Render(m.viewOptions())
	if !m.helpVisible {
		return m.newView(background, m.windowTitle())
	}

	opts := m.viewOptions()
	opts.HelpVisible = true
	return m.newView(ui.HelpOverlay(ui.Render(opts), opts), "dashburn")
}

// newView builds the alternate-screen view with wheel scrolling enabled.
func (m *Model) newView(content, title string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = title
	return v
}

// windowTitle puts the headline figures in the terminal tab.
func (m *Model) windowTitle() string {
	var parts []string
	if m.kimiSnap != nil {
		if quota, ok := m.kimiSnap.Quotas[kimiburn.QuotaLimit5h]; ok {
			if remaining := remainingRatio(quota.UsedRatio); remaining != nil {
				parts = append(parts, "5h "+ui.FormatPercent(remaining))
			}
		}
	}
	if m.orSnap != nil {
		parts = append(parts, ui.FormatCredits(m.orSnap.Account.Remaining)+" OR")
	}
	if len(parts) == 0 {
		return "dashburn"
	}
	return "dashburn · " + strings.Join(parts, " · ")
}

func remainingRatio(used *float64) *float64 {
	if used == nil || *used != *used || *used < 0 || *used > 1 {
		return nil
	}
	remaining := 1 - *used
	return &remaining
}

// warningsList collects every current warning for the Help overlay.
func (m *Model) warningsList() []string {
	out := append([]string(nil), m.warnings...)
	if m.kimiState.err != "" {
		out = append(out, "Kimi Code · "+m.kimiState.err)
	}
	if m.orState.err != "" {
		out = append(out, "OpenRouter · "+m.orState.err)
	}
	if m.kimiSnap != nil {
		for _, w := range m.kimiSnap.Warnings {
			out = append(out, "Kimi Code · "+w.Section+": "+w.Message)
		}
	}
	if m.orSnap != nil {
		for _, w := range m.orSnap.Warnings {
			out = append(out, "OpenRouter · "+w.Section+": "+w.Message)
		}
	}
	return out
}

func (m *Model) maxScroll() int {
	return ui.MaxScroll(m.viewOptions())
}

func (m *Model) maxHelpScroll() int {
	return ui.HelpScrollLimit(m.viewOptions())
}

func (m *Model) anyRefreshing() bool { return m.kimiState.refreshing || m.orState.refreshing }
