package ui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
)

// helpStyles adapts the terminal palette to the bubbles help bubble.
func helpStyles(s Styles) help.Styles {
	if !s.Enabled {
		empty := lipgloss.NewStyle()
		return help.Styles{
			Ellipsis:       empty,
			ShortKey:       empty,
			ShortDesc:      empty,
			ShortSeparator: empty,
			FullKey:        empty,
			FullDesc:       empty,
			FullSeparator:  empty,
		}
	}
	return help.Styles{
		Ellipsis:       s.Muted,
		ShortKey:       s.Key,
		ShortDesc:      s.Muted,
		ShortSeparator: s.Muted,
		FullKey:        s.Key,
		FullDesc:       s.Muted,
		FullSeparator:  s.Muted,
	}
}

// NewHelp builds the bubbles help bubble used for the footer and overlay.
func NewHelp(s Styles, width int) help.Model {
	h := help.New()
	h.SetWidth(maxInt(width, 1))
	h.ShortSeparator = " · "
	h.FullSeparator = "    "
	h.Styles = helpStyles(s)
	return h
}

// NewSpinner builds the refresh indicator.
func NewSpinner(s Styles) spinner.Model {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	if s.Enabled {
		sp.Style = s.Accent
	}
	return sp
}
