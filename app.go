package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"raceguard/internal/config"
	"raceguard/internal/guard"
	"raceguard/internal/reporter"
	"raceguard/internal/runner"
	"raceguard/internal/storage"
	"raceguard/internal/transport"
)

var validScenarioID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

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

// SaveScenario saves a redacted scenario YAML strictly inside examples/<id>.yaml.
func (a *App) SaveScenario(s config.Scenario) (string, error) {
	id := strings.TrimSpace(s.ID)
	if !validScenarioID.MatchString(id) {
		return "", fmt.Errorf("invalid scenario ID %q: use 1-64 letters, digits, hyphens, or underscores", id)
	}
	s.ID = id
	if strings.TrimSpace(s.Name) == "" {
		s.Name = id
	}
	config.ApplyDefaults(&s)

	if err := a.policy.ValidateScenario(s); err != nil {
		return "", err
	}

	rawYAML, err := config.MarshalYAML(s)
	if err != nil {
		return "", err
	}

	baseDir, err := filepath.Abs("examples")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}
	targetPath, err := filepath.Abs(filepath.Join(baseDir, id+".yaml"))
	if err != nil {
		return "", err
	}
	if filepath.Dir(targetPath) != baseDir {
		return "", fmt.Errorf("safety guard: refusing to write outside examples directory")
	}

	if err := os.WriteFile(targetPath, []byte(rawYAML), 0o644); err != nil {
		return "", err
	}
	return filepath.Join("examples", id+".yaml"), nil
}

// ExportHTMLReport opens a native save dialog and writes a self-contained static HTML report.
func (a *App) ExportHTMLReport(res config.RunResult) (string, error) {
	defaultName := sanitizeFileStem(res.ScenarioID) + "-report.html"
	outPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save RaceGuard HTML Report",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "HTML Report (*.html)", Pattern: "*.html"},
		},
	})
	if err != nil || outPath == "" {
		return "", err
	}
	if err := reporter.WriteHTML(outPath, res); err != nil {
		return "", err
	}
	return outPath, nil
}

// ExportSARIFReport opens a native save dialog and writes a SARIF 2.1.0 report for CI/Security tools.
func (a *App) ExportSARIFReport(res config.RunResult) (string, error) {
	defaultName := sanitizeFileStem(res.ScenarioID) + ".sarif"
	outPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save RaceGuard SARIF 2.1.0 Report",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "SARIF Report (*.sarif;*.json)", Pattern: "*.sarif;*.json"},
		},
	})
	if err != nil || outPath == "" {
		return "", err
	}
	if err := reporter.WriteSARIF(outPath, res); err != nil {
		return "", err
	}
	return outPath, nil
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

func sanitizeFileStem(id string) string {
	id = strings.TrimSpace(id)
	if validScenarioID.MatchString(id) {
		return id
	}
	return "raceguard"
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
			UseH2C:    true,
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
