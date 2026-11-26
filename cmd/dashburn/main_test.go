package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/acevedomiguel/dashburn/internal/config"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--version"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitOK, stderr.String())
	}
	if !strings.Contains(stdout.String(), "dashburn version") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"frobnicate"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsRefreshBelowMinimum(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--refresh", "2s"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "refresh") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsNegativeRefresh(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--refresh", "-1s"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
}

func TestParseRefreshFlag(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parse([]string{"--refresh", "45s", "--no-color"}, &stderr)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !opts.refreshSet || opts.refresh.String() != "45s" {
		t.Fatalf("refresh = %s (set=%t)", opts.refresh, opts.refreshSet)
	}
	if !opts.noColor {
		t.Fatal("--no-color should be set")
	}
	if opts.command != "dashboard" {
		t.Fatalf("command = %q", opts.command)
	}
}

func TestParseAcceptsMinimumRefresh(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parse([]string{"--refresh", config.MinRefreshInterval.String()}, &stderr)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if opts.refresh != config.MinRefreshInterval {
		t.Fatalf("refresh = %s", opts.refresh)
	}
}

// The help output names the shared engine the binary embeds, so a support
// report can identify it after both engines moved to their own module.
func TestHelpReportsEngineVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "github.com/acevedomiguel/burnblocks") {
		t.Errorf("help must name the engine module:\n%s", stdout.String())
	}
	if got := engineVersion(); got == "" {
		t.Error("engineVersion must never be empty")
	}
}
