import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import Spinner from '../components/Spinner'

export default function Targets() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { data: targets, isLoading } = useQuery({ queryKey: ['targets'], queryFn: api.targets.list })
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('nexus-core')
  const [sourceType, setSourceType] = useState('path')
  const [sourceValue, setSourceValue] = useState('/home/pablo/Projects/Pablo/nexus/nexus-core')
  const [baseUrl, setBaseUrl] = useState('http://localhost:8080')
  const [authHeaderName, setAuthHeaderName] = useState('X-Nexus-Core-Key')
  const [authHeaderValue, setAuthHeaderValue] = useState('nexus-core-local-key')

  const create = useMutation({
    mutationFn: () => {
      const hint: Record<string, unknown> = {}
      if (baseUrl) hint.base_url = baseUrl
      if (authHeaderName && authHeaderValue) {
        hint.auth_headers = { [authHeaderName]: authHeaderValue }
      }
      return api.targets.create({
        name,
        source: { type: sourceType, value: sourceValue },
        runtime_hint: Object.keys(hint).length ? hint : undefined,
      })
    },
    onSuccess: (t) => {
      qc.invalidateQueries({ queryKey: ['targets'] })
      setOpen(false)
      setName(''); setSourceValue(''); setBaseUrl('')
      setAuthHeaderName(''); setAuthHeaderValue('')
      navigate(`/targets/${t.id}`)
    },
  })

  const remove = useMutation({
    mutationFn: (id: string) => api.targets.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['targets'] }),
  })

  const [confirmId, setConfirmId] = useState<string | null>(null)

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold">Targets</h1>
        <button onClick={() => setOpen(true)} className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 rounded-lg font-medium">
          + New Target
        </button>
      </div>

      {open && (
        <div className="mb-6 p-4 rounded-lg border border-gray-700 bg-gray-900 space-y-3">
          <input placeholder="Target name" value={name} onChange={e => setName(e.target.value)}
            className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm" />
          <div className="flex gap-2">
            <select value={sourceType} onChange={e => setSourceType(e.target.value)}
              className="px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm">
              <option value="path">Local path</option>
              <option value="repo_url">Repo URL</option>
            </select>
            <input placeholder="Source path or URL" value={sourceValue} onChange={e => setSourceValue(e.target.value)}
              className="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm" />
          </div>
          <input placeholder="Base URL (e.g. http://localhost:3000)" value={baseUrl} onChange={e => setBaseUrl(e.target.value)}
            className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm" />
          <div className="flex gap-2">
            <input placeholder="Auth header name (e.g. X-API-Key)" value={authHeaderName} onChange={e => setAuthHeaderName(e.target.value)}
              className="w-1/3 px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm" />
            <input placeholder="Auth header value" value={authHeaderValue} onChange={e => setAuthHeaderValue(e.target.value)}
              type="password"
              className="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm" />
          </div>
          <div className="flex gap-2">
            <button onClick={() => create.mutate()} disabled={!name || !sourceValue || create.isPending}
              className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 rounded font-medium disabled:opacity-40">
              {create.isPending ? 'Creating...' : 'Create'}
            </button>
            <button onClick={() => setOpen(false)} className="px-4 py-2 text-sm text-gray-400 hover:text-gray-200">Cancel</button>
          </div>
          {create.error && <p className="text-sm text-red-400">{(create.error as Error).message}</p>}
        </div>
      )}

      {isLoading ? <Spinner /> : (
        <div className="space-y-2">
          {targets?.map(t => (
            <div key={t.id} className="flex items-center gap-2">
              <button onClick={() => navigate(`/targets/${t.id}`)}
                className="flex-1 text-left p-4 rounded-lg border border-gray-800 hover:border-gray-600 bg-gray-900/50 hover:bg-gray-900 transition">
                <div className="flex justify-between items-center">
                  <div>
                    <p className="font-medium">{t.name}</p>
                    <p className="text-xs text-gray-500 mt-1">{t.source.type}: {t.source.value}</p>
                  </div>
                  <span className="text-xs text-gray-600">{new Date(t.created_at).toLocaleDateString()}</span>
                </div>
              </button>
              {confirmId === t.id ? (
                <div className="flex gap-1 shrink-0">
                  <button onClick={() => { remove.mutate(t.id); setConfirmId(null) }}
                    className="px-3 py-2 text-xs bg-red-600 hover:bg-red-500 rounded font-medium">
                    Confirmar
                  </button>
                  <button onClick={() => setConfirmId(null)}
                    className="px-3 py-2 text-xs text-gray-400 hover:text-gray-200">
                    No
                  </button>
                </div>
              ) : (
                <button onClick={(e) => { e.stopPropagation(); setConfirmId(t.id) }}
                  title="Eliminar target"
                  className="shrink-0 p-2 text-gray-600 hover:text-red-400 rounded hover:bg-gray-800 transition">
                  <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                    <path fillRule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd" />
                  </svg>
                </button>
              )}
            </div>
          ))}
          {targets?.length === 0 && <p className="text-gray-500 text-sm">No targets yet. Create one to get started.</p>}
        </div>
      )}
    </div>
  )
}
