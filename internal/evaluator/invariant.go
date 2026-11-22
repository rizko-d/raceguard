package evaluator

import (
	"encoding/json"
	"fmt"
	"strings"

	"raceguard/internal/config"
)

// Evaluate checks all business-logic invariants and computes the final verdict and remediation advice.
func Evaluate(
	s config.Scenario,
	streams []config.StreamResult,
	jitterMs float64,
	postCheck *config.PostCheckResult,
) (config.Verdict, string, []config.InvariantOutcome, []string, int, int, int) {
	var (
		successCount  int
		conflictCount int
		errorCount    int
		successBodies []string
	)

	for _, st := range streams {
		switch {
		case st.Error != "":
			errorCount++
		case st.StatusCode >= 200 && st.StatusCode < 300:
			successCount++
			successBodies = append(successBodies, st.BodyPreview)
		case st.StatusCode == 409 || st.StatusCode == 429 || st.StatusCode == 400 || st.StatusCode == 422:
			conflictCount++
		case st.StatusCode >= 500:
			errorCount++
		}
	}

	var outcomes []config.InvariantOutcome
	failed := false

	// 1. Concurrency / Idempotency success count check
	if s.Mode == config.ModeIdempotency && s.Invariants.RequireIdenticalIdemp {
		allIdentical := true
		if len(successBodies) > 1 {
			first := successBodies[0]
			for _, b := range successBodies[1:] {
				if b != first {
					allIdentical = false
					break
				}
			}
		}
		passed := allIdentical && successCount >= 1
		actual := fmt.Sprintf("%dx 2xx (identical=%v)", successCount, allIdentical)
		outcomes = append(outcomes, config.InvariantOutcome{
			Rule:     "idempotent_response_consistency",
			Expected: "all 2xx responses share identical payload/transaction ID",
			Actual:   actual,
			Passed:   passed,
		})
		if !passed {
			failed = true
		}
	} else {
		passed := successCount <= s.Invariants.MaxSuccessCount
		outcomes = append(outcomes, config.InvariantOutcome{
			Rule:     "max_success_count",
			Expected: fmt.Sprintf("<= %d successful (2xx) responses", s.Invariants.MaxSuccessCount),
			Actual:   fmt.Sprintf("%d successful (2xx) responses", successCount),
			Passed:   passed,
		})
		if !passed {
			failed = true
		}
	}

	// 2. Post-check JSON field invariant
	if postCheck != nil && strings.TrimSpace(s.Invariants.PostCheckField) != "" {
		field := strings.TrimSpace(s.Invariants.PostCheckField)
		expected := strings.TrimSpace(s.Invariants.PostCheckExpected)
		if postCheck.Error != "" {
			outcomes = append(outcomes, config.InvariantOutcome{
				Rule:     "post_check:" + field,
				Expected: expected,
				Actual:   "error: " + postCheck.Error,
				Passed:   false,
			})
			failed = true
		} else {
			actualVal, ok := extractJSONField(postCheck.Body, field)
			passed := ok && actualVal == expected
			if !ok {
				actualVal = "<field not found>"
			}
			outcomes = append(outcomes, config.InvariantOutcome{
				Rule:     "post_check:" + field,
				Expected: expected,
				Actual:   actualVal,
				Passed:   passed,
			})
			if !passed {
				failed = true
			}
		}
	}

	// 3. Synchronization jitter reliability check
	maxJitter := s.Invariants.MaxAcceptableJitterMs
	if maxJitter <= 0 {
		maxJitter = 50.0
	}
	jitterOK := jitterMs <= maxJitter
	outcomes = append(outcomes, config.InvariantOutcome{
		Rule:     "dispatch_jitter_spread",
		Expected: fmt.Sprintf("<= %.1f ms", maxJitter),
		Actual:   fmt.Sprintf("%.2f ms", jitterMs),
		Passed:   jitterOK,
	})

	if failed {
		remediations := buildRemediations(s.Mode)
		summary := fmt.Sprintf("Invariant violated: %d/%d requests returned 2xx (limit: %d).", successCount, len(streams), s.Invariants.MaxSuccessCount)
		return config.VerdictFail, summary, outcomes, remediations, successCount, conflictCount, errorCount
	}

	if errorCount == len(streams) && len(streams) > 0 {
		return config.VerdictInconclusive, "All concurrent requests failed with transport/server errors; verify target availability.", outcomes, nil, successCount, conflictCount, errorCount
	}

	if !jitterOK {
		return config.VerdictInconclusive, fmt.Sprintf("No invariant violation observed, but dispatch jitter (%.2f ms) exceeded threshold (%.1f ms).", jitterMs, maxJitter), outcomes, nil, successCount, conflictCount, errorCount
	}

	summary := fmt.Sprintf("All invariants passed (%d/%d returned 2xx, %d safely rejected).", successCount, len(streams), conflictCount)
	return config.VerdictPass, summary, outcomes, nil, successCount, conflictCount, errorCount
}

func extractJSONField(body, field string) (string, bool) {
	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return "", false
	}
	cur := any(data)
	for _, part := range strings.Split(field, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		v, exists := m[part]
		if !exists {
			return "", false
		}
		cur = v
	}
	switch v := cur.(type) {
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), true
		}
		return fmt.Sprintf("%g", v), true
	case string:
		return v, true
	case bool:
		return fmt.Sprintf("%v", v), true
	default:
		b, _ := json.Marshal(v)
		return string(b), true
	}
}

func buildRemediations(mode config.TestMode) []string {
	if mode == config.ModeIdempotency {
		return []string{
			"Persist Idempotency-Key records atomically in a unique-indexed table or Redis lock before executing state transitions.",
			"Return the cached original response for duplicate completed keys and 409 Conflict for in-flight duplicate keys.",
		}
	}
	return []string{
		"Use database row-level locking (SELECT ... FOR UPDATE) or an atomic conditional update (UPDATE vouchers SET used_count = used_count + 1 WHERE code = $1 AND used_count < max_uses).",
		"Enforce a unique constraint or version column (optimistic concurrency control) to guarantee single-use state transitions.",
		"Require and validate an Idempotency-Key header on state-changing financial or redemption endpoints.",
	}
}
