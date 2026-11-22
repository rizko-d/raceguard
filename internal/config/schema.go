package config

// TestMode defines whether requests test raw concurrency or idempotency-key handling.
type TestMode string

const (
	ModeConcurrency TestMode = "concurrency"
	ModeIdempotency TestMode = "idempotency"
)

// Verdict represents the final outcome of a concurrency/idempotency evaluation.
type Verdict string

const (
	VerdictPass         Verdict = "PASS"
	VerdictFail         Verdict = "FAIL"
	VerdictInconclusive Verdict = "INCONCLUSIVE"
)

// Scenario defines a declarative concurrency or idempotency test specification.
type Scenario struct {
	ID          string          `yaml:"id" json:"id"`
	Name        string          `yaml:"name" json:"name"`
	Description string          `yaml:"description,omitempty" json:"description,omitempty"`
	Mode        TestMode        `yaml:"mode" json:"mode"`
	Streams     int             `yaml:"streams" json:"streams"`
	TimeoutMs   int             `yaml:"timeout_ms" json:"timeout_ms"`
	Warmup      bool            `yaml:"warmup" json:"warmup"`
	Target      TargetConfig    `yaml:"target" json:"target"`
	PreHook     *HookConfig     `yaml:"pre_hook,omitempty" json:"pre_hook,omitempty"`
	PostCheck   *HookConfig     `yaml:"post_check,omitempty" json:"post_check,omitempty"`
	Invariants  InvariantConfig `yaml:"invariants" json:"invariants"`
}

// TargetConfig describes the staging/local HTTP endpoint under test.
type TargetConfig struct {
	Method         string            `yaml:"method" json:"method"`
	URL            string            `yaml:"url" json:"url"`
	Headers        map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body           string            `yaml:"body,omitempty" json:"body,omitempty"`
	IdempotencyHdr string            `yaml:"idempotency_header,omitempty" json:"idempotency_header,omitempty"`
	IdempotencyKey string            `yaml:"idempotency_key,omitempty" json:"idempotency_key,omitempty"`
}

// HookConfig describes an optional pre-test setup or post-test verification request.
type HookConfig struct {
	Method  string            `yaml:"method" json:"method"`
	URL     string            `yaml:"url" json:"url"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty" json:"body,omitempty"`
}

// InvariantConfig defines the expected business-logic invariants.
type InvariantConfig struct {
	MaxSuccessCount          int    `yaml:"max_success_count" json:"max_success_count"`
	RequireIdenticalIdemp    bool   `yaml:"require_identical_idempotent_body" json:"require_identical_idempotent_body"`
	PostCheckField           string `yaml:"post_check_field,omitempty" json:"post_check_field,omitempty"`
	PostCheckExpected        string `yaml:"post_check_expected,omitempty" json:"post_check_expected,omitempty"`
	MaxAcceptableJitterMs    float64 `yaml:"max_acceptable_jitter_ms,omitempty" json:"max_acceptable_jitter_ms,omitempty"`
}

// StreamResult captures telemetry and response details for a single concurrent request.
type StreamResult struct {
	StreamID       int               `json:"stream_id"`
	StatusCode     int               `json:"status_code"`
	Proto          string            `json:"proto"`
	StartOffsetMs  float64           `json:"start_offset_ms"`
	DurationMs     float64           `json:"duration_ms"`
	BodyPreview    string            `json:"body_preview"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Error          string            `json:"error,omitempty"`
}

// InvariantOutcome captures the evaluation result of a single invariant rule.
type InvariantOutcome struct {
	Rule     string `json:"rule"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
}

// PostCheckResult stores the response from the optional post-check state endpoint.
type PostCheckResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
	Error      string `json:"error,omitempty"`
}

// RunResult is the complete, redacted result of a single scenario execution.
type RunResult struct {
	RunID           string             `json:"run_id"`
	ScenarioID      string             `json:"scenario_id"`
	ScenarioName    string             `json:"scenario_name"`
	Mode            TestMode           `json:"mode"`
	TargetURL       string             `json:"target_url"`
	Method          string             `json:"method"`
	StartedAt       string             `json:"started_at"`
	TotalDurationMs float64            `json:"total_duration_ms"`
	JitterSpreadMs  float64            `json:"jitter_spread_ms"`
	SuccessCount    int                `json:"success_count"`
	ConflictCount   int                `json:"conflict_count"`
	ErrorCount      int                `json:"error_count"`
	Verdict         Verdict            `json:"verdict"`
	Summary         string             `json:"summary"`
	Remediations    []string           `json:"remediations,omitempty"`
	Invariants      []InvariantOutcome `json:"invariants"`
	Streams         []StreamResult     `json:"streams"`
	PostCheck       *PostCheckResult   `json:"post_check,omitempty"`
}
