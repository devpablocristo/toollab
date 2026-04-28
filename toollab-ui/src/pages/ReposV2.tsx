import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { api } from '../lib/api'
import type { AuditConfigV2, AuditRunV2, EvidenceV2, FindingV2, GeneratedDocV2, RepoV2, ScoreItemV2, TestResultV2 } from '../lib/types'
import Spinner from '../components/Spinner'

const defaultConfig: AuditConfigV2 = {
  generate_tests: true,
  run_existing_tests: true,
  allow_docs_read: false,
  allow_dependency_install: false,
}

export default function ReposV2() {
  const qc = useQueryClient()
  const [selectedRepoId, setSelectedRepoId] = useState<string | null>(null)
  const [selectedAuditId, setSelectedAuditId] = useState<string | null>(null)
  const [name, setName] = useState('toollab')
  const [sourcePath, setSourcePath] = useState('/home/pablocristo/Proyectos/pablo/toollab')
  const [config, setConfig] = useState<AuditConfigV2>(defaultConfig)

  const { data: repos, isLoading } = useQuery({ queryKey: ['v2-repos'], queryFn: api.v2.repos.list })
  const selectedRepo = repos?.find(r => r.id === selectedRepoId) ?? repos?.[0] ?? null
  const repoId = selectedRepo?.id ?? null

  const { data: audits } = useQuery({
    queryKey: ['v2-audits', repoId],
    queryFn: () => api.v2.repos.audits(repoId!),
    enabled: !!repoId,
  })

  const currentAuditId = selectedAuditId ?? audits?.[0]?.id ?? null
  const { data: audit } = useQuery({
    queryKey: ['v2-audit', currentAuditId],
    queryFn: () => api.v2.audits.get(currentAuditId!),
    enabled: !!currentAuditId,
  })
  const { data: findings } = useQuery({
    queryKey: ['v2-findings', currentAuditId],
    queryFn: () => api.v2.audits.findings(currentAuditId!),
    enabled: !!currentAuditId,
  })
  const { data: docs } = useQuery({
    queryKey: ['v2-docs', currentAuditId],
    queryFn: () => api.v2.audits.docs(currentAuditId!),
    enabled: !!currentAuditId,
  })
  const { data: tests } = useQuery({
    queryKey: ['v2-tests', currentAuditId],
    queryFn: () => api.v2.audits.tests(currentAuditId!),
    enabled: !!currentAuditId,
  })
  const { data: evidence } = useQuery({
    queryKey: ['v2-evidence', currentAuditId],
    queryFn: () => api.v2.audits.evidence(currentAuditId!),
    enabled: !!currentAuditId,
  })
  const { data: scoreItems } = useQuery({
    queryKey: ['v2-score', currentAuditId],
    queryFn: () => api.v2.audits.score(currentAuditId!),
    enabled: !!currentAuditId,
  })

  const createRepo = useMutation({
    mutationFn: () => api.v2.repos.create({
      name,
      source_type: 'path',
      source_path: sourcePath,
      doc_policy: config.allow_docs_read ? 'allow_existing_docs' : 'ignore_existing_docs',
    }),
    onSuccess: repo => {
      qc.invalidateQueries({ queryKey: ['v2-repos'] })
      setSelectedRepoId(repo.id)
      setSelectedAuditId(null)
    },
  })

  const createAudit = useMutation({
    mutationFn: (repo: RepoV2) => api.v2.repos.createAudit(repo.id, config),
    onSuccess: result => {
      setSelectedAuditId(result.run.id)
      qc.invalidateQueries({ queryKey: ['v2-audits', result.run.repo_id] })
      qc.invalidateQueries({ queryKey: ['v2-audit', result.run.id] })
      qc.invalidateQueries({ queryKey: ['v2-findings', result.run.id] })
      qc.invalidateQueries({ queryKey: ['v2-docs', result.run.id] })
      qc.invalidateQueries({ queryKey: ['v2-tests', result.run.id] })
      qc.invalidateQueries({ queryKey: ['v2-evidence', result.run.id] })
      qc.invalidateQueries({ queryKey: ['v2-score', result.run.id] })
    },
  })

  const sortedAudits = useMemo(() => audits ?? [], [audits])

  return (
    <div className="p-8 max-w-7xl mx-auto animate-fade-in">
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-display font-bold tracking-wide">Repo Auditor</h1>
          <p className="text-xs font-mono text-ghost mt-1">ToolLab V2 audits source repositories without trusting repository docs by default.</p>
        </div>
        <a href="/targets" className="text-xs font-mono text-ghost hover:text-accent border border-edge rounded-sm px-3 py-1.5">V1 legacy</a>
      </div>

      <div className="grid grid-cols-[320px_1fr] gap-6">
        <aside className="space-y-4">
          <section className="card p-4 space-y-3">
            <h2 className="section-title"><span>New repo</span></h2>
            <input className="input" value={name} onChange={e => setName(e.target.value)} placeholder="Repo name" />
            <input className="input" value={sourcePath} onChange={e => setSourcePath(e.target.value)} placeholder="Local path" />
            <Toggle label="Generate tests" checked={config.generate_tests} onChange={v => setConfig({ ...config, generate_tests: v })} />
            <Toggle label="Run existing tests" checked={config.run_existing_tests} onChange={v => setConfig({ ...config, run_existing_tests: v })} />
            <Toggle label="Allow docs read" checked={config.allow_docs_read} onChange={v => setConfig({ ...config, allow_docs_read: v })} />
            <Toggle label="Install dependencies" checked={config.allow_dependency_install} onChange={v => setConfig({ ...config, allow_dependency_install: v })} />
            <button className="btn-primary w-full" disabled={!name || !sourcePath || createRepo.isPending} onClick={() => createRepo.mutate()}>
              {createRepo.isPending ? 'Creating...' : 'Create repo'}
            </button>
            {createRepo.error && <p className="text-xs font-mono text-danger">{(createRepo.error as Error).message}</p>}
          </section>

          <section className="card p-4">
            <h2 className="section-title"><span>Repos</span></h2>
            {isLoading ? <Spinner /> : (
              <div className="space-y-2">
                {(repos ?? []).map(repo => (
                  <button key={repo.id} onClick={() => { setSelectedRepoId(repo.id); setSelectedAuditId(null) }}
                    className={`w-full text-left p-3 rounded-sm border transition-colors ${repo.id === repoId ? 'border-accent bg-accent/10' : 'border-edge bg-surface/40 hover:border-ghost'}`}>
                    <p className="text-sm font-display font-semibold">{repo.name}</p>
                    <p className="text-[10px] font-mono text-ghost truncate">{repo.source_path}</p>
                  </button>
                ))}
                {(repos ?? []).length === 0 && <p className="text-xs text-ghost">No V2 repos yet.</p>}
              </div>
            )}
          </section>
        </aside>

        <main className="space-y-6">
          {selectedRepo ? (
            <>
              <section className="card p-5 flex items-center justify-between gap-4">
                <div>
                  <p className="text-xs font-mono text-ghost">Selected repo</p>
                  <h2 className="text-xl font-display font-bold">{selectedRepo.name}</h2>
                  <p className="text-xs font-mono text-ghost mt-1">{selectedRepo.source_path}</p>
                </div>
                <button className="btn-primary" disabled={createAudit.isPending} onClick={() => createAudit.mutate(selectedRepo)}>
                  {createAudit.isPending ? 'Auditing...' : 'Run audit'}
                </button>
              </section>

              {createAudit.error && <p className="text-sm font-mono text-danger">{(createAudit.error as Error).message}</p>}

              <section className="card p-4">
                <h2 className="section-title"><span>Audit runs</span></h2>
                <div className="flex flex-wrap gap-2">
                  {sortedAudits.map(run => (
                    <button key={run.id} onClick={() => setSelectedAuditId(run.id)}
                      className={`px-3 py-2 rounded-sm border text-xs font-mono ${run.id === currentAuditId ? 'border-accent text-accent bg-accent/10' : 'border-edge text-ghost hover:border-ghost'}`}>
                      {run.id.slice(0, 8)} · {run.score}/100
                    </button>
                  ))}
                  {sortedAudits.length === 0 && <p className="text-xs text-ghost">No audits yet.</p>}
                </div>
              </section>

              {audit && <AuditOverview audit={audit} />}
              {audit && <ScorePanel items={scoreItems ?? []} />}
              {audit && <FindingsPanel findings={findings ?? []} />}
              {audit && <EvidencePanel evidence={evidence ?? []} />}
              {audit && <DocsPanel docs={docs ?? []} />}
              {audit && <TestsPanel tests={tests ?? []} />}
            </>
          ) : (
            <section className="card p-10 text-center">
              <p className="text-sm text-ghost">Create or select a repo to start a V2 audit.</p>
            </section>
          )}
        </main>
      </div>
    </div>
  )
}

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex items-center justify-between gap-3 text-xs font-mono text-ghost">
      <span>{label}</span>
      <input type="checkbox" checked={checked} onChange={e => onChange(e.target.checked)} />
    </label>
  )
}

