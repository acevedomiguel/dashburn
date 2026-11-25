package app

import (
	"context"
	"errors"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/acevedomiguel/dashburn/internal/provider"
	"github.com/acevedomiguel/dashburn/internal/ui"
	"github.com/acevedomiguel/routeburn"
)

// Messages. Each provider has its own tick and result so a slow or failing
// provider can never delay or blank the other.
type (
	initMsg  struct{}
	clockMsg struct{}

	kimiTickMsg struct{}
	orTickMsg   struct{}

	kimiFetchedMsg struct {
		result      provider.KimiResult
		nextAllowed time.Time
		err         error
	}
	orFetchedMsg struct {
		snapshot    routeburn.Snapshot
		nextAllowed time.Time
		err         error
	}

	noticeMsg struct{ text string }
)

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case initMsg:
		return m, batch(m.startup(), m.startKimi(false), m.startOR(false), m.spinner.Tick)

	case tea.BackgroundColorMsg:
		if m.cfg.Theme == "dark" || m.cfg.Theme == "light" {
			return m, nil
		}
		m.styles = ui.NewStylesFor(!m.cfg.NoColor, msg.IsDark())
		m.spinner.Style = ui.NewSpinner(m.styles).Style
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scroll = clampInt(m.scroll, 0, m.maxScroll())
		return m, nil

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.scrollBy(-3)
		case tea.MouseWheelDown:
			m.scrollBy(3)
		}
		return m, nil

	case clockMsg:
		return m, clockTick()

	case kimiTickMsg:
		return m, batch(m.startKimi(false), m.spinner.Tick)

	case orTickMsg:
		return m, batch(m.startOR(false), m.spinner.Tick)

	case kimiFetchedMsg:
		m.applyKimi(msg)
		return m, batch(m.armKimiTick(), m.spinner.Tick)

	case orFetchedMsg:
		m.applyOR(msg)
		return m, batch(m.armORTick(), m.spinner.Tick)

	case noticeMsg:
		m.notice = msg.text
		return m, nil

	case spinner.TickMsg:
		if !m.anyRefreshing() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// startup runs once: ask the terminal for its background colour and start the
// one-second clock that keeps countdowns live.
func (m *Model) startup() tea.Cmd {
	if m.started {
		return nil
	}
	m.started = true
	return batch(tea.RequestBackgroundColor, clockTick())
}

// clockTick re-renders once a second. It performs no I/O.
func clockTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return clockMsg{} })
}

// refreshBoth requests a manual refresh of both providers.
func (m *Model) refreshBoth() tea.Cmd {
	m.notice = ""
	return batch(m.startKimi(true), m.startOR(true), m.spinner.Tick)
}

// startKimi begins one Kimi read unless one is already in flight or the manual
// throttle refuses it.
func (m *Model) startKimi(manual bool) tea.Cmd {
	if m.kimiSource == nil {
		return nil
	}
	s := &m.kimiState
	now := m.now()
	if s.refreshing {
		return nil
	}
	if manual {
		if wait, ok := m.manualWait(s, now); !ok {
			return func() tea.Msg { return noticeMsg{text: "Kimi refresh in " + ui.FormatCountdown(wait)} }
		}
	}
	s.refreshing = true
	s.lastAttempt = now
	s.err = ""

	source := m.kimiSource
	timeout := m.timeout
	redact := m.redact
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, err := source.Fetch(ctx, now)
		next := source.NextRefreshAllowed()
		if err != nil {
			return kimiFetchedMsg{nextAllowed: next, err: errors.New(redact(err.Error()))}
		}
		return kimiFetchedMsg{result: result, nextAllowed: next}
	}
}

// startOR begins one OpenRouter read unless one is already in flight or the
// manual throttle refuses it.
func (m *Model) startOR(manual bool) tea.Cmd {
	if m.orSource == nil {
		return nil
	}
	s := &m.orState
	now := m.now()
	if s.refreshing {
		return nil
	}
	if manual {
		if wait, ok := m.manualWait(s, now); !ok {
			return func() tea.Msg { return noticeMsg{text: "OpenRouter refresh in " + ui.FormatCountdown(wait)} }
		}
	}
	s.refreshing = true
	s.lastAttempt = now
	s.err = ""

	source := m.orSource
	timeout := m.timeout
	redact := m.redact
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		snapshot, err := source.Fetch(ctx, now)
		next := source.NextRefreshAllowed()
		if err != nil {
			return orFetchedMsg{nextAllowed: next, err: errors.New(redact(err.Error()))}
		}
		return orFetchedMsg{snapshot: snapshot, nextAllowed: next}
	}
}

// manualWait reports whether a manual refresh is allowed right now, and how
// long the caller must otherwise wait.
func (m *Model) manualWait(s *sourceState, now time.Time) (time.Duration, bool) {
	floor := m.cfg.ManualFloor
	required := s.lastAttempt.Add(floor)
	if s.nextAllowed.After(required) {
		required = s.nextAllowed
	}
	if now.Before(required) {
		return required.Sub(now), false
	}
	return 0, true
}

