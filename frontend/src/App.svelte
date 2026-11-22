<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ClearHistory,
    ExportScenarioYAML,
    GetSafetyPolicy,
    ListExampleScenarios,
    ListHistory,
    RunScenario,
  } from '../wailsjs/go/main/App';
  import { config, guard } from '../wailsjs/go/models';
  import { EventsOn } from '../wailsjs/runtime/runtime';

  let policy: guard.Policy | null = null;
  let scenarios: config.Scenario[] = [];
  let history: config.RunResult[] = [];

  let activeScenario: config.Scenario = new config.Scenario({
    id: 'voucher-redeem-vulnerable',
    name: 'Voucher Redeem (Unprotected)',
    mode: 'concurrency',
    streams: 10,
    timeout_ms: 5000,
    warmup: true,
    target: {
      method: 'POST',
      url: 'http://127.0.0.1:8090/api/v1/vulnerable/redeem',
      body: '{"voucher_code":"PROMO2026"}',
      idempotency_header: 'Idempotency-Key',
    },
    pre_hook: { method: 'POST', url: 'http://127.0.0.1:8090/api/v1/reset' },
    post_check: { method: 'GET', url: 'http://127.0.0.1:8090/api/v1/state' },
    invariants: {
      max_success_count: 1,
      require_identical_idempotent_body: false,
      post_check_field: 'used_count',
      post_check_expected: '1',
      max_acceptable_jitter_ms: 50,
    },
  });

  let activeTab: 'body' | 'hooks' | 'invariants' = 'body';
  let result: config.RunResult | null = null;
  let selectedStreamA = 1;
  let selectedStreamB = 2;

  let running = false;
  let progressText = '';
  let errorMsg = '';

  let yamlModalOpen = false;
  let exportedYaml = '';

  onMount(async () => {
    try {
      policy = await GetSafetyPolicy();
      scenarios = await ListExampleScenarios();
      if (scenarios.length > 0) {
        selectScenario(scenarios[0]);
      }
      history = (await ListHistory()) || [];
    } catch (e) {
      errorMsg = String(e);
    }

    EventsOn('run:progress', (ev: { completed: number; total: number }) => {
      progressText = `Executing streams ${ev.completed}/${ev.total}...`;
    });
  });

  function selectScenario(s: config.Scenario) {
    activeScenario = config.Scenario.createFrom(JSON.parse(JSON.stringify(s)));
    if (!activeScenario.pre_hook) {
      activeScenario.pre_hook = config.HookConfig.createFrom({ method: 'POST', url: '' });
    }
    if (!activeScenario.post_check) {
      activeScenario.post_check = config.HookConfig.createFrom({ method: 'GET', url: '' });
    }
    errorMsg = '';
  }

  function selectHistory(r: config.RunResult) {
    result = r;
    selectedStreamA = r.streams?.[0]?.stream_id ?? 1;
    selectedStreamB = r.streams?.[1]?.stream_id ?? selectedStreamA;
  }

  async function handleRun() {
    running = true;
    errorMsg = '';
    progressText = 'Running Safety Guard & dispatching streams...';
    try {
      const res = await RunScenario(activeScenario);
      result = res;
      selectedStreamA = res.streams?.[0]?.stream_id ?? 1;
      selectedStreamB = res.streams?.[1]?.stream_id ?? selectedStreamA;
      history = (await ListHistory()) || [];
    } catch (e) {
      errorMsg = String(e);
    } finally {
      running = false;
      progressText = '';
    }
  }

  async function handleExportYAML() {
    try {
      exportedYaml = await ExportScenarioYAML(activeScenario);
      yamlModalOpen = true;
    } catch (e) {
      errorMsg = String(e);
    }
  }

  async function handleClearHistory() {
    await ClearHistory();
    history = [];
  }

  function barClass(st: config.StreamResult): string {
    if (st.error || st.status_code >= 500) return 'bar-err';
    if (st.status_code >= 200 && st.status_code < 300) return 'bar-ok';
    return 'bar-conflict';
  }

  function barStyle(st: config.StreamResult, totalMs: number): string {
    const denom = Math.max(totalMs || 1, 1);
    const left = Math.min(95, (st.start_offset_ms / denom) * 100);
    const width = Math.max(3, Math.min(100 - left, (st.duration_ms / denom) * 100));
    return `left:${left.toFixed(1)}%;width:${width.toFixed(1)}%;`;
  }

  $: streamA = result?.streams?.find((s) => s.stream_id === selectedStreamA) ?? null;
  $: streamB = result?.streams?.find((s) => s.stream_id === selectedStreamB) ?? null;