function AuditOverview({ audit }: { audit: AuditRunV2 }) {
  const scoreColor = audit.score >= 80 ? 'text-ok' : audit.score >= 60 ? 'text-info' : audit.score >= 40 ? 'text-warn' : 'text-danger'
  return (
    <section className="card p-5">
      <div className="grid grid-cols-[160px_1fr] gap-6">
        <div>
          <p className="text-xs font-mono text-ghost mb-1">Score</p>
          <p className={`text-5xl font-display font-bold ${scoreColor}`}>{audit.score}</p>
          <p className="text-xs font-mono text-ghost">/100</p>
        </div>
        <div className="space-y-3">
          <p className="text-sm text-zinc-300">{audit.summary}</p>
          <div className="grid grid-cols-5 gap-2">
            {Object.entries(audit.score_breakdown).map(([k, v]) => (
              <div key={k} className="border border-edge rounded-sm p-2">
                <p className="text-[10px] font-mono text-ghost truncate">{k}</p>
                <p className="text-lg font-display font-bold">{v}</p>
              </div>
            ))}
          </div>
          <div className="flex flex-wrap gap-1">
            {Object.keys(audit.stack).map(k => <span key={k} className="px-2 py-0.5 rounded-sm border border-edge text-[10px] font-mono text-accent">{k}</span>)}
          </div>
        </div>
      </div>
    </section>
  )
}

