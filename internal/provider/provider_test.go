package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/acevedomiguel/kimiburn"
	"github.com/acevedomiguel/routeburn"
)

func TestNewKimiAcceptsDefaultBase(t *testing.T) {
	src, err := NewKimi(kimiburn.DefaultBaseURL, "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src == nil {
		t.Fatal("expected a source")
	}
	if !src.NextRefreshAllowed().IsZero() {
		t.Errorf("a fresh source should have no rate-limit constraint")
	}
}

func TestNewKimiRejectsUnknownBaseURL(t *testing.T) {
	if _, err := NewKimi("https://example.com/v1", "token"); err == nil {
		t.Fatal("expected an unsupported base URL to be rejected")
	}
}

func TestNewKimiRejectsOverseasBaseWhenSelected(t *testing.T) {
	// The overseas base is a valid, explicitly selected alternative.
	if _, err := NewKimi(kimiburn.OverseasBaseURL, "token"); err != nil {
		t.Fatalf("overseas base should be accepted: %v", err)
	}
}

func TestOpenRouterFetchWithoutManagementKeyFails(t *testing.T) {
	src, err := NewOpenRouter("", "")
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}
	// The missing key is reported by the fetch boundary, before any network
	// request is made.
	if _, err := src.Fetch(context.Background(), time.Now()); !errors.Is(err, routeburn.ErrMissingManagementKey) {
		t.Fatalf("err = %v, want routeburn.ErrMissingManagementKey", err)
	}
}

func TestNewOpenRouterAcceptsManagementKey(t *testing.T) {
	src, err := NewOpenRouter("sk-or-v1-example", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !src.NextRefreshAllowed().IsZero() {
		t.Errorf("a fresh client should have no rate-limit constraint")
	}
}
