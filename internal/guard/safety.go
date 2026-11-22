package guard

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"raceguard/internal/config"
)

const (
	DefaultMaxStreams   = 30
	DefaultMaxTimeoutMs = 15000
)

// Policy defines the Safety Guard constraints enforced on all outbound test runs.
type Policy struct {
	AllowedHosts []string `json:"allowed_hosts"`
	MaxStreams   int      `json:"max_streams"`
	MaxTimeoutMs int      `json:"max_timeout_ms"`
}

// DefaultPolicy returns a safe-by-default policy allowing only local targets
// plus any explicitly approved staging hosts configured via RACEGUARD_ALLOWLIST.
func DefaultPolicy() Policy {
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	if extra := strings.TrimSpace(os.Getenv("RACEGUARD_ALLOWLIST")); extra != "" {
		for _, h := range strings.Split(extra, ",") {
			trimmed := strings.ToLower(strings.TrimSpace(h))
			if trimmed != "" {
				hosts = append(hosts, trimmed)
			}
		}
	}
	return Policy{
		AllowedHosts: hosts,
		MaxStreams:   DefaultMaxStreams,
		MaxTimeoutMs: DefaultMaxTimeoutMs,
	}
}

// ValidateScenario enforces all safety rules before any network request is made.
func (p Policy) ValidateScenario(s config.Scenario) error {
	if s.Streams < 1 || s.Streams > p.MaxStreams {
		return fmt.Errorf("safety guard: streams (%d) must be between 1 and %d", s.Streams, p.MaxStreams)
	}
	if s.TimeoutMs < 100 || s.TimeoutMs > p.MaxTimeoutMs {
		return fmt.Errorf("safety guard: timeout_ms (%d) must be between 100 and %d", s.TimeoutMs, p.MaxTimeoutMs)
	}
	if err := p.ValidateURL(s.Target.URL); err != nil {
		return fmt.Errorf("target url: %w", err)
	}
	if s.PreHook != nil && strings.TrimSpace(s.PreHook.URL) != "" {
		if err := p.ValidateURL(s.PreHook.URL); err != nil {
			return fmt.Errorf("pre_hook url: %w", err)
		}
	}
	if s.PostCheck != nil && strings.TrimSpace(s.PostCheck.URL) != "" {
		if err := p.ValidateURL(s.PostCheck.URL); err != nil {
			return fmt.Errorf("post_check url: %w", err)
		}
	}
	return nil
}

// ValidateURL verifies scheme, cloud-metadata blocklist, and strict host allowlist.
func (p Policy) ValidateURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("safety guard: empty URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("safety guard: invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("safety guard: unsupported scheme %q (only http/https allowed)", parsed.Scheme)
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return errors.New("safety guard: missing hostname")
	}

	if isBlockedMetadataHost(host) {
		return fmt.Errorf("safety guard: blocked cloud metadata or link-local address %q", host)
	}

	if !p.isAllowedHost(host) {
		return fmt.Errorf("safety guard: host %q is not in allowlist %v (set RACEGUARD_ALLOWLIST for approved staging hosts)", host, p.AllowedHosts)
	}

	return nil
}

func (p Policy) isAllowedHost(host string) bool {
	for _, allowed := range p.AllowedHosts {
		if strings.EqualFold(host, allowed) {
			return true
		}
	}
	return false
}

func isBlockedMetadataHost(host string) bool {
	if host == "metadata.google.internal" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	// Block IPv4/IPv6 link-local (includes 169.254.169.254 cloud metadata)
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
