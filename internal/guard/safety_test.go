package guard

import (
	"testing"

	"raceguard/internal/config"
)

func TestPolicyValidateScenario(t *testing.T) {
	policy := Policy{
		AllowedHosts: []string{"localhost", "127.0.0.1", "api.staging.local"},
		MaxStreams:   30,
		MaxTimeoutMs: 15000,
	}

	valid := config.Scenario{
		Streams:   10,
		TimeoutMs: 5000,
		Target:    config.TargetConfig{Method: "POST", URL: "http://127.0.0.1:8090/api/v1/safe/redeem"},
	}
	if err := policy.ValidateScenario(valid); err != nil {
		t.Fatalf("expected valid local scenario to pass, got: %v", err)
	}

	t.Run("blocks non-allowlisted external host", func(t *testing.T) {
		bad := valid
		bad.Target.URL = "https://example.com/api/redeem"
		if err := policy.ValidateScenario(bad); err == nil {
			t.Fatal("expected external host to be rejected")
		}
	})

	t.Run("blocks cloud metadata IP", func(t *testing.T) {
		metaPolicy := Policy{
			AllowedHosts: []string{"169.254.169.254"}, // even if misconfigured
			MaxStreams:   30,
			MaxTimeoutMs: 15000,
		}
		bad := valid
		bad.Target.URL = "http://169.254.169.254/latest/meta-data/"
		if err := metaPolicy.ValidateScenario(bad); err == nil {
			t.Fatal("expected cloud metadata IP to be rejected")
		}
	})

	t.Run("enforces max streams cap", func(t *testing.T) {
		bad := valid
		bad.Streams = 100
		if err := policy.ValidateScenario(bad); err == nil {
			t.Fatal("expected excessive streams to be rejected")
		}
	})
}
