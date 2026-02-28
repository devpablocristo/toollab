export interface Target {
  id: string
  name: string
  source: { type: string; value: string }
  runtime_hint?: { base_url?: string; docker_compose_path?: string; cmd?: string }
  created_at: string
  updated_at: string
}

// --- Analysis (unified pipeline output) ---

export interface ProgressEvent {
  phase: string
  message: string
  current?: number
  total?: number
}

export interface AnalyzeResult {
  target_id: string
  run_id: string
  analysis: Analysis
}

export interface BehaviorObservation {
  quality: string
  summary: string
  tested: number
}

export interface InferredModel {
  name: string
  fields: { name: string; json_type: string; example?: string }[]
  seen_from: string[]
}

export interface EndpointBehaviorSummary {
  endpoint: string
  method: string
  path: string
  status_codes: Record<number, number>
  request_count: number
  avg_latency_ms: number
  error_count: number
  requires_auth: boolean
}

export interface ProbesSummary {
  total_probes: number
  injection_probes: number
  malformed_probes: number
  boundary_probes: number
  method_tamper_probes: number
  hidden_endpoint_probes: number
  large_payload_probes: number
  content_type_probes: number
  no_auth_probes: number
}

export interface Analysis {
  target_id: string
  target_name: string
  created_at: string
  run_id: string
  discovery: { framework: string; endpoints_count: number; confidence: number; gaps: string[] }
  performance: { total_requests: number; success_rate: number; error_rate: number; p50_ms: number; p95_ms: number; p99_ms: number; status_histogram: Record<string, number>; slowest_endpoints: { method: string; path: string; timing_ms: number; status: number }[] }
  security: { score: number; grade: string; findings: { id: string; category: string; severity: string; title: string; description: string; endpoint?: string; remediation: string }[]; summary: { total: number; critical: number; high: number; medium: number; low: number } }
  contract: { compliant: boolean; compliance_rate: number; total_checks: number; total_violations: number; violations: { endpoint: string; status_code: number; field: string; expected: string; actual: string; description: string }[] }
  coverage: { total_endpoints: number; tested_endpoints: number; coverage_rate: number; by_method: { method: string; total: number; tested: number; rate: number }[]; status_codes_observed: { code: number; observed: boolean }[]; untested: { method: string; path: string }[] }
  behavior: {
    invalid_input: BehaviorObservation
    missing_auth: BehaviorObservation
    not_found: BehaviorObservation
    error_consistency: BehaviorObservation
    inferred_models: InferredModel[]
    endpoint_behavior: EndpointBehaviorSummary[]
  }
  probes_summary: ProbesSummary
  score: number
  grade: string
}

// --- LLM Interpretation ---

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

export interface LLMInterpretation {
  schema_version: string
  run_id: string
  created_at: string
  overview?: { service_name: string; description: string; framework: string; total_endpoints: number; architecture_notes: string }
  data_models?: { name: string; description: string; fields: string[]; used_by: string[] }[]
  flows?: { name: string; description: string; importance: string; endpoints: string[]; sequence: string; example_requests?: FlowExample[]; evidence_refs?: string[] }[]
  security_assessment?: { overall_risk: string; summary: string; critical_findings: string[]; positive_findings: string[]; attack_surface: string }
  behavior_assessment?: { input_validation: string; auth_enforcement: string; error_handling: string; robustness: string }
  facts: { text: string; evidence_refs?: string[]; confidence: number }[]
  inferences: { text: string; rule_of_inference: string; evidence_refs?: string[]; confidence: number }[]
  improvements?: { title: string; severity: string; category: string; description: string; remediation: string; evidence_refs?: string[] }[]
  tests?: { name: string; description: string; flow: string; importance: string; request: { method: string; path: string; headers?: Record<string, string>; body?: unknown }; expected: { status: number; body_contains?: string[]; description: string }; evidence_refs?: string[] }[]
  open_questions: { question: string; why_missing?: string }[]
  stats: {
    facts_count: number
    inferences_count: number
    questions_count: number
    rejected_claims_count: number
    provider_name: string
    validation_mode: string
  }
}
