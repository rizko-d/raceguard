package evaluator

import (
	"testing"

	"raceguard/internal/config"
)

func TestEvaluateConcurrencyFail(t *testing.T) {
	s := config.Scenario{
		Mode: config.ModeConcurrency,
		Invariants: config.InvariantConfig{
			MaxSuccessCount:       1,
			PostCheckField:        "used_count",
			PostCheckExpected:     "1",
			MaxAcceptableJitterMs: 50,
		},
	}

	streams := []config.StreamResult{
		{StreamID: 1, StatusCode: 200, BodyPreview: `{"tx_id":"1"}`},
		{StreamID: 2, StatusCode: 200, BodyPreview: `{"tx_id":"2"}`},
		{StreamID: 3, StatusCode: 409, BodyPreview: `{"error":"already used"}`},
	}
	post := &config.PostCheckResult{StatusCode: 200, Body: `{"used_count":2}`}

	verdict, _, outcomes, remediations, succ, conf, _ := Evaluate(s, streams, 1.5, post)
	if verdict != config.VerdictFail {
		t.Fatalf("expected FAIL verdict, got %s", verdict)
	}
	if succ != 2 || conf != 1 {
		t.Fatalf("unexpected counts: succ=%d conf=%d", succ, conf)
	}
	if len(remediations) == 0 {
		t.Fatal("expected remediation advice on FAIL")
	}
	if len(outcomes) < 2 || outcomes[0].Passed || outcomes[1].Passed {
		t.Fatalf("expected both invariants to fail, got %+v", outcomes)
	}
}

func TestEvaluateIdempotencyPass(t *testing.T) {
	s := config.Scenario{
		Mode: config.ModeIdempotency,
		Invariants: config.InvariantConfig{
			MaxSuccessCount:       1,
			RequireIdenticalIdemp: true,
			MaxAcceptableJitterMs: 50,
		},
	}
	streams := []config.StreamResult{
		{StreamID: 1, StatusCode: 200, BodyPreview: `{"tx_id":"same","status":"ok"}`},
		{StreamID: 2, StatusCode: 200, BodyPreview: `{"tx_id":"same","status":"ok"}`},
		{StreamID: 3, StatusCode: 409, BodyPreview: `{"error":"in_progress"}`},
	}

	verdict, _, _, _, _, _, _ := Evaluate(s, streams, 2.0, nil)
	if verdict != config.VerdictPass {
		t.Fatalf("expected PASS for consistent idempotent responses, got %s", verdict)
	}
}
