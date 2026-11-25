package credentials

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Service names and accounts owned by the sibling applications.
const (
	KimiService = "kimiburn"
	KimiAccount = "token"

	OpenRouterService           = "routeburn"
	OpenRouterManagementAccount = "management"
	OpenRouterAPIKeyAccount     = "api-key"
)

// Environment variables the sibling applications read credentials from.
const (
	EnvKimiToken     = "KIMIBURN_TOKEN"
	EnvManagementKey = "ROUTEBURN_MANAGEMENT_KEY"
	EnvAPIKey        = "ROUTEBURN_API_KEY"
	EnvOpenRouterKey = "OPENROUTER_API_KEY"
)

// Sentinel errors surfaced by the credential store.
var (
	ErrInsecurePermissions = errors.New("credentials: the credential file must be readable only by its owner (0600)")
)

// Options configures a Store. Tests inject a fake keyring and temp paths.
type Options struct {
	Keyring Keyring
	// KimiKeyringEnabled and OpenRouterKeyringEnabled mirror each sibling's
	// own `keyring` setting, so a provider whose keyring is disabled never
	// consults it.
	KimiKeyringEnabled       bool
	OpenRouterKeyringEnabled bool

	KimiTokenFile            string
	OpenRouterManagementFile string
	OpenRouterAPIKeyFile     string
	Env                      map[string]string
}

// Store resolves credentials from the same surfaces the sibling applications
// use. It only reads: it has no Save or Logout.
type Store struct {
	keyring                  Keyring
	kimiKeyringEnabled       bool
	openRouterKeyringEnabled bool

	kimiTokenFile    string
	orManagementFile string
	orAPIKeyFile     string

	// env overrides the process environment; nil means os.Getenv.
	env map[string]string
}

// NewStore builds a store from explicit options.
func NewStore(opts Options) *Store {
	return &Store{
		keyring:                  opts.Keyring,
		kimiKeyringEnabled:       opts.KimiKeyringEnabled && opts.Keyring != nil,
		openRouterKeyringEnabled: opts.OpenRouterKeyringEnabled && opts.Keyring != nil,
		kimiTokenFile:            opts.KimiTokenFile,
		orManagementFile:         opts.OpenRouterManagementFile,
		orAPIKeyFile:             opts.OpenRouterAPIKeyFile,
		env:                      opts.Env,
	}
}

// Creds is the resolved credential set. An empty value means the provider is
// not configured through any of the surfaces DashBurn consults.
type Creds struct {
	KimiToken     string
	ManagementKey string
	APIKey        string
}

// HasKimi reports whether a Kimi Code token is available.
func (c Creds) HasKimi() bool { return strings.TrimSpace(c.KimiToken) != "" }

// HasOpenRouter reports whether an OpenRouter management key is available.
func (c Creds) HasOpenRouter() bool { return strings.TrimSpace(c.ManagementKey) != "" }

// Secrets lists every non-empty secret, for the redactor.
func (c Creds) Secrets() []string {
	var out []string
	for _, v := range []string{c.KimiToken, c.ManagementKey, c.APIKey} {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Resolve returns every credential the process can use right now. A missing
// credential is not an error here; callers decide how to react.
func (s *Store) Resolve(ctx context.Context) (Creds, error) {
	if err := ctx.Err(); err != nil {
		return Creds{}, err
	}

	kimi, err := s.resolve(ctx, []string{EnvKimiToken}, s.kimiKeyringEnabled, KimiService, KimiAccount, s.kimiTokenFile)
	if err != nil {
		return Creds{}, err
	}
	management, err := s.resolve(ctx, []string{EnvManagementKey}, s.openRouterKeyringEnabled, OpenRouterService, OpenRouterManagementAccount, s.orManagementFile)
	if err != nil {
		return Creds{}, err
	}
	apiKey, err := s.resolve(ctx, []string{EnvAPIKey, EnvOpenRouterKey}, s.openRouterKeyringEnabled, OpenRouterService, OpenRouterAPIKeyAccount, s.orAPIKeyFile)
	if err != nil {
		return Creds{}, err
	}
	return Creds{KimiToken: kimi, ManagementKey: management, APIKey: apiKey}, nil
}

// resolve reads one slot in the documented order: environment, keyring, then a
// permission-restricted fallback file. A slot whose environment variable is set
// never touches the keyring or the filesystem.
func (s *Store) resolve(ctx context.Context, envNames []string, keyringEnabled bool, service, account, file string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	for _, name := range envNames {
		if v := s.lookupEnv(name); v != "" {
			return v, nil
		}
	}
	if keyringEnabled {
		if v, err := s.keyring.Get(service, account); err == nil {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				return trimmed, nil
			}
		}
	}
	return readSecretFile(file)
}

func (s *Store) lookupEnv(key string) string {
	if s.env != nil {
		return strings.TrimSpace(s.env[key])
	}
	return strings.TrimSpace(os.Getenv(key))
}

// readSecretFile enforces 0600 (no group or world access) before reading. A
// missing file is not an error.
func readSecretFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("credentials: inspect %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("credentials: %s is a directory", path)
	}
	// Windows reports 0666 for every file, so the group/world check would reject
	// the fallback unconditionally. There the file's protection comes from the
	// per-user profile ACL it lives in.
	if PermissionsAreEnforced {
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			return "", ErrInsecurePermissions
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("credentials: read %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}
