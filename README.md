# RaceGuard Studio

**RaceGuard** adalah tool AppSec & QA berbasis **Go + Wails (Desktop) + CLI (CI/CD)** untuk memverifikasi ketahanan *concurrency* (*business-logic race condition*) dan konsistensi *idempotency* (`Idempotency-Key`) pada endpoint API staging/lokal sebelum rilis ke produksi.

## Fitur Utama
* **Dual Interface (1 Core Engine):**
  * **Desktop Studio (Wails + Svelte):** 3-pane workbench visual dengan *Waterfall Timeline*, perbandingan respons antar-stream, riwayat eksekusi lokal, dan *export* YAML untuk CI/CD — berkomunikasi lewat IPC WebView tanpa membuka port HTTP.
  * **Headless CLI (`cmd/cli`):** Siap dipakai di GitHub Actions / GitLab CI dengan exit code `0` (`PASS`) / `1` (`FAIL`), serta dukungan output **JSON**, **SARIF 2.1.0**, dan **Static HTML Report**.
* **Safety-by-Design (`internal/guard`):**
  * Secara default hanya mengizinkan target `localhost`, `127.0.0.1`, dan `::1`.
  * Host staging tambahan wajib didaftarkan eksplisit lewat environment variable `RACEGUARD_ALLOWLIST`.
  * Memblokir alamat *cloud metadata* (`169.254.169.254`, `metadata.google.internal`) serta membatasi jumlah *stream* (`max 30`) dan *timeout* (`max 15s`).
  * Auto-redaction pada header sensitif (`Authorization`, `Cookie`, `X-Api-Key`) saat menyimpan riwayat dan mengekspor YAML.
* **Local Testlab (`testlab/api`):**
  * API mini lokal (`127.0.0.1:8090`) berisi endpoint *unprotected* (`/api/v1/vulnerable/redeem`) vs *protected* (`/api/v1/safe/redeem`) untuk demo perbandingan langsung.

---

## Quickstart

### 1. Jalankan Local Testlab (Terminal 1)
```powershell
go run ./testlab/api
```

### 2. Jalankan Mode CLI (Terminal 2)
Uji endpoint yang belum memakai lock (akan menghasilkan `FAIL` + saran remediasi):
```powershell
go run ./cmd/cli -spec examples/voucher-redeem-vulnerable.yaml -html report.html -sarif report.sarif
```

Uji endpoint yang sudah memakai lock & `Idempotency-Key` (akan menghasilkan `PASS`):
```powershell
go run ./cmd/cli -spec examples/voucher-redeem-safe.yaml
go run ./cmd/cli -spec examples/idempotency-check.yaml
```

### 3. Jalankan Mode Desktop Studio (Wails)
Cara cepat di Windows (otomatis menjalankan `testlab/api` + `wails dev`):
```powershell
.\start-dev.bat
```

Atau manual (untuk Wails v2.17+, set `WAILS_DEV_ALLOWED_HOSTS` agar host `wails.localhost` diizinkan di mode dev):
```powershell
$env:WAILS_DEV_ALLOWED_HOSTS="wails.localhost,localhost,127.0.0.1"
wails dev
```
Untuk mem-build executable desktop produksi:
```powershell
wails build
```

---

## Struktur Proyek
```text
raceguard/
├── main.go                     # Entry point Wails Desktop
├── app.go                      # IPC bindings Go (least-privilege)
├── cmd/cli/main.go             # Entry point CLI untuk CI/CD
├── frontend/                   # UI Svelte + TypeScript (ter-embed ke binary)
├── internal/
│   ├── config/                 # Schema YAML, env expansion, header redaction
│   ├── guard/                  # Safety Guard (allowlist, metadata block, caps)
│   ├── transport/              # HTTP/2 multiplexed concurrent test engine
│   ├── runner/                 # Lifecycle orchestrator (pre-hook -> batch -> post-check)
│   ├── evaluator/              # Business-logic invariant evaluator + remediation hints
│   ├── reporter/               # CLI, JSON, SARIF 2.1.0, & Static HTML report
│   └── storage/                # Local JSON run history (.raceguard/history.json)
├── examples/                   # Contoh skenario uji YAML
└── testlab/api/                # Mini API lokal untuk pengujian & demo
```
