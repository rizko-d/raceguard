package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"proxy-authorization": true,
}

// LoadScenario reads a YAML scenario file and expands environment variables (e.g., ${STAGING_TOKEN}).
func LoadScenario(path string) (Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, fmt.Errorf("read scenario %s: %w", path, err)
	}

	expanded := os.ExpandEnv(string(raw))
	var s Scenario
	if err := yaml.Unmarshal([]byte(expanded), &s); err != nil {
		return Scenario{}, fmt.Errorf("parse scenario yaml: %w", err)
	}

	ApplyDefaults(&s)
	return s, nil
}

// MarshalYAML serializes a scenario to YAML after redacting sensitive header values.
func MarshalYAML(s Scenario) (string, error) {
	safe := s
	safe.Target.Headers = RedactHeaders(s.Target.Headers)
	if s.PreHook != nil {
		h := *s.PreHook
		h.Headers = RedactHeaders(h.Headers)
		safe.PreHook = &h
	}
	if s.PostCheck != nil {
		h := *s.PostCheck
		h.Headers = RedactHeaders(h.Headers)
		safe.PostCheck = &h
	}

	out, err := yaml.Marshal(&safe)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ApplyDefaults fills sensible defaults for omitted scenario fields.
func ApplyDefaults(s *Scenario) {
	if s.Mode == "" {
		s.Mode = ModeConcurrency
	}
	if s.Streams <= 0 {
		s.Streams = 10
	}
	if s.TimeoutMs <= 0 {
		s.TimeoutMs = 5000
	}
	if s.Target.Method == "" {
		s.Target.Method = "POST"
	}
	if s.Target.IdempotencyHdr == "" {
		s.Target.IdempotencyHdr = "Idempotency-Key"
	}
	if s.Invariants.MaxSuccessCount <= 0 {
		s.Invariants.MaxSuccessCount = 1
	}
	if s.Invariants.MaxAcceptableJitterMs <= 0 {
		s.Invariants.MaxAcceptableJitterMs = 50.0
	}
}

// RedactHeaders returns a copy of headers with credential values masked.
func RedactHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		if sensitiveHeaders[strings.ToLower(strings.TrimSpace(k))] {
			out[k] = "[REDACTED]"
		} else {
			out[k] = v
		}
	}
	return out
}
