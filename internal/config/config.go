// Package config reads the non-secret settings DashBurn derives from the
// existing kimiburn and routeburn configuration files. It never writes them and
// never stores a credential.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/acevedomiguel/kimiburn"
)

// DefaultRefreshInterval is the automatic polling interval each provider uses
// when its own configuration does not specify one.
const DefaultRefreshInterval = 30 * time.Second

// MinRefreshInterval is the floor for a positive automatic interval. A value of
// zero disables automatic refresh and leaves manual refresh available.
const MinRefreshInterval = 10 * time.Second

// KimiSettings is the non-secret subset of kimiburn's configuration.
type KimiSettings struct {
	BaseURL         string
	RefreshInterval time.Duration
	Keyring         bool
	Theme           string
	Color           string
}

// OpenRouterSettings is the non-secret subset of routeburn's configuration.
type OpenRouterSettings struct {
	RefreshInterval time.Duration
	Keyring         bool
	Theme           string
	Color           string
}

// Settings bundles both providers' resolved settings.
type Settings struct {
	Kimi       KimiSettings
	OpenRouter OpenRouterSettings
}

// Overrides carries explicit command-line flags, which win over everything
// else.
type Overrides struct {
	RefreshInterval *time.Duration
	NoColor         bool
}

// DefaultKimi returns kimiburn's built-in non-secret defaults.
func DefaultKimi() KimiSettings {
	return KimiSettings{
		BaseURL:         kimiburn.DefaultBaseURL,
		RefreshInterval: DefaultRefreshInterval,
		Keyring:         true,
		Theme:           "auto",
		Color:           "auto",
	}
}

// DefaultOpenRouter returns routeburn's built-in non-secret defaults.
func DefaultOpenRouter() OpenRouterSettings {
	return OpenRouterSettings{
		RefreshInterval: DefaultRefreshInterval,
		Keyring:         true,
		Theme:           "auto",
		Color:           "auto",
	}
}

// kimiFile is the lenient view of kimiburn's configuration file. Unknown keys
// are ignored: the file belongs to the sibling application, which rejects its
// own unknown keys already.
type kimiFile struct {
	Version         int           `yaml:"version"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	BaseURL         string        `yaml:"base_url"`
	Theme           string        `yaml:"theme"`
	Color           string        `yaml:"color"`
	Keyring         *bool         `yaml:"keyring"`
}

// routeBurnFile is the lenient view of routeburn's configuration file.
type routeBurnFile struct {
	Version         int           `yaml:"version"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	Theme           string        `yaml:"theme"`
	Color           string        `yaml:"color"`
	Keyring         *bool         `yaml:"keyring"`
}

