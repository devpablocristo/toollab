const BASE = import.meta.env.VITE_API_BASE_URL ?? ''

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  }
  if (body !== undefined) opts.body = JSON.stringify(body)
  const res = await fetch(`${BASE}${path}`, opts)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(`${res.status}: ${text}`)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

function get<T>(path: string) { return request<T>('GET', path) }
function post<T>(path: string, body?: unknown) { return request<T>('POST', path, body) }
function del<T>(path: string) { return request<T>('DELETE', path) }

import type * as T from './types'

export const api = {
  targets: {
    list: () => get<{ items: T.Target[] }>('/api/v1/targets').then(r => r.items ?? []),
    get: (id: string) => get<T.Target>(`/api/v1/targets/${id}`),
    create: (data: { name: string; source: { type: string; value: string }; runtime_hint?: Record<string, unknown> }) =>
      post<T.Target>('/api/v1/targets', data),
    delete: (id: string) => del<void>(`/api/v1/targets/${id}`),
    latestRun: (id: string) => get<{ run: T.Run; run_summary: T.RunSummary | null }>(`/api/v1/targets/${id}/latest-run`),
    analyzeSSE: (targetId: string, onProgress: (event: T.ProgressEvent) => void): Promise<T.AnalyzeResult> => {
      return new Promise((resolve, reject) => {
        const url = `${BASE}/api/v1/targets/${targetId}/analyze`
        fetch(url, { method: 'POST', headers: { 'Accept': 'text/event-stream' } }).then(response => {
          if (!response.ok) {
            response.text().then(t => reject(new Error(`${response.status}: ${t}`))).catch(() => reject(new Error(`${response.status}`)))
            return
          }
          const reader = response.body?.getReader()
          if (!reader) { reject(new Error('No response body')); return }
          const decoder = new TextDecoder()
          let buffer = ''

          function pump(): Promise<void> {
            return reader!.read().then(({ done, value }) => {
              if (done) { reject(new Error('Stream ended without result')); return }
              buffer += decoder.decode(value, { stream: true })
              const lines = buffer.split('\n')
              buffer = lines.pop() ?? ''
              let currentEvent = ''
              for (const line of lines) {
                if (line.startsWith('event: ')) {
                  currentEvent = line.slice(7).trim()
                } else if (line.startsWith('data: ')) {
                  const data = line.slice(6)
                  try {
                    const parsed = JSON.parse(data)
                    if (currentEvent === 'progress') {
                      onProgress(parsed)
                    } else if (currentEvent === 'result') {
                      resolve(parsed)
                      return
                    } else if (currentEvent === 'error') {
                      reject(new Error(parsed.error ?? 'Analysis failed'))
                      return
                    }
                  } catch { /* skip malformed */ }
                }
              }
              return pump()
            })
          }
          pump().catch(reject)
        }).catch(reject)
      })
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
