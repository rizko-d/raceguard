package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"raceguard/internal/config"
)

func TestExecuteBatchH2C(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	srv.Config.Protocols = &protocols
	srv.Start()
	defer srv.Close()

	eng := NewEngine(3*time.Second, true)
	scenario := config.Scenario{
		Mode:    config.ModeConcurrency,
		Streams: 4,
		Warmup:  true,
		UseH2C:  true,
		Target: config.TargetConfig{
			Method: http.MethodPost,
			URL:    srv.URL,
			Body:   `{"test":1}`,
		},
	}

	results, _, _ := eng.ExecuteBatch(context.Background(), scenario, nil)
	if len(results) != 4 {
		t.Fatalf("expected 4 stream results, got %d", len(results))
	}
	for _, r := range results {
		if r.Error != "" {
			t.Fatalf("unexpected stream error: %s", r.Error)
		}
		if r.Proto != "HTTP/2.0" {
			t.Fatalf("expected HTTP/2.0 via h2c, got %q", r.Proto)
		}
	}
}
