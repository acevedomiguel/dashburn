package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Paths holds every sibling location DashBurn reads. DashBurn never writes
// these files: it reads only the configuration and credentials the two
// applications already own.
type Paths struct {
	KimiConfigDir  string
	KimiConfigFile string
	KimiTokenFile  string

	OpenRouterConfigDir  string
	OpenRouterConfigFile string
	OpenRouterManageFile string
	OpenRouterAPIKeyFile string
}

// DefaultPaths resolves the two sibling configuration directories the same way
// each application resolves its own, including their different XDG handling.
func DefaultPaths() (Paths, error) {
	kimiBase, err := kimiConfigBase()
	if err != nil {
		return Paths{}, fmt.Errorf("config: resolve user config dir: %w", err)
	}
	orBase, err := openRouterConfigBase()
	if err != nil {
		return Paths{}, fmt.Errorf("config: resolve user config dir: %w", err)
	}

	kimiDir := filepath.Join(kimiBase, "kimiburn")
	orDir := filepath.Join(orBase, "routeburn")
	return Paths{
		KimiConfigDir:        kimiDir,
		KimiConfigFile:       filepath.Join(kimiDir, "config.yaml"),
		KimiTokenFile:        filepath.Join(kimiDir, "token"),
		OpenRouterConfigDir:  orDir,
		OpenRouterConfigFile: filepath.Join(orDir, "config.yaml"),
		OpenRouterManageFile: filepath.Join(orDir, "management-token"),
		OpenRouterAPIKeyFile: filepath.Join(orDir, "api-key"),
	}, nil
}

const xdgConfigHome = "XDG_CONFIG_HOME"

// kimiConfigBase mirrors kimiburn's own resolution: an absolute XDG override
// wins, and otherwise the platform default is used. On macOS the platform
// default ignores XDG, which is why the override is suppressed while the
// fallback runs: a relative value must not make the resolved path depend on the
// process working directory.
func kimiConfigBase() (string, error) {
	v := strings.TrimSpace(os.Getenv(xdgConfigHome))
	if v != "" && filepath.IsAbs(v) {
		return v, nil
	}

	set := false
	if _, ok := os.LookupEnv(xdgConfigHome); ok {
		set = true
		if err := os.Unsetenv(xdgConfigHome); err != nil {
			return "", err
		}
	}
	defer func() {
		if set {
			_ = os.Setenv(xdgConfigHome, v)
		}
	}()
	return os.UserConfigDir()
}

// openRouterConfigBase mirrors routeburn's resolution: any non-empty XDG
// override wins, otherwise the platform default is used.
func openRouterConfigBase() (string, error) {
	if v := strings.TrimSpace(os.Getenv(xdgConfigHome)); v != "" {
		return v, nil
	}
	return os.UserConfigDir()
}
