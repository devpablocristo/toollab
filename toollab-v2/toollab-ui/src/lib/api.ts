const BASE = import.meta.env.VITE_DASHBOARD_URL || "http://localhost:8090";

async function request<T>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...opts,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export interface Run {
  id: string;
  status: "queued" | "running" | "succeeded" | "failed";
  source_type: string;
  source_ref: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  error_message?: string;
}

export interface Stats {
  total_runs: number;
  queued: number;
  running: number;
  succeeded: number;
  failed: number;
}

export interface EvidenceRef {
  file: string;
  line_start: number;
  line_end: number;
  symbol?: string;
}

export interface Endpoint {
  id: string;
  method: string;
  path: string;
  handler_pkg: string;
  handler_name: string;
  handler_receiver?: string;
  middleware_chain: string[];
  evidence: EvidenceRef;
}

export interface ServiceModel {
  model_version: string;
  snapshot_id: string;
  hash_tree: string;
  service_name: string;
  language_detected: string;
  framework_detected: string;
  endpoints: Endpoint[];
  types: Array<{ name: string; kind: string; fields: Array<{ name: string; type: string; json_tag?: string; validate?: string; binding?: string; is_required: boolean }>; evidence: EvidenceRef }>;
  dependencies: Array<{ name: string; type: string; scope: string; evidence: EvidenceRef }>;
  flows: Array<{ id: string; endpoint_id: string; steps: Array<{ order: number; from: string; to: string; kind: string; symbol?: string }>; evidence: EvidenceRef }>;
  domain_groups: Array<{ name: string; endpoint_ids: string[] }>;
  fingerprint: string;
}

export interface Summary {
  service_name: string;
  endpoint_count: number;
  dependency_count: number;
  type_count: number;
  complex_endpoints: Array<{ endpoint_id: string; reason: string }>;
  top_dependencies: string[];
}

export interface AuditFinding {
  id: string;
  severity: string;
  category: string;
  title: string;
  description: string;
  recommendation: string;
  endpoint_id?: string;
  evidence: EvidenceRef;
}

export interface AuditReport {
  model_fingerprint: string;
  generated_at: string;
  findings: AuditFinding[];
}

export interface Scenario {
  id: string;
  endpoint_id: string;
  method: string;
  path: string;
  payload_template?: unknown;
  expected_status: number;
  risk_category: string;
  notes: string;
}

export interface LLMInterpretation {
  model_fingerprint: string;
  provider: string;
  model: string;
  functional_summary: string;
  domain_groups: string[];
  risk_hypotheses: string[];
  suggested_test_scenarios: string[];
  raw?: string;
}

export interface RunArtifacts {
  snapshot?: {
    snapshot_id: string;
    source_type: string;
    repo_name: string;
    hash_tree: string;
    language_detected: string;
    framework_detected: string;
    created_at: string;
    files: Array<{ path: string; size: number; sha256: string }>;
  };
  model?: ServiceModel;
  summary?: Summary;
  audit?: AuditReport;
}

export const api = {
  createRun: (data: { source_type: "local_path"; local_path: string; llm_enabled: boolean }) =>
    request<{ run_id: string; status: Run["status"] }>("/v1/runs", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  listRuns: () => request<{ items: Run[] }>("/v1/runs"),
  getRun: (id: string) => request<Run>(`/v1/runs/${id}`),
  deleteRun: (id: string) => request<void>(`/v1/runs/${id}`, { method: "DELETE" }),
  getRunModel: (id: string) => request<ServiceModel>(`/v1/runs/${id}/model`),
  getRunSummary: (id: string) => request<Summary>(`/v1/runs/${id}/summary`),
  getRunAudit: (id: string) => request<AuditReport>(`/v1/runs/${id}/audit`),
  getRunScenarios: (id: string) => request<{ items: Scenario[] }>(`/v1/runs/${id}/scenarios`),
  getRunLLM: (id: string) => request<LLMInterpretation>(`/v1/runs/${id}/llm`),
  getRunArtifacts: (id: string) => request<RunArtifacts>(`/v1/runs/${id}/artifacts`),
  getRunLogs: (id: string) => request<{ items: string[] }>(`/v1/runs/${id}/logs`),
  getStats: async (): Promise<Stats> => {
    const data = await request<{ items: Run[] }>("/v1/runs");
    const stats: Stats = { total_runs: data.items.length, queued: 0, running: 0, succeeded: 0, failed: 0 };
    for (const run of data.items) {
      if (run.status === "queued") stats.queued++;
      if (run.status === "running") stats.running++;
      if (run.status === "succeeded") stats.succeeded++;
      if (run.status === "failed") stats.failed++;
    }
    return stats;
  },
};
