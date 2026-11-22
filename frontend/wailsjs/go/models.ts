export namespace config {
	
	export class HookConfig {
	    method: string;
	    url: string;
	    headers?: Record<string, string>;
	    body?: string;
	
	    static createFrom(source: any = {}) {
	        return new HookConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method = source["method"];
	        this.url = source["url"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	    }
	}
	export class InvariantConfig {
	    max_success_count: number;
	    require_identical_idempotent_body: boolean;
	    post_check_field?: string;
	    post_check_expected?: string;
	    max_acceptable_jitter_ms?: number;
	
	    static createFrom(source: any = {}) {
	        return new InvariantConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.max_success_count = source["max_success_count"];
	        this.require_identical_idempotent_body = source["require_identical_idempotent_body"];
	        this.post_check_field = source["post_check_field"];
	        this.post_check_expected = source["post_check_expected"];
	        this.max_acceptable_jitter_ms = source["max_acceptable_jitter_ms"];
	    }
	}
	export class InvariantOutcome {
	    rule: string;
	    expected: string;
	    actual: string;
	    passed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InvariantOutcome(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rule = source["rule"];
	        this.expected = source["expected"];
	        this.actual = source["actual"];
	        this.passed = source["passed"];
	    }
	}
	export class PostCheckResult {
	    url: string;
	    status_code: number;
	    body: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PostCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.status_code = source["status_code"];
	        this.body = source["body"];
	        this.error = source["error"];
	    }
	}
	export class StreamResult {
	    stream_id: number;
	    status_code: number;
	    proto: string;
	    start_offset_ms: number;
	    duration_ms: number;
	    body_preview: string;
	    idempotency_key?: string;
	    headers?: Record<string, string>;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new StreamResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stream_id = source["stream_id"];
	        this.status_code = source["status_code"];
	        this.proto = source["proto"];
	        this.start_offset_ms = source["start_offset_ms"];
	        this.duration_ms = source["duration_ms"];
	        this.body_preview = source["body_preview"];
	        this.idempotency_key = source["idempotency_key"];
	        this.headers = source["headers"];
	        this.error = source["error"];
	    }
	}
	export class RunResult {
	    run_id: string;
	    scenario_id: string;
	    scenario_name: string;
	    mode: string;
	    target_url: string;
	    method: string;
	    started_at: string;
	    total_duration_ms: number;
	    jitter_spread_ms: number;
	    success_count: number;
	    conflict_count: number;
	    error_count: number;
	    verdict: string;
	    summary: string;
	    remediations?: string[];
	    invariants: InvariantOutcome[];
	    streams: StreamResult[];
	    post_check?: PostCheckResult;
	
	    static createFrom(source: any = {}) {
	        return new RunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.run_id = source["run_id"];
	        this.scenario_id = source["scenario_id"];
	        this.scenario_name = source["scenario_name"];
	        this.mode = source["mode"];
	        this.target_url = source["target_url"];
	        this.method = source["method"];
	        this.started_at = source["started_at"];
	        this.total_duration_ms = source["total_duration_ms"];
	        this.jitter_spread_ms = source["jitter_spread_ms"];
	        this.success_count = source["success_count"];
	        this.conflict_count = source["conflict_count"];
	        this.error_count = source["error_count"];
	        this.verdict = source["verdict"];
	        this.summary = source["summary"];
	        this.remediations = source["remediations"];
	        this.invariants = this.convertValues(source["invariants"], InvariantOutcome);
	        this.streams = this.convertValues(source["streams"], StreamResult);
	        this.post_check = this.convertValues(source["post_check"], PostCheckResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TargetConfig {
	    method: string;
	    url: string;
	    headers?: Record<string, string>;
	    body?: string;
	    idempotency_header?: string;
	    idempotency_key?: string;
	
	    static createFrom(source: any = {}) {
	        return new TargetConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method = source["method"];
	        this.url = source["url"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.idempotency_header = source["idempotency_header"];
	        this.idempotency_key = source["idempotency_key"];
	    }
	}
	export class Scenario {
	    id: string;
	    name: string;
	    description?: string;
	    mode: string;
	    streams: number;
	    timeout_ms: number;
	    warmup: boolean;
	    target: TargetConfig;
	    pre_hook?: HookConfig;
	    post_check?: HookConfig;
	    invariants: InvariantConfig;
	
	    static createFrom(source: any = {}) {
	        return new Scenario(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.mode = source["mode"];
	        this.streams = source["streams"];
	        this.timeout_ms = source["timeout_ms"];
	        this.warmup = source["warmup"];
	        this.target = this.convertValues(source["target"], TargetConfig);
	        this.pre_hook = this.convertValues(source["pre_hook"], HookConfig);
	        this.post_check = this.convertValues(source["post_check"], HookConfig);
	        this.invariants = this.convertValues(source["invariants"], InvariantConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace guard {
	
	export class Policy {
	    allowed_hosts: string[];
	    max_streams: number;
	    max_timeout_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new Policy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allowed_hosts = source["allowed_hosts"];
	        this.max_streams = source["max_streams"];
	        this.max_timeout_ms = source["max_timeout_ms"];
	    }
	}

}

