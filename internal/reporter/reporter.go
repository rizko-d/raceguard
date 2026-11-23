package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"

	"raceguard/internal/config"
)

// FormatCLI returns a concise human-readable summary for terminal / CI logs.
func FormatCLI(r config.RunResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== RaceGuard Result [%s] ===\n", r.Verdict)
	fmt.Fprintf(&b, "Scenario : %s (%s)\n", r.ScenarioName, r.Mode)
	fmt.Fprintf(&b, "Target   : %s %s\n", r.Method, r.TargetURL)
	fmt.Fprintf(&b, "Summary  : %s\n", r.Summary)
	fmt.Fprintf(&b, "Metrics  : 2xx=%d | Conflict/Reject=%d | Errors=%d | Total=%.2fms | Jitter=%.2fms\n\n",
		r.SuccessCount, r.ConflictCount, r.ErrorCount, r.TotalDurationMs, r.JitterSpreadMs)

	b.WriteString("Invariants:\n")
	for _, inv := range r.Invariants {
		mark := "PASS"
		if !inv.Passed {
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "  [%s] %-30s expected=%s | actual=%s\n", mark, inv.Rule, inv.Expected, inv.Actual)
	}

	b.WriteString("\nStreams:\n")
	for _, st := range r.Streams {
		if st.Error != "" {
			fmt.Fprintf(&b, "  #%02d ERR   (+%.2fms, %.2fms) %s\n", st.StreamID, st.StartOffsetMs, st.DurationMs, st.Error)
		} else {
			fmt.Fprintf(&b, "  #%02d %-3d   %-8s (+%.2fms, %.2fms) %s\n",
				st.StreamID, st.StatusCode, st.Proto, st.StartOffsetMs, st.DurationMs, truncate(st.BodyPreview, 80))
		}
	}

	if len(r.Remediations) > 0 {
		b.WriteString("\nRemediation Hints:\n")
		for _, hint := range r.Remediations {
			fmt.Fprintf(&b, "  - %s\n", hint)
		}
	}
	return b.String()
}

// WriteJSON saves the structured RunResult as formatted JSON.
func WriteJSON(path string, r config.RunResult) error {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// WriteSARIF exports findings in SARIF 2.1.0 format for GitHub/GitLab CI Security tabs.
func WriteSARIF(path string, r config.RunResult) error {
	results := []map[string]any{}
	if r.Verdict == config.VerdictFail {
		results = append(results, map[string]any{
			"ruleId": "RG-001",
			"level":  "error",
			"message": map[string]any{
				"text": fmt.Sprintf("%s (%s %s) — %s", r.ScenarioName, r.Method, r.TargetURL, r.Summary),
			},
			"locations": []map[string]any{
				{
					"physicalLocation": map[string]any{
						"artifactLocation": map[string]any{
							"uri": r.ScenarioID + ".yaml",
						},
					},
				},
			},
		})
	}

	sarif := map[string]any{
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"version": "2.1.0",
		"runs": []map[string]any{
			{
				"tool": map[string]any{
					"driver": map[string]any{
						"name":           "RaceGuard",
						"informationUri": "https://github.com/rizko-d/raceguard",
						"version":        "0.2.0",
						"rules": []map[string]any{
							{
								"id":               "RG-001",
								"name":             "ConcurrencyOrIdempotencyInvariantViolation",
								"shortDescription": map[string]any{"text": "Concurrency or Idempotency invariant violated under parallel requests"},
							},
						},
					},
				},
				"results": results,
			},
		},
	}

	raw, err := json.MarshalIndent(sarif, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// WriteHTML writes a self-contained, auto-escaped static HTML report (no running server needed).
func WriteHTML(path string, r config.RunResult) error {
	tmpl, err := template.New("report").Parse(htmlReportTemplate)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, r); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

const htmlReportTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>RaceGuard Report - {{.ScenarioName}}</title>
<style>
  body { font-family: system-ui, -apple-system, sans-serif; background: #0f172a; color: #e2e8f0; margin: 24px; }
  .card { background: #1e293b; border: 1px solid #334155; border-radius: 10px; padding: 16px; margin-bottom: 16px; }
  .badge { display: inline-block; padding: 4px 10px; border-radius: 6px; font-weight: 700; font-size: 13px; }
  .PASS { background: #14532d; color: #86efac; }
  .FAIL { background: #7f1d1d; color: #fca5a5; }
  .INCONCLUSIVE { background: #78350f; color: #fde68a; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th, td { text-align: left; padding: 8px; border-bottom: 1px solid #334155; }
  code, pre { background: #020617; padding: 4px 6px; border-radius: 4px; font-family: ui-monospace, monospace; white-space: pre-wrap; word-break: break-all; }
</style>
</head>
<body>
  <div class="card">
    <h1>RaceGuard Report <span class="badge {{.Verdict}}">{{.Verdict}}</span></h1>
    <p><strong>Scenario:</strong> {{.ScenarioName}} ({{.Mode}})</p>
    <p><strong>Target:</strong> <code>{{.Method}} {{.TargetURL}}</code></p>
    <p><strong>Summary:</strong> {{.Summary}}</p>
    <p><strong>Metrics:</strong> 2xx={{.SuccessCount}} | Rejected={{.ConflictCount}} | Errors={{.ErrorCount}} | Total={{.TotalDurationMs}}ms | Jitter={{.JitterSpreadMs}}ms</p>
  </div>
  <div class="card">
    <h2>Invariants</h2>
    <table>
      <thead><tr><th>Rule</th><th>Expected</th><th>Actual</th><th>Status</th></tr></thead>
      <tbody>
        {{range .Invariants}}
        <tr>
          <td><code>{{.Rule}}</code></td>
          <td>{{.Expected}}</td>
          <td>{{.Actual}}</td>
          <td>{{if .Passed}}PASS{{else}}FAIL{{end}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </div>
  <div class="card">
    <h2>Concurrent Streams</h2>
    <table>
      <thead><tr><th>#</th><th>Status</th><th>Proto</th><th>Offset (ms)</th><th>Duration (ms)</th><th>Response Preview</th></tr></thead>
      <tbody>
        {{range .Streams}}
        <tr>
          <td>#{{.StreamID}}</td>
          <td>{{.StatusCode}}</td>
          <td>{{.Proto}}</td>
          <td>{{.StartOffsetMs}}</td>
          <td>{{.DurationMs}}</td>
          <td><code>{{if .Error}}{{.Error}}{{else}}{{.BodyPreview}}{{end}}</code></td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </div>
  {{if .Remediations}}
  <div class="card">
    <h2>Remediation Hints</h2>
    <ul>
      {{range .Remediations}}<li>{{.}}</li>{{end}}
    </ul>
  </div>
  {{end}}
</body>
</html>`
