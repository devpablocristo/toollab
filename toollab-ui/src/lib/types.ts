export interface Project {
  id: string
  name: string
  source_path: string
  created_at: string
  updated_at: string
}

export interface AuditConfig {
  generate_tests: boolean
  run_existing_tests: boolean
  allow_docs_read: boolean
  allow_dependency_install: boolean
}

export interface AuditRun {
  id: string
  project_id: string
  status: string
  score: number
  score_breakdown: Record<string, number>
  summary: string
  stack: Record<string, string>
  created_at: string
  completed_at?: string
}

export interface Evidence {
  id: string
  audit_id: string
  kind: string
  summary: string
  command?: string
  file_path?: string
  line?: number
  created_at: string
}

export interface Finding {
  id: string
  audit_id: string
  rule_id: string
  severity: string
  priority: string
  state: string
  category: string
  title: string
  description: string
  confidence: string
  file_path?: string
  line?: number
  evidence_refs: Evidence[]
  details: FindingDetails
  created_at: string
}

export interface FindingDetails {
  why_problem?: string
  impact?: string
  risk_of_change?: string
  minimum_recommendation?: string
  avoid?: string
  validation?: string
}

export interface GeneratedDoc {
  id: string
  audit_id: string
  title: string
  content: string
  created_at: string
}

export interface TestResult {
  id: string
  audit_id: string
  kind: string
  name: string
  command?: string
  status: string
  output?: string
  generated_path?: string
  created_at: string
}

export interface ScoreItem {
  id: string
  audit_id: string
  category: string
  max_points: number
  awarded_points: number
  deducted_points: number
  reason: string
  created_at: string
}

export interface AuditResult {
  run: AuditRun
  findings: Finding[]
  evidence: Evidence[]
  docs: GeneratedDoc[]
  tests: TestResult[]
  score_items: ScoreItem[]
}
