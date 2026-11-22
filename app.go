package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"raceguard/internal/config"
	"raceguard/internal/guard"
	"raceguard/internal/runner"
	"raceguard/internal/storage"
	"raceguard/internal/transport"
)

// App exposes strictly scoped, safe-by-default Wails IPC bindings for the desktop UI.
type App struct {
	ctx    context.Context
	policy guard.Policy
	runner *runner.Runner
	store  *storage.Store
}

// NewApp creates a new RaceGuard desktop application instance.
func NewApp() *App {
	policy := guard.DefaultPolicy()
	return &App{
		policy: policy,
		runner: runner.New(policy),
		store:  storage.New(filepath.Join(".raceguard", "history.json")),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetSafetyPolicy returns active Safety Guard constraints (allowlist, max streams, max timeout).
func (a *App) GetSafetyPolicy() guard.Policy {
	return a.policy
}

// ListExampleScenarios loads bundled YAML scenarios from the examples/ directory.
func (a *App) ListExampleScenarios() []config.Scenario {
	entries, err := os.ReadDir("examples")
	if err != nil {
		return defaultScenarios()
	}

	var scenarios []config.Scenario
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		if s, err := config.LoadScenario(filepath.Join("examples", entry.Name())); err == nil {
			scenarios = append(scenarios, s)
		}
	}
	if len(scenarios) == 0 {
		return defaultScenarios()
	}
	sort.Slice(scenarios, func(i, j int) bool {
		return scenarios[i].ID < scenarios[j].ID
	})
	return scenarios
}

// RunScenario validates a scenario against Safety Guard, executes it, persists history, and returns the redacted result.
func (a *App) RunScenario(s config.Scenario) (config.RunResult, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	res, err := a.runner.Execute(ctx, s, func(ev transport.ProgressEvent) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "run:progress", ev)
		}
	})
	if err != nil {
		return config.RunResult{}, err
	}

	_ = a.store.Append(res)
	return res, nil
}

// ListHistory returns previously executed runs from local storage.
func (a *App) ListHistory() ([]config.RunResult, error) {
	return a.store.List()
}

// ClearHistory deletes saved run history.
func (a *App) ClearHistory() error {
	return a.store.Clear()
}

// ExportScenarioYAML converts the active scenario into a redacted YAML string for CI/CD pipelines.
func (a *App) ExportScenarioYAML(s config.Scenario) (string, error) {
	config.ApplyDefaults(&s)
	return config.MarshalYAML(s)
}

func defaultScenarios() []config.Scenario {
	return []config.Scenario{
		{
			ID:        "voucher-redeem-vulnerable",
			Name:      "Voucher Redeem (Unprotected)",
			Mode:      config.ModeConcurrency,
			Streams:   10,
			TimeoutMs: 5000,
			Warmup:    true,
			Target: config.TargetConfig{
				Method: "POST",
				URL:    "http://127.0.0.1:8090/api/v1/vulnerable/redeem",
				Body:   `{"voucher_code":"PROMO2026"}`,
			},
			PreHook: &config.HookConfig{
				Method: "POST",
				URL:    "http://127.0.0.1:8090/api/v1/reset",
			},
			PostCheck: &config.HookConfig{
				Method: "GET",
				URL:    "http://127.0.0.1:8090/api/v1/state",
			},
			Invariants: config.InvariantConfig{
				MaxSuccessCount:       1,
				PostCheckField:        "used_count",
				PostCheckExpected:     "1",
				MaxAcceptableJitterMs: 50,
			},
		},
	}
}