function ScorePanel({ items }: { items: ScoreItemV2[] }) {
  return (
    <section className="card p-5">
      <h2 className="section-title"><span>Score reasons</span></h2>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {items.map(item => (
          <div key={item.id} className="border border-edge rounded-sm p-3">
            <div className="flex items-center justify-between gap-3">
              <p className="text-xs font-mono text-accent">{item.category}</p>
              <p className="text-sm font-display font-bold">{item.awarded_points}/{item.max_points}</p>
            </div>
            <p className="text-xs text-zinc-300 mt-2">{item.reason}</p>
            {item.deducted_points > 0 && <p className="text-[10px] font-mono text-warn mt-2">-{item.deducted_points} points</p>}
            {item.evidence_refs.length > 0 && (
              <p className="text-[10px] font-mono text-ghost mt-2">{item.evidence_refs.length} evidence refs</p>
            )}
          </div>
        ))}
        {items.length === 0 && <p className="text-xs text-ghost">No score details for this audit.</p>}
      </div>
    </section>
  )
}

function FindingsPanel({ findings }: { findings: FindingV2[] }) {
  return (
    <section className="card p-5">
      <h2 className="section-title"><span>Findings</span></h2>
      <div className="space-y-3">
        {findings.map(f => (
          <div key={f.id} className="border border-edge rounded-sm p-4">
            <div className="flex items-center gap-2 mb-2">
              <Severity severity={f.severity} />
              <span className="text-[10px] font-mono text-ghost">{f.priority}</span>
              <span className="text-[10px] font-mono text-ghost">{f.state}</span>
              {f.rule_id && <span className="text-[10px] font-mono text-accent">{f.rule_id}</span>}
            </div>
            <h3 className="text-sm font-display font-bold">{f.title}</h3>
            <p className="text-xs text-zinc-300 mt-1">{f.description}</p>
            {f.file_path && <p className="text-[10px] font-mono text-ghost mt-2">{f.file_path}{f.line ? `:${f.line}` : ''}</p>}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2 mt-3">
              {detailEntries(f).map(([label, value]) => (
                <div key={label} className="border border-edge/70 rounded-sm p-2">
                  <p className="text-[10px] font-mono text-ghost">{label}</p>
                  <p className="text-xs text-zinc-300 mt-1">{value}</p>
                </div>
              ))}
            </div>
            {f.evidence_refs.length > 0 && (
              <div className="mt-3 space-y-1">
                {f.evidence_refs.map(ref => (
                  <p key={ref.id || `${ref.file_path}-${ref.line}-${ref.summary}`} className="text-[10px] font-mono text-ghost">
                    evidence · {formatEvidence(ref)}
                  </p>
                ))}
              </div>
            )}
          </div>
        ))}
        {findings.length === 0 && <p className="text-xs text-ghost">No findings for this audit.</p>}
      </div>
    </section>
  )
}