</script>

<div class="shell">
  <header class="topbar">
    <div class="brand">
      <span class="brand-dot"></span>
      <span>RaceGuard Studio</span>
      {#if policy}
        <span class="guard-pill" title="Enforced in Go backend">
          Safety Guard: {policy.allowed_hosts.join(', ')} (max {policy.max_streams} streams)
        </span>
      {/if}
    </div>
    <div class="top-actions">
      <button class="btn" on:click={handleExportYAML}>Export YAML</button>
      <button class="btn btn-primary" disabled={running} on:click={handleRun}>
        {running ? 'Running...' : 'Run Scenario'}
      </button>
    </div>
  </header>

  <main class="workspace">
    <!-- Left Pane: Scenarios & History -->
    <aside class="pane">
      <div class="pane-section">
        <h2 class="section-title">Example Scenarios</h2>
        {#each scenarios as s}
          <button
            class="list-item"
            class:active={activeScenario.id === s.id}
            on:click={() => selectScenario(s)}
          >
            <span class="item-title">{s.name}</span>
            <span class="item-meta">
              <span>{s.mode}</span>
              <span>{s.streams} streams</span>
            </span>
          </button>
        {/each}
      </div>

      <div class="pane-scroll">
        <div class="section-title">
          <span>Run History</span>
          {#if history.length > 0}
            <button class="tab" on:click={handleClearHistory}>Clear</button>
          {/if}
        </div>
        {#if history.length === 0}
          <p style="color: var(--muted); font-size: 12px;">
            No runs yet. Start local testlab (<code>go run ./testlab/api</code>) and click Run.
          </p>
        {:else}
          {#each history as h}
            <button
              class="list-item"
              class:active={result?.run_id === h.run_id}
              on:click={() => selectHistory(h)}
            >
              <span class="item-title">{h.scenario_name || h.scenario_id}</span>
              <span class="item-meta">
                <span class="badge badge-{h.verdict}">{h.verdict}</span>
                <span>2xx:{h.success_count} | {h.jitter_spread_ms}ms</span>
              </span>
            </button>
          {/each}
        {/if}
      </div>
    </aside>

    <!-- Center Pane: Scenario Editor + Waterfall Timeline -->
    <section class="pane">
      <div class="pane-section">
        <h2 class="section-title">Target & Concurrency Setup</h2>
        {#if errorMsg}
          <div class="error-banner">{errorMsg}</div>
        {/if}

        <div class="form-row">
          <div style="width: 95px;">
            <select bind:value={activeScenario.target.method}>
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
              <option value="PATCH">PATCH</option>
              <option value="DELETE">DELETE</option>
              <option value="GET">GET</option>
            </select>
          </div>
          <input
            class="input mono"
            bind:value={activeScenario.target.url}
            placeholder="http://127.0.0.1:8090/api/v1/safe/redeem"
          />
        </div>

        <div class="form-row">
          <div class="field">
            <label for="mode">Mode</label>
            <select id="mode" bind:value={activeScenario.mode}>
              <option value="concurrency">Concurrency (Race Check)</option>
              <option value="idempotency">Idempotency-Key Check</option>
            </select>
          </div>
          <div class="field" style="max-width: 110px;">
            <label for="streams">Streams</label>
            <input
              id="streams"
              class="input"
              type="number"
              min="1"
              max={policy?.max_streams ?? 30}
              bind:value={activeScenario.streams}
            />
          </div>
          <div class="field" style="max-width: 120px;">
            <label for="timeout">Timeout (ms)</label>
            <input
              id="timeout"
              class="input"
              type="number"
              min="100"
              max={policy?.max_timeout_ms ?? 15000}
              bind:value={activeScenario.timeout_ms}
            />
          </div>
        </div>

        <div class="tabs">
          <button class="tab" class:active={activeTab === 'body'} on:click={() => (activeTab = 'body')}>
            Request Body
          </button>
          <button class="tab" class:active={activeTab === 'hooks'} on:click={() => (activeTab = 'hooks')}>
            Pre / Post Hooks
          </button>
          <button class="tab" class:active={activeTab === 'invariants'} on:click={() => (activeTab = 'invariants')}>
            Invariants
          </button>
        </div>

        {#if activeTab === 'body'}
          <textarea
            class="mono"
            rows="3"
            bind:value={activeScenario.target.body}
            placeholder={'{"voucher_code":"PROMO2026"}'}
          ></textarea>
        {:else if activeTab === 'hooks'}
          <div class="form-row">
            {#if activeScenario.pre_hook}
              <div class="field">
                <label for="prehook">Pre-Hook URL (e.g. reset staging state)</label>
                <input id="prehook" class="input mono" bind:value={activeScenario.pre_hook.url} />
              </div>
            {/if}
            {#if activeScenario.post_check}
              <div class="field">
                <label for="postcheck">Post-Check URL (verify final state)</label>
                <input id="postcheck" class="input mono" bind:value={activeScenario.post_check.url} />
              </div>
            {/if}
          </div>
        {:else}
          <div class="form-row">
            <div class="field">
              <label for="maxsucc">Max 2xx Allowed</label>
              <input id="maxsucc" class="input" type="number" min="1" bind:value={activeScenario.invariants.max_success_count} />
            </div>
            <div class="field">
              <label for="pcfield">Post-Check JSON Field</label>
              <input id="pcfield" class="input mono" bind:value={activeScenario.invariants.post_check_field} />
            </div>
            <div class="field">
              <label for="pcexp">Expected Value</label>
              <input id="pcexp" class="input mono" bind:value={activeScenario.invariants.post_check_expected} />
            </div>
          </div>
        {/if}
      </div>

      <!-- Waterfall Timeline -->
      <div class="pane-scroll">
        <div class="section-title">
          <span>Waterfall Timeline (HTTP/2 Multiplexed Streams)</span>
          {#if progressText}
            <span style="color: var(--accent);">{progressText}</span>
          {:else if result}
            <span>Total: {result.total_duration_ms} ms | Jitter: {result.jitter_spread_ms} ms</span>
          {/if}
        </div>

        {#if !result}
          <p style="color: var(--muted); font-size: 12px;">
            Click <strong>Run Scenario</strong> to visualize concurrent stream execution timing and response statuses.
          </p>
        {:else}
          {#each result.streams as st}
            <div
              class="waterfall-row"
              class:selected={selectedStreamA === st.stream_id || selectedStreamB === st.stream_id}
              role="button"
              tabindex="0"
              on:click={() => {
                selectedStreamB = selectedStreamA;
                selectedStreamA = st.stream_id;
              }}
              on:keydown={(e) => {
                if (e.key === 'Enter') {
                  selectedStreamB = selectedStreamA;
                  selectedStreamA = st.stream_id;
                }
              }}
            >
              <span class="mono" style="color: var(--muted);">#{String(st.stream_id).padStart(2, '0')}</span>
              <span class="mono" style="font-weight: 700;">{st.error ? 'ERR' : st.status_code}</span>
              <div class="track" title="Offset +{st.start_offset_ms}ms, Duration {st.duration_ms}ms">
                <div class="bar {barClass(st)}" style={barStyle(st, result.total_duration_ms)}></div>
              </div>
              <span class="mono" style="text-align: right; color: var(--muted);">{st.duration_ms}ms</span>
            </div>
          {/each}
        {/if}
      </div>
    </section>

    <!-- Right Pane: Verdict, Invariants, Diff, Remediation -->
    <aside class="pane">
      <div class="pane-section">
        <div class="section-title">
          <span>Verdict & Metrics</span>
          {#if result}
            <span class="badge badge-{result.verdict}">{result.verdict}</span>
          {/if}
        </div>

        {#if !result}
          <p style="color: var(--muted); font-size: 12px; margin: 0;">Awaiting test run...</p>
        {:else}
          <p style="margin: 0 0 6px 0;">{result.summary}</p>
          <div class="metric-grid">
            <div class="metric-card">
              <div class="metric-label">2xx OK</div>
              <div class="metric-val" style="color: {result.verdict === 'FAIL' ? 'var(--fail)' : 'var(--pass)'};">
                {result.success_count}
              </div>
            </div>
            <div class="metric-card">
              <div class="metric-label">Rejected</div>
              <div class="metric-val" style="color: var(--accent);">{result.conflict_count}</div>
            </div>
            <div class="metric-card">
              <div class="metric-label">Jitter</div>
              <div class="metric-val">{result.jitter_spread_ms}ms</div>
            </div>
          </div>
        {/if}
      </div>

      <div class="pane-scroll">
        {#if result}
          <h3 class="section-title">Invariants</h3>
          {#each result.invariants as inv}
            <div class="metric-card" style="margin-bottom: 6px;">
              <div style="display:flex; justify-content: space-between; align-items:center;">
                <span class="mono" style="font-weight:600;">{inv.rule}</span>
                <span class="badge badge-{inv.passed ? 'PASS' : 'FAIL'}">{inv.passed ? 'PASS' : 'FAIL'}</span>
              </div>
              <div style="font-size:11px; color:var(--muted); margin-top:4px;">
                Exp: {inv.expected} | Act: {inv.actual}
              </div>
            </div>
          {/each}

          <h3 class="section-title" style="margin-top: 12px;">Stream Response Compare</h3>
          {#if streamA}
            <div style="font-size:11px; color:var(--muted); margin-bottom:4px;">
              Stream #{streamA.stream_id} ({streamA.status_code} {streamA.proto})
            </div>
            <pre class="code-box mono">{streamA.error || streamA.body_preview || '<empty>'}</pre>
          {/if}
          {#if streamB && streamB.stream_id !== streamA?.stream_id}
            <div style="font-size:11px; color:var(--muted); margin:8px 0 4px 0;">
              Stream #{streamB.stream_id} ({streamB.status_code} {streamB.proto})
            </div>
            <pre class="code-box mono">{streamB.error || streamB.body_preview || '<empty>'}</pre>
          {/if}

          {#if result.post_check}
            <h3 class="section-title" style="margin-top: 12px;">Post-Check State</h3>
            <pre class="code-box mono">{result.post_check.error || result.post_check.body}</pre>
          {/if}

          {#if result.remediations && result.remediations.length > 0}
            <h3 class="section-title" style="margin-top: 12px;">Remediation Hints</h3>
            <ul style="padding-left: 16px; margin: 0; color: #cbd5e1; line-height: 1.5;">
              {#each result.remediations as hint}
                <li style="margin-bottom: 6px;">{hint}</li>
              {/each}
            </ul>
          {/if}
        {/if}
      </div>
    </aside>
  </main>

  {#if yamlModalOpen}
    <div class="modal-backdrop" role="presentation" on:click={() => (yamlModalOpen = false)}>
      <div
        class="modal"
        role="dialog"
        aria-modal="true"
        tabindex="-1"
        on:click|stopPropagation
        on:keydown={(e) => e.key === 'Escape' && (yamlModalOpen = false)}
      >
        <div class="section-title">
          <span>Redacted CI/CD Scenario YAML</span>
          <button class="btn" on:click={() => (yamlModalOpen = false)}>Close</button>
        </div>
        <pre class="code-box mono" style="max-height: 360px;">{exportedYaml}</pre>
      </div>
    </div>
  {/if}
</div>