// Load resolves both providers' settings from their configuration files and the
// environment overrides the siblings honour. A missing file is not an error; an
// unreadable or malformed file degrades to the built-in default and produces a
// human-readable warning.
func Load(paths Paths, env map[string]string) (Settings, []string) {
	lookup := func(key string) string {
		if env != nil {
			return strings.TrimSpace(env[key])
		}
		return strings.TrimSpace(os.Getenv(key))
	}

	s := Settings{Kimi: DefaultKimi(), OpenRouter: DefaultOpenRouter()}
	var warnings []string

	var kimi kimiFile
	if err := readYAML(paths.KimiConfigFile, &kimi); err != nil {
		warnings = append(warnings, "kimiburn config: "+err.Error())
	} else {
		if strings.TrimSpace(kimi.BaseURL) != "" {
			s.Kimi.BaseURL = strings.TrimSpace(kimi.BaseURL)
		}
		if kimi.RefreshInterval != 0 {
			s.Kimi.RefreshInterval = kimi.RefreshInterval
		}
		if kimi.Keyring != nil {
			s.Kimi.Keyring = *kimi.Keyring
		}
		if strings.TrimSpace(kimi.Theme) != "" {
			s.Kimi.Theme = strings.TrimSpace(kimi.Theme)
		}
		if strings.TrimSpace(kimi.Color) != "" {
			s.Kimi.Color = strings.TrimSpace(kimi.Color)
		}
	}

	var or routeBurnFile
	if err := readYAML(paths.OpenRouterConfigFile, &or); err != nil {
		warnings = append(warnings, "routeburn config: "+err.Error())
	} else {
		if or.RefreshInterval != 0 {
			s.OpenRouter.RefreshInterval = or.RefreshInterval
		}
		if or.Keyring != nil {
			s.OpenRouter.Keyring = *or.Keyring
		}
		if strings.TrimSpace(or.Theme) != "" {
			s.OpenRouter.Theme = strings.TrimSpace(or.Theme)
		}
		if strings.TrimSpace(or.Color) != "" {
			s.OpenRouter.Color = strings.TrimSpace(or.Color)
		}
	}

	// Environment overrides, in the same variables the siblings read.
	if v := lookup("KIMIBURN_BASE_URL"); v != "" {
		s.Kimi.BaseURL = v
	}
	if d, err := parseInterval(lookup("KIMIBURN_REFRESH_INTERVAL")); err != nil {
		warnings = append(warnings, "KIMIBURN_REFRESH_INTERVAL: "+err.Error())
	} else if d != nil {
		s.Kimi.RefreshInterval = *d
	}
	if v := lookup("KIMIBURN_KEYRING"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			s.Kimi.Keyring = b
		} else {
			warnings = append(warnings, "KIMIBURN_KEYRING: "+err.Error())
		}
	}
	if v := lookup("KIMIBURN_THEME"); v != "" {
		s.Kimi.Theme = v
	}
	if v := lookup("KIMIBURN_COLOR"); v != "" {
		s.Kimi.Color = v
	}

	if d, err := parseInterval(lookup("ROUTEBURN_REFRESH_INTERVAL")); err != nil {
		warnings = append(warnings, "ROUTEBURN_REFRESH_INTERVAL: "+err.Error())
	} else if d != nil {
		s.OpenRouter.RefreshInterval = *d
	}
	if v := lookup("ROUTEBURN_KEYRING"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			s.OpenRouter.Keyring = b
		} else {
			warnings = append(warnings, "ROUTEBURN_KEYRING: "+err.Error())
		}
	}
	if v := lookup("ROUTEBURN_THEME"); v != "" {
		s.OpenRouter.Theme = v
	}
	if v := lookup("ROUTEBURN_COLOR"); v != "" {
		s.OpenRouter.Color = v
	}

	warnings = append(warnings, s.clampIntervals()...)
	return s, warnings
}

// clampIntervals enforces the floor and a non-negative value, mirroring the
// siblings' validation while keeping the dashboard usable.
func (s *Settings) clampIntervals() []string {
	var warnings []string
	clamp := func(name string, d time.Duration) time.Duration {
		if d < 0 {
			warnings = append(warnings, name+" was negative; automatic refresh disabled")
			return 0
		}
		if d > 0 && d < MinRefreshInterval {
			warnings = append(warnings, fmt.Sprintf("%s was below %s; raised to the minimum", name, MinRefreshInterval))
			return MinRefreshInterval
		}
		return d
	}
	s.Kimi.RefreshInterval = clamp("kimiburn refresh_interval", s.Kimi.RefreshInterval)
	s.OpenRouter.RefreshInterval = clamp("routeburn refresh_interval", s.OpenRouter.RefreshInterval)
	return warnings
}

// Apply layers explicit flags on top of the resolved settings.
func (s Settings) Apply(o Overrides) Settings {
	if o.RefreshInterval != nil {
		s.Kimi.RefreshInterval = *o.RefreshInterval
		s.OpenRouter.RefreshInterval = *o.RefreshInterval
	}
	if o.NoColor {
		s.Kimi.Color = "never"
		s.OpenRouter.Color = "never"
	}
	return s
}

// ColorsEnabled reports whether any colour should be emitted. NO_COLOR and an
// explicit --no-color both disable it.
func (s Settings) ColorsEnabled() bool {
	if _, off := os.LookupEnv("NO_COLOR"); off {
		return false
	}
	return s.Kimi.Color != "never" || s.OpenRouter.Color != "never"
}

// Theme resolves the palette preference. A theme both providers agree on is
// honoured; otherwise the terminal's own background decides.
func (s Settings) Theme() string {
	pick := func(t string) string {
		switch t {
		case "dark", "light":
			return t
		default:
			return "auto"
		}
	}
	kimi, or := pick(s.Kimi.Theme), pick(s.OpenRouter.Theme)
	if kimi == or {
		return kimi
	}
	return "auto"
}

func parseInterval(v string) (*time.Duration, error) {
	if v == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// readYAML decodes the first document of path into dst. A missing file leaves
// dst untouched. Unknown keys are ignored.
func readYAML(path string, dst any) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