function EvidencePanel({ evidence }: { evidence: EvidenceV2[] }) {
  return (
    <section className="card p-5">
      <h2 className="section-title"><span>Evidence ledger</span></h2>
      <div className="space-y-2 max-h-72 overflow-auto">
        {evidence.map(item => (
          <div key={item.id} className="border border-edge rounded-sm p-3">
            <div className="flex items-center gap-2">
              <span className="text-[10px] font-mono text-accent">{item.kind}</span>
              {item.command && <span className="text-[10px] font-mono text-ghost">{item.command}</span>}
            </div>
            <p className="text-xs text-zinc-300 mt-1">{item.summary}</p>
            {item.file_path && <p className="text-[10px] font-mono text-ghost mt-1">{item.file_path}{item.line ? `:${item.line}` : ''}</p>}
          </div>
        ))}
        {evidence.length === 0 && <p className="text-xs text-ghost">No evidence recorded for this audit.</p>}
      </div>
    </section>
  )
}

function detailEntries(finding: FindingV2) {
  const d = finding.details ?? {}
  return [
    ['Why', d.why_problem],
    ['Impact', d.impact],
    ['Minimum', d.minimum_recommendation],
    ['Avoid', d.avoid],
    ['Validation', d.validation],
  ].filter((entry): entry is [string, string] => Boolean(entry[1]))
}

function formatEvidence(ref: EvidenceV2) {
  const loc = ref.file_path ? `${ref.file_path}${ref.line ? `:${ref.line}` : ''}` : ''
  return [loc, ref.command, ref.summary].filter(Boolean).join(' · ')
}

function DocsPanel({ docs }: { docs: GeneratedDocV2[] }) {
  const doc = docs[0]
  return (
    <section className="card p-5">
      <h2 className="section-title"><span>Generated docs</span></h2>
      {doc ? (
        <div className="prose prose-invert prose-sm max-w-none">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{doc.content}</ReactMarkdown>
        </div>
      ) : <p className="text-xs text-ghost">No generated docs yet.</p>}
    </section>
  )
}

function TestsPanel({ tests }: { tests: TestResultV2[] }) {
  return (
    <section className="card p-5">
      <h2 className="section-title"><span>Tests</span></h2>
      <div className="space-y-3">
        {tests.map(t => (
          <div key={t.id} className="border border-edge rounded-sm p-3">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-display font-semibold">{t.name}</p>
                <p className="text-[10px] font-mono text-ghost">{t.kind} · {t.command || 'no command'}</p>
              </div>
              <span className={`text-xs font-mono ${t.status === 'passed' ? 'text-ok' : t.status === 'blocked' ? 'text-warn' : 'text-danger'}`}>{t.status}</span>
            </div>
            {t.generated_path && <p className="text-[10px] font-mono text-ghost mt-2">{t.generated_path}</p>}
            {t.output && <pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap text-[10px] font-mono text-zinc-400">{t.output}</pre>}
          </div>
        ))}
        {tests.length === 0 && <p className="text-xs text-ghost">No test results for this audit.</p>}
      </div>
    </section>
  )
}

function Severity({ severity }: { severity: string }) {
  const cls = severity === 'Critical' || severity === 'High' ? 'text-danger border-danger/40 bg-danger-muted'
    : severity === 'Medium' ? 'text-warn border-warn/40 bg-warn-muted'
    : 'text-info border-info/40 bg-surface'
  return <span className={`px-2 py-0.5 rounded-sm border text-[10px] font-mono ${cls}`}>{severity}</span>
}
