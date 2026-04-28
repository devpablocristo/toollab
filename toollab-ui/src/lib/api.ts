import { request, requestJSONEventStream } from '@devpablocristo/core-http'

const BASE = import.meta.env.VITE_API_BASE_URL ?? ''

async function apiRequest<T>(method: string, path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method,
    body,
    baseURLs: BASE ? [BASE] : undefined,
  })
}

function get<T>(path: string) { return apiRequest<T>('GET', path) }
function post<T>(path: string, body?: unknown) { return apiRequest<T>('POST', path, body) }
function del<T>(path: string) { return apiRequest<T>('DELETE', path) }

import type * as T from './types'

export const api = {
  v2: {
    repos: {
      list: () => get<{ items: T.RepoV2[] }>('/api/v2/repos').then(r => r.items ?? []),
      create: (data: { name: string; source_type: 'path'; source_path: string; doc_policy?: string }) =>
        post<T.RepoV2>('/api/v2/repos', data),
      audits: (repoId: string) => get<{ items: T.AuditRunV2[] }>(`/api/v2/repos/${repoId}/audits`).then(r => r.items ?? []),
      createAudit: (repoId: string, config: T.AuditConfigV2) =>
        post<T.AuditResultV2>(`/api/v2/repos/${repoId}/audits`, config),
    },
    audits: {
      get: (auditId: string) => get<T.AuditRunV2>(`/api/v2/audits/${auditId}`),
      findings: (auditId: string) => get<{ items: T.FindingV2[] }>(`/api/v2/audits/${auditId}/findings`).then(r => r.items ?? []),
      docs: (auditId: string) => get<{ items: T.GeneratedDocV2[] }>(`/api/v2/audits/${auditId}/docs`).then(r => r.items ?? []),
      tests: (auditId: string) => get<{ items: T.TestResultV2[] }>(`/api/v2/audits/${auditId}/tests`).then(r => r.items ?? []),
      evidence: (auditId: string) => get<{ items: T.EvidenceV2[] }>(`/api/v2/audits/${auditId}/evidence`).then(r => r.items ?? []),
      score: (auditId: string) => get<{ items: T.ScoreItemV2[] }>(`/api/v2/audits/${auditId}/score`).then(r => r.items ?? []),
    },
  },
  targets: {
    list: () => get<{ items: T.Target[] }>('/api/v1/targets').then(r => r.items ?? []),
    get: (id: string) => get<T.Target>(`/api/v1/targets/${id}`),
    create: (data: { name: string; description?: string; source: { type: string; value: string }; runtime_hint?: Record<string, unknown> }) =>
      post<T.Target>('/api/v1/targets', data),
    delete: (id: string) => del<void>(`/api/v1/targets/${id}`),
    latestRun: (id: string) => get<{ run: T.Run; run_summary: T.RunSummary | null }>(`/api/v1/targets/${id}/latest-run`),
    analyzeSSE: (targetId: string, onProgress: (event: T.ProgressEvent) => void, lang?: string): Promise<T.AnalyzeResult> => {
      const langParam = lang ? `?lang=${lang}` : ''
      return requestJSONEventStream<T.ProgressEvent, T.AnalyzeResult>(
        `/api/v1/targets/${targetId}/analyze${langParam}`,
        {
          method: 'POST',
          baseURLs: BASE ? [BASE] : undefined,
          onProgress,
        },
      )
    },
  },
  runs: {
    audit: (runId: string) => get<T.LLMReport>(`/api/v1/runs/${runId}/audit`),
    docs: (runId: string) => get<T.LLMReport>(`/api/v1/runs/${runId}/docs`),
    artifact: (runId: string, type: string) => get<unknown>(`/api/v1/runs/${runId}/artifact/${type}`),
    endpointIndex: (runId: string) => get<T.IntelIndex>(`/api/v1/runs/${runId}/endpoints`),
    endpointDetail: (runId: string, endpointId: string) => get<T.IntelEndpointDetail>(`/api/v1/runs/${runId}/endpoints/${endpointId}`),
    endpointScripts: (runId: string, endpointId: string) => get<T.EndpointScriptSet>(`/api/v1/runs/${runId}/endpoints/${endpointId}/scripts`),
  },
  playground: {
    send: (runId: string, req: T.PlaygroundRequest) =>
      post<T.PlaygroundResponse>(`/api/v1/runs/${runId}/playground/send`, req),
    replay: (runId: string, evidenceId: string) =>
      post<T.PlaygroundResponse>(`/api/v1/runs/${runId}/playground/replay`, { evidence_id: evidenceId }),
    authProfiles: (runId: string) =>
      get<{ profiles: T.AuthProfile[] }>(`/api/v1/runs/${runId}/playground/auth-profiles`).then(r => r.profiles ?? []),
    createAuthProfile: (runId: string, data: { name: string; mechanism: string; header_key?: string; value: string; env?: string }) =>
      post<T.AuthProfile>(`/api/v1/runs/${runId}/playground/auth-profiles`, data),
    deleteAuthProfile: (runId: string, profileId: string) =>
      del<void>(`/api/v1/runs/${runId}/playground/auth-profiles/${profileId}`),
  },
}
