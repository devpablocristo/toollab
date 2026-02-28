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
    <div className="p-8 max-w-4xl mx-auto animate-fade-in">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-display font-bold tracking-wide">Targets</h1>
          <p className="text-xs font-mono text-ghost mt-1">Configure API targets for security analysis</p>
        </div>
        <button onClick={() => setOpen(true)} className="btn-primary flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
            <path fillRule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clipRule="evenodd" />
          </svg>
          New Target
        </button>
      </div>

      {open && (
        <div className="mb-8 p-5 card space-y-3 animate-fade-up glow-accent">
          <div className="flex items-center gap-2 mb-1">
            <div className="w-1.5 h-1.5 rounded-full bg-accent animate-glow-pulse" />
            <span className="text-xs font-display font-semibold tracking-wider text-accent uppercase">New Target</span>
          </div>
          <input placeholder="Target name" value={name} onChange={e => setName(e.target.value)} className="input" />
          <div className="flex gap-2">
            <select value={sourceType} onChange={e => setSourceType(e.target.value)}
              className="input w-auto">
              <option value="path">Local path</option>
              <option value="repo_url">Repo URL</option>
            </select>
            <input placeholder="Source path or URL" value={sourceValue} onChange={e => setSourceValue(e.target.value)}
              className="input flex-1" />
          </div>
          <input placeholder="Base URL (e.g. http://localhost:3000)" value={baseUrl} onChange={e => setBaseUrl(e.target.value)}
            className="input" />
          <div className="flex gap-2">
            <input placeholder="Auth header name (e.g. X-API-Key)" value={authHeaderName} onChange={e => setAuthHeaderName(e.target.value)}
              className="input w-1/3" />
            <input placeholder="Auth header value" value={authHeaderValue} onChange={e => setAuthHeaderValue(e.target.value)}
              type="password" className="input flex-1" />
          </div>
          <div className="flex gap-2 pt-1">
            <button onClick={() => create.mutate()} disabled={!name || !sourceValue || create.isPending}
              className="btn-primary">
              {create.isPending ? 'Creating...' : 'Create Target'}
            </button>
            <button onClick={() => setOpen(false)} className="btn-ghost">Cancel</button>
          </div>
          {create.error && <p className="text-sm text-danger font-mono">{(create.error as Error).message}</p>}
        </div>
      )}

      {isLoading ? (
        <div className="flex justify-center py-12"><Spinner /></div>
      ) : (
        <div className="space-y-2">
          {targets?.map((t, idx) => (
            <div key={t.id} className={`flex items-center gap-2 animate-fade-up stagger-${Math.min(idx + 1, 8)}`}>
              <button onClick={() => navigate(`/targets/${t.id}`)}
                className="flex-1 text-left p-4 card group">
                <div className="flex justify-between items-center">
                  <div className="flex items-center gap-3">
                    <div className="w-1 h-8 rounded-full bg-accent/30 group-hover:bg-accent transition-colors" />
                    <div>
                      <p className="font-display font-semibold tracking-wide group-hover:text-accent transition-colors">{t.name}</p>
                      <p className="text-xs font-mono text-ghost mt-0.5">{t.source.type}: {t.source.value}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-xs font-mono text-ghost-faint">{new Date(t.created_at).toLocaleDateString()}</span>
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 text-ghost-faint group-hover:text-accent transition-colors" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clipRule="evenodd" />
                    </svg>
                  </div>
                </div>
              </button>
              {confirmId === t.id ? (
                <div className="flex gap-1 shrink-0">
                  <button onClick={() => { remove.mutate(t.id); setConfirmId(null) }} className="btn-danger">
                    Confirmar
                  </button>
                  <button onClick={() => setConfirmId(null)} className="btn-ghost text-xs">
                    No
                  </button>
                </div>
              ) : (
                <button onClick={(e) => { e.stopPropagation(); setConfirmId(t.id) }}
                  title="Eliminar target"
                  className="shrink-0 p-2.5 text-ghost-faint hover:text-danger rounded-sm hover:bg-danger-muted transition-colors">
                  <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                    <path fillRule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd" />
                  </svg>
                </button>
              )}
            </div>
          ))}
          {targets?.length === 0 && (
            <div className="text-center py-16 animate-fade-in">
              <div className="w-12 h-12 mx-auto mb-4 rounded-sm border border-edge flex items-center justify-center">
                <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-ghost-faint" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                </svg>
              </div>
              <p className="text-ghost font-body text-sm">No targets yet</p>
              <p className="text-ghost-faint font-mono text-xs mt-1">Create one to begin security analysis</p>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
