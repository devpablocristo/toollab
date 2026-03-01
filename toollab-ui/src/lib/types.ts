export interface Target {
  id: string
  name: string
  source: { type: string; value: string }
  runtime_hint?: { base_url?: string; docker_compose_path?: string; cmd?: string }
  created_at: string
  updated_at: string
}

export interface Run {
  id: string
  target_id: string
  status: string
  seed?: string
  notes?: string
  created_at: string
  completed_at?: string
}

export interface ProgressEvent {
  step: string
  phase: string
  message: string
  current?: number
  total?: number
}

export interface AnalyzeResult {
  target_id: string
  run_id: string
  run_summary: RunSummary
}

export type RunMode = 'offline' | 'online_partial' | 'online_good' | 'online_strong'

export interface RunModeDetail {
  mode: RunMode
  total_samples: number
  http_responses: number
  connection_errors: number
  connection_error_pct: number
  happy_path_endpoints: number
  real_errors: number
  confirmed_findings: number
  reason: string
}

export interface RunSummary {
  run_id: string
  status: string
  run_mode: RunMode
  run_mode_detail?: RunModeDetail
  duration_seconds: number
  endpoints_discovered_ast: number
  endpoints_confirmed_runtime: number
  coverage_pct: number
  evidence_count_full: number
  budget_usage: BudgetUsage
  scores_available: boolean
  scores?: Record<string, number>
  top_findings: FindingSummary[]
}

export interface PlaygroundRequest {
  endpoint_id: string
  method: string
  url: string
  path_params?: Record<string, string>
  query?: Record<string, string>
  headers?: Record<string, string>
  body?: string
  auth_profile_id?: string
  timeout_ms?: number
}

export interface PlaygroundResponse {
  evidence_id: string
  status?: number
  headers?: Record<string, string>
  body?: string
  body_snippet?: string
  content_type?: string
  latency_ms: number
  size?: number
  error?: string
  error_signature_id?: string
}

export interface AuthProfile {
  id: string
  name: string
  mechanism: string
  header_key?: string
  masked_value: string
  env?: string
}

export interface BudgetUsage {
  requests_total: number
  duration_seconds: number
  by_category: Record<string, number>
}

export interface FindingSummary {
  id: string
  severity: string
  title: string
  evidence_refs: string[]
}

// --- LLM Reports ---

export interface FlowExample {
  step: string
  method: string
  path: string
  headers?: Record<string, string>
  body?: unknown
  expected_status: number
  expected_response_snippet?: unknown
  notes?: string
}

export interface LLMReport {
  [key: string]: unknown
}

// --- Endpoint Intelligence ---

export interface IntelIndex {
  schema_version: string
  run_id: string
  base_url: string
  total_endpoints: number
  domains: IntelDomainSummary[]
  endpoints: IntelEndpointIndex[]
}

export interface IntelDomainSummary {
  domain_name: string
  endpoint_count: number
}

export interface IntelEndpointIndex {
  endpoint_id: string
  method: string
  path: string
  operation_id: string
  domain: string
  auth_required: string
  summary: string
  confidence: number
  command_count: number
  has_evidence: boolean
}

export interface IntelEndpointDetail {
  domain: string
  endpoint: IntelEndpoint
}

export interface IntelEndpoint {
  endpoint_id: string
  method: string
  path_template: string
  operation_id: string
  tags: string[]
  auth: { required: string; from: string; mechanism: string; notes?: string }
  what_it_does: {
    summary: string
    detailed: string
    confidence: number
    facts: { text: string; evidence_refs?: string[] }[]
    inferences: { text: string; rule_of_inference: string; confidence: number; ast_refs?: unknown[]; evidence_refs?: string[] }[]
  }
  inputs: {
    path_params?: { name: string; type: string; meaning: string; source: string; confidence: number }[]
    query_params?: { name: string; type: string; meaning?: string; observed_values?: string[]; source: string; confidence: number }[]
    headers?: { name: string; required: string; observed_values?: string[]; source: string; confidence: number }[]
    body?: { content_type: string; schema_ref?: string; required_fields?: { field_path: string; type: string; meaning?: string; source: string; confidence: number }[]; example_from_evidence_ref?: string; notes?: string }
  }
  outputs: {
    responses?: { status: number; content_type: string; schema_ref?: string; what_you_get: string; example_ref?: string }[]
    common_errors?: { status: number; meaning: string; example_ref?: string }[]
  }
  how_to_query: {
    goal: string
    ready_commands: { name: string; kind: string; command: string; placeholders?: { name: string; example: string }[]; based_on: string; evidence_refs?: string[]; notes?: string }[]
    query_variants?: { variant_name: string; description: string; command: string; source: string; confidence: number; evidence_refs?: string[]; notes?: string }[]
    warnings?: string[]
  }
  tests_you_should_run?: { name: string; why: string; command_ref?: string; importance: string; evidence_refs?: string[] }[]
  security_notes: {
    exposures?: { text: string; severity: string; evidence_refs?: string[] }[]
    ast_code_patterns_related?: { pattern: string; ast_ref: unknown; only_if_correlated_with_runtime: boolean; evidence_refs?: string[] }[]
  }
  ast_refs?: unknown[]
  evidence_refs?: string[]
}

export interface EndpointScriptSet {
  happy_path?: string
  no_auth?: string
  invalid_auth?: string
  common_errors?: string
  variants?: string
  http_file?: string
}