// applyKimi merges a Kimi result, keeping the last good snapshot on failure.
func (m *Model) applyKimi(msg kimiFetchedMsg) {
	s := &m.kimiState
	s.refreshing = false
	s.lastDone = m.now()
	if msg.nextAllowed.After(s.nextAllowed) {
		s.nextAllowed = msg.nextAllowed
	}
	if msg.err != nil {
		s.err = msg.err.Error()
		return
	}
	snapshot := msg.result.Snapshot
	m.kimiSnap = &snapshot
	m.kimiHistory = msg.result.History
	s.err = ""
	m.notice = ""
}

// applyOR merges an OpenRouter result, keeping the last good snapshot on
// failure.
func (m *Model) applyOR(msg orFetchedMsg) {
	s := &m.orState
	s.refreshing = false
	s.lastDone = m.now()
	if msg.nextAllowed.After(s.nextAllowed) {
		s.nextAllowed = msg.nextAllowed
	}
	if msg.err != nil {
		s.err = msg.err.Error()
		return
	}
	snapshot := msg.snapshot
	m.orSnap = &snapshot
	s.err = ""
	m.notice = ""
}

// armKimiTick schedules the next automatic Kimi read and remembers when it is
// due for the section's countdown.
func (m *Model) armKimiTick() tea.Cmd {
	if m.kimiSource == nil || m.cfg.KimiInterval <= 0 {
		return nil
	}
	now := m.now()
	delay := m.kimiState.nextDelay(now, m.cfg.KimiInterval)
	return tea.Tick(delay, func(time.Time) tea.Msg { return kimiTickMsg{} })
}

// armORTick schedules the next automatic OpenRouter read.
func (m *Model) armORTick() tea.Cmd {
	if m.orSource == nil || m.cfg.OpenRouterInterval <= 0 {
		return nil
	}
	now := m.now()
	delay := m.orState.nextDelay(now, m.cfg.OpenRouterInterval)
	return tea.Tick(delay, func(time.Time) tea.Msg { return orTickMsg{} })
}

// handleKey handles every key binding.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keys := ui.NewKeyMap()
	overlay := ui.NewOverlayKeyMap()
	page := maxInt(m.height-3, 1)

	if m.helpVisible {
		switch {
		case key.Matches(msg, overlay.Close):
			m.helpVisible, m.helpScroll = false, 0
		case key.Matches(msg, overlay.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Up):
			m.scrollHelp(-1)
		case key.Matches(msg, keys.Down):
			m.scrollHelp(1)
		case key.Matches(msg, keys.PageUp):
			m.scrollHelp(-page)
		case key.Matches(msg, keys.PageDown):
			m.scrollHelp(page)
		case key.Matches(msg, keys.Home):
			m.helpScroll = 0
		case key.Matches(msg, keys.End):
			m.helpScroll = m.maxHelpScroll()
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, keys.Quit), key.Matches(msg, keys.Escape):
		return m, tea.Quit
	case key.Matches(msg, keys.Refresh):
		m.notice = ""
		return m, m.refreshBoth()
	case key.Matches(msg, keys.Help):
		m.helpVisible, m.helpScroll = true, 0
	case key.Matches(msg, keys.Tab):
		m.tableFocused = !m.tableFocused
	case key.Matches(msg, keys.Up):
		m.scrollBy(-1)
	case key.Matches(msg, keys.Down):
		m.scrollBy(1)
	case key.Matches(msg, keys.PageUp):
		m.scrollBy(-page)
	case key.Matches(msg, keys.PageDown):
		m.scrollBy(page)
	case key.Matches(msg, keys.Home):
		m.scroll = 0
	case key.Matches(msg, keys.End):
		m.scroll = m.maxScroll()
	}
	return m, nil
}

// scrollBy scrolls the daily table when it has focus, otherwise the body.
func (m *Model) scrollBy(delta int) {
	if m.tableFocused {
		m.moveTable(delta)
		return
	}
	m.scroll = clampInt(m.scroll+delta, 0, m.maxScroll())
}

// moveTable scrolls the OpenRouter daily table by delta days.
func (m *Model) moveTable(delta int) {
	if m.orSnap == nil {
		return
	}
	limit := maxInt(len(m.orSnap.Daily)-1, 0)
	m.tableOffset = clampInt(m.tableOffset+delta, 0, limit)
}

func (m *Model) scrollHelp(delta int) {
	limit := m.maxHelpScroll()
	m.helpScroll = clampInt(m.helpScroll+delta, 0, limit)
}

// batch runs the non-nil commands, returning nil when there are none.
func batch(cmds ...tea.Cmd) tea.Cmd {
	filtered := make([]tea.Cmd, 0, len(cmds))
	for _, cmd := range cmds {
		if cmd != nil {
			filtered = append(filtered, cmd)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return tea.Batch(filtered...)
	}
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
