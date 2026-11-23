# Security Policy

## Intended Use & Responsible Testing

**RaceGuard Studio** is a defensive Application Security (AppSec) and QA verification tool designed to help developers and security engineers test **their own local, staging, or explicitly authorized environments** for concurrency bugs (*business-logic race conditions*) and broken idempotency (`Idempotency-Key`) handling.

Do **not** use this tool against production systems or third-party targets without explicit written authorization.

---

## Built-in Security & Safety Controls

RaceGuard enforces several safety guardrails in the Go backend (`internal/guard` and `internal/config`):

1. **Strict Host Allowlist (Default Localhost-Only):**
   * By default, outbound requests are permitted **only** to `localhost`, `127.0.0.1`, and `::1`.
   * Testing approved staging hosts requires explicit opt-in via the `RACEGUARD_ALLOWLIST` environment variable.
2. **SSRF & Cloud Metadata Protection:**
   * Link-local addresses (including cloud metadata endpoints such as `169.254.169.254` and `metadata.google.internal`) and non-`http`/`https` schemes are blocked regardless of allowlist configuration.
3. **Concurrency & Resource Limits:**
   * Maximum parallel streams (`30`), request timeouts (`15s`), and per-response body reads (`64 KB`) are hard-capped in Go to prevent accidental self-DoS.
4. **Credential Redaction:**
   * Sensitive headers (`Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`, `Proxy-Authorization`) are redacted automatically when persisting run history or exporting YAML scenarios.
5. **No Open Local HTTP Port in Desktop Mode:**
   * The Wails desktop app communicates between Go and the UI strictly over internal OS WebView IPC bindings, avoiding localhost CSRF and DNS rebinding exposure.
   * All target HTTP response bodies are rendered strictly as escaped plain text (no raw HTML injection).

---

## Supported Versions

| Version | Supported |
| :--- | :---: |
| `0.2.x` (current) | Yes |
| `0.1.x` | Security fixes only |

---

## Reporting a Vulnerability

If you discover a security vulnerability in **RaceGuard Studio** itself (for example, a bypass in `internal/guard`, WebView XSS, or sensitive credential leakage):

1. **Do not** open a public GitHub issue with exploit details.
2. Report it privately using **GitHub Security Advisories** (*Security* tab → *Report a vulnerability*) or contact the repository maintainer directly.
3. Please include:
   * A clear description of the vulnerability and its impact.
   * Minimal steps to reproduce.
   * Suggested fix or mitigation (if available).

We aim to acknowledge reports within **72 hours** and release a fix as promptly as possible.
