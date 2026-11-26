// Package provider adapts the public kimiburn and routeburn Go libraries into
// the two read-only sources DashBurn polls. Each source owns exactly one
// provider, serialises its own requests, and retains in-memory state so runway
// estimates improve across reads. Nothing here reads or writes a cache.
package provider

import (
	"context"
	"sync"
	"time"

	"github.com/acevedomiguel/burnblocks/kimi"
	"github.com/acevedomiguel/burnblocks/openrouter"
)

// KimiResult is one Kimi reading plus the retained local sample history the
// trend view draws.
type KimiResult struct {
	Snapshot kimi.Snapshot
	History  map[kimi.QuotaKey][]kimi.Sample
}

// KimiSource is the read-only Kimi Code boundary DashBurn depends on.
type KimiSource interface {
	Fetch(ctx context.Context, now time.Time) (KimiResult, error)
	NextRefreshAllowed() time.Time
}

// OpenRouterSource is the read-only OpenRouter boundary DashBurn depends on.
type OpenRouterSource interface {
	Fetch(ctx context.Context, now time.Time) (openrouter.Snapshot, error)
	NextRefreshAllowed() time.Time
}

// Kimi polls one Kimi Code account through the kimiburn library.
type Kimi struct {
	mu     sync.Mutex
	source kimi.UsageSource
	state  kimi.StoredState
}

// NewKimi builds a Kimi source from an explicitly supplied base URL and token.
// It reads no environment, keyring, configuration, or cache.
func NewKimi(baseURL, token string) (*Kimi, error) {
	source, err := kimi.NewSource(baseURL, token)
	if err != nil {
		return nil, err
	}
	return &Kimi{source: source, state: kimi.EmptyStoredState()}, nil
}

// Fetch performs one serialised read and returns the snapshot with the retained
// sample history. Reusing the source keeps its rate-limit constraint, and
// reusing the state keeps the local runway history.
func (k *Kimi) Fetch(ctx context.Context, now time.Time) (KimiResult, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	snapshot, err := kimi.Fetch(ctx, k.source, &k.state)
	if err != nil {
		return KimiResult{}, err
	}
	return KimiResult{Snapshot: snapshot, History: cloneSamples(k.state.Samples)}, nil
}

// NextRefreshAllowed reports the earliest instant another read may run, or the
// zero time when no constraint was observed.
func (k *Kimi) NextRefreshAllowed() time.Time {
	if k == nil || k.source == nil {
		return time.Time{}
	}
	return k.source.NextRefreshAllowedAt()
}

// OpenRouter polls one OpenRouter account through the routeburn library.
type OpenRouter struct {
	mu     sync.Mutex
	client *openrouter.Client
	prev   *openrouter.Snapshot
}

// NewOpenRouter builds an OpenRouter source from the caller-supplied keys. The
// management key is required; the regular API key is optional. It reads no
// environment, keyring, configuration, or cache.
func NewOpenRouter(managementKey, apiKey string) (*OpenRouter, error) {
	opts := []openrouter.Option{openrouter.WithManagementKey(managementKey)}
	if apiKey != "" {
		opts = append(opts, openrouter.WithAPIKey(apiKey))
	}
	client, err := openrouter.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &OpenRouter{client: client}, nil
}

// Fetch performs one serialised read. The previous in-memory snapshot is passed
// back to the library so an optional analytics or key failure degrades to a
// warning instead of blanking the section.
func (o *OpenRouter) Fetch(ctx context.Context, now time.Time) (openrouter.Snapshot, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	snapshot, err := openrouter.Fetch(ctx, o.client, openrouter.FetchOptions{
		Now:      now.UTC(),
		Previous: o.prev,
	})
	if err != nil {
		return openrouter.Snapshot{}, err
	}
	stored := snapshot
	o.prev = &stored
	return snapshot, nil
}

// NextRefreshAllowed reports the server-directed earliest next request time.
func (o *OpenRouter) NextRefreshAllowed() time.Time {
	if o == nil || o.client == nil {
		return time.Time{}
	}
	return o.client.NextRefreshAllowedAt()
}

// cloneSamples copies the sample history so the renderer never shares a slice
// with the goroutine that keeps appending to it.
func cloneSamples(samples map[kimi.QuotaKey][]kimi.Sample) map[kimi.QuotaKey][]kimi.Sample {
	if samples == nil {
		return nil
	}
	out := make(map[kimi.QuotaKey][]kimi.Sample, len(samples))
	for key, values := range samples {
		out[key] = append([]kimi.Sample(nil), values...)
	}
	return out
}
