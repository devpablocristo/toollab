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

export interface Target {
  id: string;
  name: string;
  base_url: string;
  description: string;
  created_at: string;
}

export interface Run {
  id: string;
  target_id: string;
  verdict: string;
  total_requests: number;
  completed_requests: number;
  success_rate: number;
  error_rate: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
  duration_s: number;
  concurrency: number;
  status_histogram: string;
  deterministic_fingerprint: string;
  started_at: string;
  finished_at: string;
  created_at: string;
}

export interface AssertionResult {
  rule_id: string;
  rule_type: string;
  passed: boolean;
  observed: string;
  expected: string;
  message: string;
}

export interface RunDetail {
  run: Run;
  assertions: AssertionResult[];
  interpretation: string | null;
}

export interface Stats {
  total_targets: number;
  total_runs: number;
  passed: number;
  failed: number;
}

export interface ExecResult {
  success: boolean;
  output: string;
  error?: string;
  elapsed: string;
}

export interface ScenarioFile {
  name: string;
  path: string;
  size: number;
}

export interface Interpretation {
  id: string;
  run_id: string;
  model: string;
  narrative: string;
  created_at: string;
}

export interface SecurityFinding {
  id: string;
  category: string;
  severity: string;
  title: string;
  description: string;
  endpoint?: string;
  remediation: string;
}

export interface SecurityAudit {
  score: number;
  grade: string;
  findings: SecurityFinding[];
  summary: { total: number; critical: number; high: number; medium: number; low: number; info: number };
}

export interface CoverageEndpoint {
  method: string;
  path: string;
  tested: boolean;
  hits: number;
  success: number;
  errors: number;
}

export interface CoverageReport {
  total_endpoints: number;
  tested_endpoints: number;
  coverage_rate: number;
  endpoints: CoverageEndpoint[];
  untested: CoverageEndpoint[];
  by_method: { method: string; total: number; tested: number; rate: number }[];
  status_codes: { status_code: number; observed: boolean }[];
}

export interface ContractViolation {
  endpoint: string;
  status_code: number;
  field: string;
  expected: string;
  actual: string;
  description: string;
}

export interface ContractReport {
  compliant: boolean;
  total_checks: number;
  total_violations: number;
  compliance_rate: number;
  violations: ContractViolation[];
}

export const api = {
  getStats: () => request<Stats>("/api/v1/stats"),

  listTargets: () => request<Target[]>("/api/v1/targets"),
  createTarget: (data: { name: string; base_url: string; description?: string }) =>
    request<Target>("/api/v1/targets", { method: "POST", body: JSON.stringify(data) }),
  deleteTarget: (id: string) =>
    request<void>(`/api/v1/targets/${id}`, { method: "DELETE" }),

  listRuns: (targetId?: string) =>
    request<Run[]>(`/api/v1/runs${targetId ? `?target_id=${targetId}` : ""}`),
  getRun: (id: string) => request<RunDetail>(`/api/v1/runs/${id}`),
  ingestRun: (data: { run_dir: string; target_id: string }) =>
    request<Run>("/api/v1/runs/ingest", { method: "POST", body: JSON.stringify(data) }),

  listScenarios: () => request<ScenarioFile[]>("/api/v1/scenarios"),

  execGenerate: (data: {
    from: string;
    target_base_url?: string;
    openapi_file?: string;
    openapi_url?: string;
    mode?: string;
  }) => request<ExecResult>("/api/v1/exec/generate", { method: "POST", body: JSON.stringify(data) }),

  execRun: (data: { scenario_path: string; target_id?: string }) =>
    request<{ exec: ExecResult; run?: Run; run_id?: string }>(
      "/api/v1/exec/run",
      { method: "POST", body: JSON.stringify(data) },
    ),

  execEnrich: (data: { scenario_path: string; from: string; target_base_url?: string }) =>
    request<ExecResult>("/api/v1/exec/enrich", { method: "POST", body: JSON.stringify(data) }),

  execInterpret: (data: { run_id: string }) =>
    request<{ exec: ExecResult; interpretation?: Interpretation }>(
      "/api/v1/exec/interpret",
      { method: "POST", body: JSON.stringify(data) },
    ),

  execAudit: (data: { run_id: string }) =>
    request<{ exec: ExecResult }>("/api/v1/exec/audit", { method: "POST", body: JSON.stringify(data) }),

  execCoverage: (data: { run_id: string }) =>
    request<{ exec: ExecResult }>("/api/v1/exec/coverage", { method: "POST", body: JSON.stringify(data) }),

  getRunAudit: (id: string) => request<SecurityAudit>(`/api/v1/runs/${id}/audit`),
  getRunCoverage: (id: string) => request<CoverageReport>(`/api/v1/runs/${id}/coverage`),
  getRunContract: (id: string) => request<ContractReport>(`/api/v1/runs/${id}/contract`),
  getRunComprehension: (id: string) => request<{ markdown: string }>(`/api/v1/runs/${id}/comprehension`),
};
