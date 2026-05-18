import type * as T from './types'

const BASE = import.meta.env.VITE_API_BASE_URL ?? ''

async function requestJSON<T>(method: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(`${BASE}${path}`, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!response.ok) {
    const text = await response.text()
    throw new Error(text || `${response.status} ${response.statusText}`)
  }
  return response.json() as Promise<T>
}

function get<T>(path: string) {
  return requestJSON<T>('GET', path)
}

function post<T>(path: string, body?: unknown) {
  return requestJSON<T>('POST', path, body)
}

function put<T>(path: string, body?: unknown) {
  return requestJSON<T>('PUT', path, body)
}

export const api = {
  projects: {
    list: () => get<{ items: T.Project[] }>('/api/projects').then(r => r.items ?? []),
    create: (data: { name: string; source_path: string }) => post<T.Project>('/api/projects', data),
    audits: (projectId: string) => get<{ items: T.AuditRun[] }>(`/api/projects/${projectId}/audits`).then(r => r.items ?? []),
    createAudit: (projectId: string, config: T.AuditConfig) => post<T.AuditResult>(`/api/projects/${projectId}/audits`, config),
    specs: (projectId: string, module?: string) => {
      const query = module ? `?module=${encodeURIComponent(module)}` : ''
      return get<{ items: T.TaskSpec[] }>(`/api/projects/${projectId}/specs${query}`).then(r => r.items ?? [])
    },
    createSpec: (projectId: string, data: T.CreateTaskSpecRequest) => post<T.TaskSpec>(`/api/projects/${projectId}/specs`, data),
    prReviews: (projectId: string) => get<{ items: T.PRReview[] }>(`/api/projects/${projectId}/pr-reviews`).then(r => r.items ?? []),
    createPRReview: (projectId: string, data: T.CreatePRReviewRequest) => post<T.PRReviewResult>(`/api/projects/${projectId}/pr-reviews`, data),
  },
  audits: {
    get: (auditId: string) => get<T.AuditRun>(`/api/audits/${auditId}`),
    findings: (auditId: string) => get<{ items: T.Finding[] }>(`/api/audits/${auditId}/findings`).then(r => r.items ?? []),
    evidence: (auditId: string) => get<{ items: T.Evidence[] }>(`/api/audits/${auditId}/evidence`).then(r => r.items ?? []),
    docs: (auditId: string) => get<{ items: T.GeneratedDoc[] }>(`/api/audits/${auditId}/docs`).then(r => r.items ?? []),
    tests: (auditId: string) => get<{ items: T.TestResult[] }>(`/api/audits/${auditId}/tests`).then(r => r.items ?? []),
    score: (auditId: string) => get<{ items: T.ScoreItem[] }>(`/api/audits/${auditId}/score`).then(r => r.items ?? []),
  },
  specs: {
    get: (specId: string) => get<T.TaskSpec>(`/api/specs/${specId}`),
    update: (specId: string, data: T.CreateTaskSpecRequest) => put<T.TaskSpec>(`/api/specs/${specId}`, data),
  },
  prReviews: {
    get: (reviewId: string) => get<T.PRReviewResult>(`/api/pr-reviews/${reviewId}`),
    findings: (reviewId: string) => get<{ items: T.PRReviewFinding[] }>(`/api/pr-reviews/${reviewId}/findings`).then(r => r.items ?? []),
    files: (reviewId: string) => get<{ items: T.PRReviewFile[] }>(`/api/pr-reviews/${reviewId}/files`).then(r => r.items ?? []),
  },
}
