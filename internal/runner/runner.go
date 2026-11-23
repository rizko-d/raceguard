package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"raceguard/internal/config"
	"raceguard/internal/evaluator"
	"raceguard/internal/guard"
	"raceguard/internal/transport"
)

// Runner coordinates Safety Guard checks, lifecycle hooks, concurrent execution, and invariant evaluation.
type Runner struct {
	Policy guard.Policy
}

// New creates a Runner initialized with the given Safety Guard policy.
func New(policy guard.Policy) *Runner {
	return &Runner{Policy: policy}
}

// Execute runs a single scenario end-to-end and returns a redacted RunResult.
func (r *Runner) Execute(
	ctx context.Context,
	s config.Scenario,
	onProgress func(transport.ProgressEvent),
) (config.RunResult, error) {
	config.ApplyDefaults(&s)

	if err := r.Policy.ValidateScenario(s); err != nil {
		return config.RunResult{}, err
	}

	useH2C := s.UseH2C && strings.HasPrefix(strings.ToLower(s.Target.URL), "http://")
	eng := transport.NewEngine(time.Duration(s.TimeoutMs)*time.Millisecond, useH2C)

	// 1. Optional pre-hook (e.g., reset staging voucher/wallet state)
	if s.PreHook != nil && strings.TrimSpace(s.PreHook.URL) != "" {
		status, body, err := eng.ExecuteHook(ctx, *s.PreHook)
		if err != nil {
			return config.RunResult{}, fmt.Errorf("pre_hook failed: %w", err)
		}
		if status >= 400 {
			return config.RunResult{}, fmt.Errorf("pre_hook returned status %d: %s", status, body)
		}
	}

	startedAt := time.Now()
	streams, totalMs, jitterMs := eng.ExecuteBatch(ctx, s, onProgress)

	// 2. Optional post-check (e.g., verify final DB/API state)
	var postCheck *config.PostCheckResult
	if s.PostCheck != nil && strings.TrimSpace(s.PostCheck.URL) != "" {
		status, body, err := eng.ExecuteHook(ctx, *s.PostCheck)
		pc := &config.PostCheckResult{
			URL:        s.PostCheck.URL,
			StatusCode: status,
			Body:       strings.TrimSpace(body),
		}
		if err != nil {
			pc.Error = err.Error()
		}
		postCheck = pc
	}

	verdict, summary, outcomes, remediations, succ, conf, errs := evaluator.Evaluate(s, streams, jitterMs, postCheck)

	return config.RunResult{
		RunID:           "run-" + uuid.NewString()[:8],
		ScenarioID:      s.ID,
		ScenarioName:    s.Name,
		Mode:            s.Mode,
		TargetURL:       s.Target.URL,
		Method:          s.Target.Method,
		StartedAt:       startedAt.Format(time.RFC3339),
		TotalDurationMs: totalMs,
		JitterSpreadMs:  jitterMs,
		SuccessCount:    succ,
		ConflictCount:   conf,
		ErrorCount:      errs,
		Verdict:         verdict,
		Summary:         summary,
		Remediations:    remediations,
		Invariants:      outcomes,
		Streams:         streams,
		PostCheck:       postCheck,
	}, nil
}
