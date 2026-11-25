package credentials

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeKeyring is an in-memory Keyring for tests.
type fakeKeyring struct {
	values map[string]string
	err    error
}

func (f fakeKeyring) Get(service, user string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if v, ok := f.values[service+"/"+user]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}

func TestResolvePrefersEnvironment(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	if err := os.WriteFile(file, []byte("from-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(Options{
		Keyring:            fakeKeyring{values: map[string]string{"kimiburn/token": "from-keyring"}},
		KimiKeyringEnabled: true,
		KimiTokenFile:      file,
		Env:                map[string]string{EnvKimiToken: "from-env"},
	})
	creds, err := store.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if creds.KimiToken != "from-env" {
		t.Fatalf("KimiToken = %q, want from-env", creds.KimiToken)
	}
}

func TestResolveFallsBackToKeyringThenFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewStore(Options{
		Keyring:            fakeKeyring{values: map[string]string{"kimiburn/token": "from-keyring"}},
		KimiKeyringEnabled: true,
		KimiTokenFile:      file,
		Env:                map[string]string{},
	})
	creds, err := store.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if creds.KimiToken != "from-keyring" {
		t.Fatalf("KimiToken = %q, want from-keyring", creds.KimiToken)
	}

	// With the keyring disabled, the 0600 file wins.
	store = NewStore(Options{
		Keyring:            fakeKeyring{values: map[string]string{"kimiburn/token": "from-keyring"}},
		KimiKeyringEnabled: false,
		KimiTokenFile:      file,
		Env:                map[string]string{},
	})
	creds, err = store.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if creds.KimiToken != "from-file" {
		t.Fatalf("KimiToken = %q, want from-file", creds.KimiToken)
	}
}

func TestResolveRejectsInsecureFile(t *testing.T) {
	if !PermissionsAreEnforced {
		t.Skip("POSIX permission bits are not enforced on this platform")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	if err := os.WriteFile(file, []byte("wide-open"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(Options{
		KimiKeyringEnabled: false,
		KimiTokenFile:      file,
		Env:                map[string]string{},
	})
	if _, err := store.Resolve(context.Background()); !errors.Is(err, ErrInsecurePermissions) {
		t.Fatalf("err = %v, want ErrInsecurePermissions", err)
	}
}

func TestResolveMissingEverythingIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(Options{
		KimiTokenFile:            filepath.Join(dir, "absent-token"),
		OpenRouterManagementFile: filepath.Join(dir, "absent-management"),
		OpenRouterAPIKeyFile:     filepath.Join(dir, "absent-api-key"),
		Env:                      map[string]string{},
	})
	creds, err := store.Resolve(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.HasKimi() || creds.HasOpenRouter() {
		t.Fatalf("expected no credentials, got %+v", creds)
	}
}

func TestResolveOpenRouterSlots(t *testing.T) {
	store := NewStore(Options{
		OpenRouterKeyringEnabled: false,
		Env: map[string]string{
			EnvManagementKey: "mgmt",
			EnvOpenRouterKey: "compat-key",
		},
	})
	creds, err := store.Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if creds.ManagementKey != "mgmt" {
		t.Errorf("ManagementKey = %q", creds.ManagementKey)
	}
	if creds.APIKey != "compat-key" {
		t.Errorf("APIKey = %q, want the OPENROUTER_API_KEY fallback", creds.APIKey)
	}
	if !creds.HasOpenRouter() {
		t.Error("HasOpenRouter should be true")
	}
}

func TestSecretsListsEveryConfiguredValue(t *testing.T) {
	creds := Creds{KimiToken: "a", ManagementKey: "b", APIKey: ""}
	secrets := creds.Secrets()
	if len(secrets) != 2 {
		t.Fatalf("Secrets() = %v, want 2 non-empty values", secrets)
	}
}

func TestRedactorScrubsConfiguredSecret(t *testing.T) {
	// Deliberately not an sk-or-v1- shaped value: the CI credential scan
	// rejects that shape, and the redactor's literal match needs no realism.
	const secret = "or-test-secret-9f2c1a4b"
	r := NewRedactor(secret)
	got := r.Redact("request failed with Authorization: Bearer " + secret)
	if got == "" || contains(got, "9f2c1a4b") {
		t.Fatalf("secret was not redacted: %q", got)
	}
}

func TestRedactScrubsGenericKeyShape(t *testing.T) {
	got := Redact("using sk-abc123def456 with Bearer tok_abcdef")
	if contains(got, "sk-abc123def456") {
		t.Fatalf("generic sk- key was not redacted: %q", got)
	}
	if contains(got, "tok_abcdef") {
		t.Fatalf("generic bearer token was not redacted: %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
