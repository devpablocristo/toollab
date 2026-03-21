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
