// Command dashburn is a read-only terminal dashboard that shows Kimi Code
// quota and OpenRouter credits side by side. It reads the credentials the
// existing kimiburn and routeburn applications already store, keeps nothing on
// disk, and never sends inference traffic.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/acevedomiguel/dashburn/internal/app"
	"github.com/acevedomiguel/dashburn/internal/config"
	"github.com/acevedomiguel/dashburn/internal/credentials"
	"github.com/acevedomiguel/dashburn/internal/provider"
)

// version is overridden at release time via -ldflags "-X main.version=...".
var version = "dev"

// Exit codes.
const (
	exitOK          = 0
	exitFailure     = 1
	exitUsage       = 2
	exitCredentials = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// options is the parsed command line.
type options struct {
	noColor    bool
	refresh    time.Duration
	refreshSet bool
	command    string
}

// usageError marks failures caused by the command line rather than the
// environment.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parse(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		var ue *usageError
		if errors.As(err, &ue) {
			fmt.Fprintf(stderr, "dashburn: %s\n", ue.msg)
		}
		return exitUsage
	}

	switch opts.command {
	case "version":
		fmt.Fprintf(stdout, "dashburn version %s\n", version)
		return exitOK
	case "help":
		printUsage(stdout)
		return exitOK
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		fmt.Fprintf(stderr, "dashburn: %v\n", err)
		return exitFailure
	}

	settings, warnings := config.Load(paths, nil)
	overrides := config.Overrides{NoColor: opts.noColor}
	if opts.refreshSet {
		interval := opts.refresh
		overrides.RefreshInterval = &interval
	}
	settings = settings.Apply(overrides)

	creds, err := resolveCredentials(paths, settings)
	if err != nil {
		fmt.Fprintf(stderr, "dashburn: %s\n", credentials.Redact(err.Error()))
		return exitFailure
	}

	redactor := credentials.NewRedactor(creds.Secrets()...)

	kimiSource, kimiHint := buildKimi(settings, creds)
	orSource, orHint := buildOpenRouter(creds)

	if kimiSource == nil && orSource == nil {
		fmt.Fprint(stderr, credentialsInstructions())
		return exitCredentials
	}

	model := app.New(app.Deps{
		Config: app.Config{
			KimiInterval:       settings.Kimi.RefreshInterval,
			OpenRouterInterval: settings.OpenRouter.RefreshInterval,
			Theme:              settings.Theme(),
			NoColor:            !settings.ColorsEnabled(),
			Version:            version,
		},
		Kimi:           kimiSource,
		OpenRouter:     orSource,
		KimiHint:       kimiHint,
		OpenRouterHint: orHint,
		Warnings:       warnings,
		Redact:         func(s string) string { return redactor.Redact(s) },
	})

	if _, err := tea.NewProgram(model, tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout)).Run(); err != nil {
		fmt.Fprintf(stderr, "dashburn: %s\n", redactor.Redact(err.Error()))
		return exitFailure
	}
	return exitOK
}

// parse turns argv into options.
func parse(args []string, stderr io.Writer) (options, error) {
	for _, a := range args {
		if a == "-h" || a == "-help" || a == "--help" {
			return options{command: "help"}, nil
		}
	}

	fs := flag.NewFlagSet("dashburn", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts options
	var showVersion bool
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.BoolVar(&opts.noColor, "no-color", false, "disable colored output")
	fs.DurationVar(&opts.refresh, "refresh", 0, "override both providers' refresh interval (0 disables auto-refresh)")

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "refresh" {
			opts.refreshSet = true
		}
	})

	if showVersion {
		opts.command = "version"
		return opts, nil
	}

	rest := fs.Args()
	switch {
	case len(rest) == 0:
		opts.command = "dashboard"
	case len(rest) == 1 && (rest[0] == "help" || rest[0] == "version"):
		opts.command = rest[0]
	default:
		return options{}, &usageError{msg: fmt.Sprintf("unknown command %q", strings.Join(rest, " "))}
	}

	if opts.refreshSet {
		if opts.refresh < 0 {
			return options{}, &usageError{msg: "--refresh must not be negative"}
		}
		if opts.refresh > 0 && opts.refresh < config.MinRefreshInterval {
			return options{}, &usageError{msg: fmt.Sprintf("--refresh must be %s or greater, or 0 to disable", config.MinRefreshInterval)}
		}
	}
	return opts, nil
}

// resolveCredentials reads the sibling credential stores. A missing provider is
// not an error here.
func resolveCredentials(paths config.Paths, settings config.Settings) (credentials.Creds, error) {
	var keyring credentials.Keyring
	if settings.Kimi.Keyring || settings.OpenRouter.Keyring {
		keyring = credentials.NewOSKeyring()
	}
	store := credentials.NewStore(credentials.Options{
		Keyring:                  keyring,
		KimiKeyringEnabled:       settings.Kimi.Keyring,
		OpenRouterKeyringEnabled: settings.OpenRouter.Keyring,
		KimiTokenFile:            paths.KimiTokenFile,
		OpenRouterManagementFile: paths.OpenRouterManageFile,
		OpenRouterAPIKeyFile:     paths.OpenRouterAPIKeyFile,
	})
	return store.Resolve(context.Background())
}

// buildKimi constructs the Kimi source, or a hint explaining why it is absent.
func buildKimi(settings config.Settings, creds credentials.Creds) (provider.KimiSource, string) {
	if !creds.HasKimi() {
		return nil, "No Kimi Code token found. Run `kimiburn setup` to store one, or export KIMIBURN_TOKEN."
	}
	source, err := provider.NewKimi(settings.Kimi.BaseURL, creds.KimiToken)
	if err != nil {
		return nil, "Kimi Code is misconfigured: " + credentials.Redact(err.Error())
	}
	return source, ""
}

// buildOpenRouter constructs the OpenRouter source, or a hint explaining why it
// is absent.
func buildOpenRouter(creds credentials.Creds) (provider.OpenRouterSource, string) {
	if !creds.HasOpenRouter() {
		return nil, "No OpenRouter management key found. Run `routeburn setup` to store one, or export ROUTEBURN_MANAGEMENT_KEY."
	}
	source, err := provider.NewOpenRouter(creds.ManagementKey, creds.APIKey)
	if err != nil {
		return nil, "OpenRouter is misconfigured: " + credentials.Redact(err.Error())
	}
	return source, ""
}

func credentialsInstructions() string {
	return "dashburn: no credentials found for either provider.\n\n" +
		"DashBurn reads the credentials kimiburn and routeburn already store; it never asks for its own.\n\n" +
		"  Kimi Code:   run `kimiburn setup`   or export KIMIBURN_TOKEN\n" +
		"  OpenRouter:  run `routeburn setup`  or export ROUTEBURN_MANAGEMENT_KEY\n"
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, "Usage:\n"+
		"  dashburn                  Start the unified live dashboard\n"+
		"  dashburn --version        Print version and exit\n"+
		"  dashburn --help           Print this help and exit\n"+
		"\nFlags:\n"+
		"  --refresh <duration>      Refresh interval for both providers (minimum 10s, 0 disables)\n"+
		"  --no-color                Disable colored output\n"+
		"\nDashBurn is read-only and keeps no cache. It reads the credentials the\n"+
		"existing kimiburn and routeburn applications store.\n"+
		"\nEngine: "+engineModule+" "+engineVersion()+"\n")
}
