package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"raceguard/internal/config"
)

const maxBodyBytes = 64 * 1024 // 64 KB cap per response for safety

// ProgressEvent is emitted as each concurrent stream completes.
type ProgressEvent struct {
	Completed int                 `json:"completed"`
	Total     int                 `json:"total"`
	Stream    config.StreamResult `json:"stream"`
}

// Engine executes synchronized concurrent HTTP/2-capable QA requests.
type Engine struct {
	client *http.Client
}

// NewEngine builds an HTTP client configured with HTTP/2 multiplexing support.
func NewEngine(timeout time.Duration) *Engine {
	tr := &http.Transport{
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 64,
		MaxConnsPerHost:     64,
		IdleConnTimeout:     30 * time.Second,
	}
	return &Engine{
		client: &http.Client{
			Transport: tr,
			Timeout:   timeout,
		},
	}
}

// ExecuteHook runs a single pre-hook or post-check request.
func (e *Engine) ExecuteHook(ctx context.Context, hook config.HookConfig) (int, string, error) {
	method := hook.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, hook.URL, strings.NewReader(hook.Body))
	if err != nil {
		return 0, "", err
	}
	for k, v := range hook.Headers {
		req.Header.Set(k, v)
	}
	if hook.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, string(body), nil
}

// ExecuteBatch dispatches N concurrent requests behind a barrier gate and collects timing telemetry.
func (e *Engine) ExecuteBatch(
	ctx context.Context,
	s config.Scenario,
	onProgress func(ProgressEvent),
) ([]config.StreamResult, float64, float64) {
	if s.Warmup {
		// Lightweight HEAD/OPTIONS warm-up so connection pool & TLS are primed before the barrier.
		if warmReq, err := http.NewRequestWithContext(ctx, http.MethodHead, s.Target.URL, nil); err == nil {
			if resp, err := e.client.Do(warmReq); err == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
				resp.Body.Close()
			}
		}
	}

	sharedIdempKey := strings.TrimSpace(s.Target.IdempotencyKey)
	if s.Mode == config.ModeIdempotency && sharedIdempKey == "" {
		sharedIdempKey = "rg-idemp-" + uuid.NewString()
	}

	var (
		readyWG   sync.WaitGroup
		doneWG    sync.WaitGroup
		startGate = make(chan struct{})
		mu        sync.Mutex
		results   = make([]config.StreamResult, 0, s.Streams)
		completed int
		batchT0   time.Time
	)

	readyWG.Add(s.Streams)
	doneWG.Add(s.Streams)

	for i := 1; i <= s.Streams; i++ {
		streamID := i
		go func() {
			defer doneWG.Done()

			req, err := http.NewRequestWithContext(ctx, s.Target.Method, s.Target.URL, bytes.NewReader([]byte(s.Target.Body)))
			if err != nil {
				readyWG.Done()
				<-startGate
				res := config.StreamResult{StreamID: streamID, Error: err.Error()}
				mu.Lock()
				results = append(results, res)
				completed++
				if onProgress != nil {
					onProgress(ProgressEvent{Completed: completed, Total: s.Streams, Stream: res})
				}
				mu.Unlock()
				return
			}

			for k, v := range s.Target.Headers {
				req.Header.Set(k, v)
			}
			if s.Target.Body != "" && req.Header.Get("Content-Type") == "" {
				req.Header.Set("Content-Type", "application/json")
			}
			if s.Mode == config.ModeIdempotency {
				req.Header.Set(s.Target.IdempotencyHdr, sharedIdempKey)
			}

			// Signal ready and wait for simultaneous barrier release
			readyWG.Done()
			<-startGate

			started := time.Now()
			offsetMs := float64(started.Sub(batchT0).Microseconds()) / 1000.0
			resp, err := e.client.Do(req)
			durMs := float64(time.Since(started).Microseconds()) / 1000.0

			res := config.StreamResult{
				StreamID:       streamID,
				StartOffsetMs:  round2(offsetMs),
				DurationMs:     round2(durMs),
				IdempotencyKey: sharedIdempKey,
			}

			if err != nil {
				res.Error = err.Error()
			} else {
				bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
				resp.Body.Close()
				res.StatusCode = resp.StatusCode
				res.Proto = resp.Proto
				res.Headers = flattenHeaders(resp.Header)
				if readErr != nil {
					res.Error = fmt.Sprintf("read body: %v", readErr)
				} else {
					res.BodyPreview = strings.TrimSpace(string(bodyBytes))
				}
			}

			mu.Lock()
			results = append(results, res)
			completed++
			c := completed
			mu.Unlock()

			if onProgress != nil {
				onProgress(ProgressEvent{Completed: c, Total: s.Streams, Stream: res})
			}
		}()
	}

	readyWG.Wait()
	batchT0 = time.Now()
	close(startGate)
	doneWG.Wait()

	totalMs := round2(float64(time.Since(batchT0).Microseconds()) / 1000.0)
	sort.Slice(results, func(i, j int) bool {
		return results[i].StreamID < results[j].StreamID
	})

	jitterMs := computeStartJitter(results)
	return results, totalMs, jitterMs
}

func computeStartJitter(results []config.StreamResult) float64 {
	if len(results) < 2 {
		return 0
	}
	minOff := results[0].StartOffsetMs
	maxOff := results[0].StartOffsetMs
	for _, r := range results[1:] {
		if r.StartOffsetMs < minOff {
			minOff = r.StartOffsetMs
		}
		if r.StartOffsetMs > maxOff {
			maxOff = r.StartOffsetMs
		}
	}
	return round2(maxOff - minOff)
}

func flattenHeaders(h http.Header) map[string]string {
	raw := make(map[string]string, len(h))
	for k, vals := range h {
		raw[k] = strings.Join(vals, ", ")
	}
	return config.RedactHeaders(raw)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
