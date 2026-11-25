package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/acevedomiguel/kimiburn"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestLoadDefaultsWhenFilesAreMissing(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{
		KimiConfigFile:       filepath.Join(dir, "kimiburn.yaml"),
		OpenRouterConfigFile: filepath.Join(dir, "routeburn.yaml"),
	}
	settings, warnings := Load(paths, map[string]string{})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if settings.Kimi.BaseURL != kimiburn.DefaultBaseURL {
		t.Errorf("default base URL = %q", settings.Kimi.BaseURL)
	}
	if settings.Kimi.RefreshInterval != DefaultRefreshInterval {
		t.Errorf("default Kimi interval = %s", settings.Kimi.RefreshInterval)
	}
	if !settings.Kimi.Keyring || !settings.OpenRouter.Keyring {
		t.Error("keyring should default to enabled")
	}
}

func TestLoadReadsSiblingFiles(t *testing.T) {
	dir := t.TempDir()
	kimiFile := writeFile(t, dir, "kimiburn.yaml", "version: 1\nrefresh_interval: 45s\nbase_url: https://api.kimi.ai/coding/v1\nkeyring: false\n")
	orFile := writeFile(t, dir, "routeburn.yaml", "version: 1\nrefresh_interval: 15s\n")

	settings, warnings := Load(Paths{KimiConfigFile: kimiFile, OpenRouterConfigFile: orFile}, map[string]string{})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if settings.Kimi.BaseURL != kimiburn.OverseasBaseURL {
		t.Errorf("base URL = %q", settings.Kimi.BaseURL)
	}
	if settings.Kimi.RefreshInterval != 45*time.Second {
		t.Errorf("Kimi interval = %s", settings.Kimi.RefreshInterval)
	}
	if settings.Kimi.Keyring {
		t.Error("kimiburn keyring should be disabled")
	}
	if settings.OpenRouter.RefreshInterval != 15*time.Second {
		t.Errorf("OpenRouter interval = %s", settings.OpenRouter.RefreshInterval)
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	kimiFile := writeFile(t, dir, "kimiburn.yaml", "version: 1\nrefresh_interval: 45s\n")
	env := map[string]string{
		"KIMIBURN_REFRESH_INTERVAL": "90s",
		"KIMIBURN_KEYRING":          "false",
	}
	settings, warnings := Load(Paths{KimiConfigFile: kimiFile}, env)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if settings.Kimi.RefreshInterval != 90*time.Second {
		t.Errorf("interval = %s, want 90s", settings.Kimi.RefreshInterval)
	}
	if settings.Kimi.Keyring {
		t.Error("keyring should be disabled by the environment")
	}
}

func TestLoadClampsIntervals(t *testing.T) {
	dir := t.TempDir()
	kimiFile := writeFile(t, dir, "kimiburn.yaml", "version: 1\nrefresh_interval: 2s\n")
	settings, warnings := Load(Paths{KimiConfigFile: kimiFile}, map[string]string{})
	if settings.Kimi.RefreshInterval != MinRefreshInterval {
		t.Errorf("interval = %s, want %s", settings.Kimi.RefreshInterval, MinRefreshInterval)
	}
	if len(warnings) == 0 {
		t.Error("expected a clamp warning")
	}
}

func TestLoadMalformedFileWarnsAndDefaults(t *testing.T) {
	dir := t.TempDir()
	bad := writeFile(t, dir, "kimiburn.yaml", "version: [this is not valid\n")
	settings, warnings := Load(Paths{KimiConfigFile: bad}, map[string]string{})
	if len(warnings) == 0 {
		t.Fatal("expected a parse warning")
	}
	if settings.Kimi.RefreshInterval != DefaultRefreshInterval {
		t.Errorf("interval = %s, want the default", settings.Kimi.RefreshInterval)
	}
}

func TestApplyOverrides(t *testing.T) {
	settings := Settings{Kimi: DefaultKimi(), OpenRouter: DefaultOpenRouter()}
	interval := 12 * time.Second
	settings = settings.Apply(Overrides{RefreshInterval: &interval, NoColor: true})
	if settings.Kimi.RefreshInterval != 12*time.Second || settings.OpenRouter.RefreshInterval != 12*time.Second {
		t.Errorf("refresh override not applied: %s/%s", settings.Kimi.RefreshInterval, settings.OpenRouter.RefreshInterval)
	}
	if settings.ColorsEnabled() {
		t.Error("--no-color should disable colours")
	}
}

func TestThemeAgreement(t *testing.T) {
	settings := Settings{Kimi: DefaultKimi(), OpenRouter: DefaultOpenRouter()}
	if got := settings.Theme(); got != "auto" {
		t.Errorf("theme = %q, want auto", got)
	}
	settings.Kimi.Theme = "dark"
	settings.OpenRouter.Theme = "dark"
	if got := settings.Theme(); got != "dark" {
		t.Errorf("theme = %q, want dark", got)
	}
	settings.OpenRouter.Theme = "light"
	if got := settings.Theme(); got != "auto" {
		t.Errorf("theme = %q, want auto when the providers disagree", got)
	}
}

func TestColorsEnabledHonoursNoColorEnv(t *testing.T) {
	settings := Settings{Kimi: DefaultKimi(), OpenRouter: DefaultOpenRouter()}
	t.Setenv("NO_COLOR", "1")
	if settings.ColorsEnabled() {
		t.Error("NO_COLOR should disable colours")
	}
}
