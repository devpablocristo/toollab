import { useState, useRef, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { api } from '../lib/api'
import type { RunSummary, RunMode, LLMReport, ProgressEvent, IntelIndex, IntelEndpointIndex, IntelEndpointDetail, EndpointScriptSet, PlaygroundResponse, AuthProfile } from '../lib/types'
import Spinner from '../components/Spinner'
import { ScoreRing, DonutChart, DonutLegend, HBarChart, StackedBar, SparkBars } from '../components/Charts'

const SCORE_COLORS: Record<string, string> = {
  security: '#ff4f5e',
  auth: '#ffb224',
  contract: '#52a8ff',
  robustness: '#a78bfa',
  performance: '#3dd68c',
  observability: '#22d3ee',
}

const SCORE_ICONS: Record<string, string> = {
  security: '\u{1f6e1}',
  auth: '\u{1f511}',
  contract: '\u{1f4cb}',
  robustness: '\u{1f4aa}',
  performance: '\u26a1',
  observability: '\u{1f441}',
}

function scoreToGrade(avg: number): string {
  if (avg >= 4.5) return 'A'
  if (avg >= 3.5) return 'B'
  if (avg >= 2.5) return 'C'
  if (avg >= 1.5) return 'D'
  return 'F'
}

export default function TargetDetail() {
  const { targetId } = useParams<{ targetId: string }>()
  const navigate = useNavigate()
  const { data: target, isLoading: tLoading } = useQuery({ queryKey: ['target', targetId], queryFn: () => api.targets.get(targetId!) })

  const [summary, setSummary] = useState<RunSummary | null>(null)
  const [runId, setRunId] = useState<string | null>(null)
  const [tab, setTab] = useState<'dashboard' | 'endpoints' | 'docs' | 'audit' | 'raw'>('dashboard')
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [logs, setLogs] = useState<ProgressEvent[]>([])
  const logRef = useRef<HTMLDivElement>(null)

  const { data: latestRun, isLoading: latestLoading } = useQuery({
    queryKey: ['latest-run', targetId],
    queryFn: () => api.targets.latestRun(targetId!),
    enabled: !!targetId && !runId && !running,
    retry: false,
  })

  useEffect(() => {
    if (latestRun?.run?.id && !runId && !running) {
      setRunId(latestRun.run.id)
      if (latestRun.run_summary) setSummary(latestRun.run_summary)
    }
  }, [latestRun, runId, running])

  useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight
  }, [logs])

  const startAnalysis = useCallback(() => {
    if (!targetId || running) return
    setRunning(true)
    setError(null)
    setLogs([])
    setSummary(null)
    setRunId(null)

    api.targets.analyzeSSE(targetId, (event) => {
      setLogs(prev => [...prev, event])
    }).then((result) => {
      setSummary(result.run_summary)
      setRunId(result.run_id)
      setRunning(false)
    }).catch((err) => {
      setError(err.message)
      setRunning(false)
    })
  }, [targetId, running])

  const maxRetries = 200
  const [docRetries, setDocRetries] = useState(0)
  const [docFailed, setDocFailed] = useState(false)
  const { data: documentation } = useQuery({
    queryKey: ['docs', runId],
    queryFn: async () => {
      try {
        return await api.runs.docs(runId!)
      } catch (e: any) {
        if ((e?.message ?? '').startsWith('503')) { setDocFailed(true); throw e }
        setDocRetries(prev => prev + 1)
        throw e
      }
    },
    enabled: !!runId && !docFailed,
    retry: false,
    refetchInterval: (q) => {
      if (q.state.data || docFailed || docRetries >= maxRetries) return false
      return 3000
    },
  })

  const [auditRetries, setAuditRetries] = useState(0)
  const [auditFailed, setAuditFailed] = useState(false)
  const { data: audit } = useQuery({
    queryKey: ['audit', runId],
    queryFn: async () => {
      try {
        return await api.runs.audit(runId!)
      } catch (e: any) {
        if ((e?.message ?? '').startsWith('503')) { setAuditFailed(true); throw e }
        setAuditRetries(prev => prev + 1)
        throw e
      }
    },
    enabled: !!runId && !auditFailed,
    retry: false,
    refetchInterval: (q) => {
      if (q.state.data || auditFailed || auditRetries >= maxRetries) return false
      return 3000
    },
  })

  const { data: endpointIndex } = useQuery({
    queryKey: ['endpoints', runId],
    queryFn: () => api.runs.endpointIndex(runId!),
    enabled: !!runId,
    retry: 3,
    retryDelay: 2000,
  })

  useEffect(() => {
    if (runId) {
      setDocRetries(0); setDocFailed(false)
      setAuditRetries(0); setAuditFailed(false)
    }
  }, [runId])

  if (tLoading || latestLoading) return <div className="p-8"><Spinner /></div>

  const lastLog = logs.length > 0 ? logs[logs.length - 1] : null

  return (
    <div className="p-8 max-w-7xl mx-auto animate-fade-in">
      <button onClick={() => navigate('/targets')}
        className="text-xs font-mono text-ghost hover:text-accent mb-5 flex items-center gap-1.5 transition-colors group">
        <svg xmlns="http://www.w3.org/2000/svg" className="h-3 w-3 group-hover:-translate-x-0.5 transition-transform" viewBox="0 0 20 20" fill="currentColor">
          <path fillRule="evenodd" d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z" clipRule="evenodd" />
        </svg>
        All Targets
      </button>

      <div className="mb-1">
        <h1 className="text-2xl font-display font-bold tracking-wide">{target?.name}</h1>
      </div>
      <div className="flex items-center gap-4 mb-8">
        <p className="text-xs font-mono text-ghost">{target?.source.type}: {target?.source.value}</p>
        {!running && (
          <button onClick={startAnalysis}
            className="text-xs font-mono px-3 py-1 rounded border border-accent/30 text-accent hover:bg-accent/10 transition-colors">
            {summary ? 'Re-analyze' : 'Analyze'}
          </button>
        )}
      </div>

      {!running && !summary && !error && (
        <div className="card p-8 text-center mb-8">
          <p className="text-ghost text-sm mb-4">No previous analysis found for this target.</p>
          <button onClick={startAnalysis}
            className="px-4 py-2 rounded bg-accent/20 text-accent font-mono text-sm hover:bg-accent/30 transition-colors">
            Start Analysis
          </button>
        </div>
      )}

      {running && (
        <div className="mb-8 card overflow-hidden animate-fade-up glow-accent">
          <div className="px-5 py-3 border-b border-edge flex items-center justify-between bg-surface/80">
            <div className="flex items-center gap-3">
              <Spinner />
              <span className="text-sm font-display font-semibold tracking-wide text-accent">
                {lastLog ? stepLabel(lastLog.step) : 'Initializing...'}
              </span>
            </div>
            {lastLog?.step && (
              <span className="text-xs text-ghost font-mono">{lastLog.phase}</span>
            )}
          </div>
          <div ref={logRef} className="max-h-72 overflow-y-auto p-4 font-mono text-xs leading-relaxed">
            {logs.map((log, i) => <LogLine key={i} event={log} />)}
          </div>
        </div>
      )}

      {error && (
        <div className="p-4 card border-danger/40 bg-danger-muted text-danger text-sm font-mono mb-6 glow-danger">
          {error}
        </div>
      )}

      {summary && (
        <>
          <RunModeBanner summary={summary} />
          <ScoreHeader summary={summary} />

          <div className="flex gap-1 mt-8 mb-6 border-b border-edge">
            <TabBtn active={tab === 'dashboard'} onClick={() => setTab('dashboard')}>Dashboard</TabBtn>
            <TabBtn active={tab === 'endpoints'} onClick={() => setTab('endpoints')}>
              Endpoints
              {endpointIndex && <span className="ml-1.5 text-[10px] font-mono text-ghost">{endpointIndex.total_endpoints}</span>}
            </TabBtn>
            <TabBtn active={tab === 'raw'} onClick={() => setTab('raw')}>
              Raw QA
            </TabBtn>
            <TabBtn active={tab === 'docs'} onClick={() => setTab('docs')}>
              Documentation
              {!documentation && !!runId && !docFailed && <span className="ml-2 inline-block w-1.5 h-1.5 rounded-full bg-accent animate-glow-pulse" />}
            </TabBtn>
            <TabBtn active={tab === 'audit'} onClick={() => setTab('audit')}>
              Audit
              {!audit && !!runId && !auditFailed && <span className="ml-2 inline-block w-1.5 h-1.5 rounded-full bg-accent animate-glow-pulse" />}
            </TabBtn>
          </div>

          {tab === 'dashboard' && <DashboardTab summary={summary} />}
          {tab === 'endpoints' && <EndpointsTab index={endpointIndex ?? null} runId={runId} />}
          {tab === 'raw' && <RawDataTab runId={runId} />}
          {tab === 'docs' && <DocsTab data={documentation ?? null} loading={!!runId && !documentation && !docFailed && docRetries < maxRetries} failed={docFailed || docRetries >= maxRetries} retries={docRetries} />}
          {tab === 'audit' && <AuditTab data={audit ?? null} loading={!!runId && !audit && !auditFailed && auditRetries < maxRetries} failed={auditFailed || auditRetries >= maxRetries} retries={auditRetries} />}
        </>
      )}
    </div>
  )
}

/* ─── Pipeline step labels ─── */

function stepLabel(step: string): string {
  const labels: Record<string, string> = {
    preflight: 'Target Normalization',
    discovery: 'AST Discovery',
    schema: 'Schema Inference',
    smoke: 'Baseline Smoke',
    auth_matrix: 'Auth Matrix',
    fuzz: 'Guided Fuzzing',
    logic: 'Business Logic',
    abuse: 'Abuse & Resilience',
    confirm: 'Confirmations',
    report: 'Report Builder',
  }
  return labels[step] ?? step
}

/* ─── Run Mode Banner ─── */

function RunModeBanner({ summary }: { summary: RunSummary }) {
  const mode = summary.run_mode
  if (!mode) return null

  const detail = summary.run_mode_detail
  const configs: Record<RunMode, { bg: string; border: string; text: string; icon: string; label: string }> = {
    offline: { bg: 'bg-danger-muted', border: 'border-danger/50', text: 'text-danger', icon: '\u26d4', label: 'SERVICE OFFLINE' },
    online_partial: { bg: 'bg-warn-muted', border: 'border-warn/40', text: 'text-warn', icon: '\u26a0\ufe0f', label: 'PARTIAL EVIDENCE' },
    online_good: { bg: 'bg-ok-muted', border: 'border-ok/30', text: 'text-ok', icon: '\u2705', label: 'ONLINE - GOOD EVIDENCE' },
    online_strong: { bg: 'bg-ok-muted', border: 'border-ok/40', text: 'text-ok', icon: '\u{1f4aa}', label: 'ONLINE - STRONG EVIDENCE' },
  }

  const cfg = configs[mode] ?? configs.online_good

  return (
    <div className={`mt-4 mb-2 p-4 rounded-sm border ${cfg.bg} ${cfg.border} animate-fade-up`}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="text-lg">{cfg.icon}</span>
          <div>
            <p className={`text-xs font-display font-bold tracking-wider uppercase ${cfg.text}`}>{cfg.label}</p>
            {detail?.reason && <p className="text-xs font-body text-ghost mt-0.5">{detail.reason}</p>}
          </div>
        </div>
        {detail && (
          <div className="flex gap-4 text-[10px] font-mono text-ghost">
            <span>{detail.http_responses} HTTP responses</span>
            <span>{detail.happy_path_endpoints} happy endpoints</span>
            {detail.connection_errors > 0 && <span className="text-danger">{detail.connection_errors} conn errors</span>}
          </div>
        )}
      </div>
      {mode === 'offline' && (
        <div className="mt-3 p-3 bg-obsidian/50 rounded-sm border border-danger/20">
          <p className="text-xs font-display font-semibold text-danger mb-1">Consequences of OFFLINE mode:</p>
          <ul className="text-xs font-body text-ghost space-y-0.5 list-disc list-inside">
            <li>Documentation shows AST-only data + how to make it run</li>
            <li>Audit scores are N/A - API is not auditable without runtime evidence</li>
            <li>No operational flows or expected status codes generated</li>
            <li>Use the Try It playground to test connectivity manually</li>
          </ul>
        </div>
      )}
      {mode === 'online_partial' && (
        <p className="text-[10px] font-body text-warn mt-2">
          Scores and documentation have LOW confidence. Collect more evidence for reliable results.
        </p>
      )}
    </div>
  )
}

function LogLine({ event }: { event: ProgressEvent }) {
  const stepColors: Record<string, string> = {
    preflight: 'text-ghost', discovery: 'text-info', schema: 'text-purple-400',
    smoke: 'text-accent-dim', auth_matrix: 'text-warn', fuzz: 'text-danger/80',
    logic: 'text-purple-400', abuse: 'text-danger', confirm: 'text-ok', report: 'text-accent',
  }
  return (
    <div className={`py-0.5 ${event.phase === 'error' ? 'text-danger' : stepColors[event.step] ?? 'text-ghost'}`}>
      <span className="text-ghost-faint mr-2">[{event.step}]</span>
      {event.message}
    </div>
  )
}

function TabBtn({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button onClick={onClick} className={`tab-btn ${active ? 'tab-btn-active' : 'tab-btn-inactive'}`}>
      {children}
    </button>
  )
}

/* ─── Score Header ─── */

function ScoreHeader({ summary }: { summary: RunSummary }) {
  const isOffline = summary.run_mode === 'offline'
  const scoresAvailable = summary.scores_available && !isOffline
  const scores = scoresAvailable ? (summary.scores ?? {}) : {}
  const dims = Object.entries(scores)
  const avg = dims.length > 0 ? dims.reduce((a, [, v]) => a + v, 0) / dims.length : 0
  const overall = scoresAvailable ? Math.round(avg * 20) : 0
  const grade = scoresAvailable ? scoreToGrade(avg) : 'N/A'

  return (
    <div className="mt-6 space-y-4 animate-fade-up">
      <div className="grid grid-cols-4 gap-4">
        <div className="p-5 card flex flex-col items-center justify-center stagger-1 animate-fade-up">
          {scoresAvailable ? (
            <>
              <ScoreRing score={overall} grade={grade} size={130} stroke={12} />
              <span className="text-[10px] font-display font-semibold tracking-wider text-ghost mt-3 uppercase">Overall ({grade})</span>
            </>
          ) : (
            <div className="flex flex-col items-center justify-center h-[130px]">
              <span className="text-3xl font-display font-bold text-ghost-faint">N/A</span>
              <span className="text-[10px] font-display font-semibold tracking-wider text-ghost mt-2 uppercase">
                {isOffline ? 'Not Auditable' : 'Insufficient Data'}
              </span>
            </div>
          )}
        </div>
        <div className="p-5 card flex flex-col gap-3 stagger-2 animate-fade-up col-span-2">
          {scoresAvailable ? (
            <>
              <div className="flex items-center gap-2">
                <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase">Dimensional Scores (0-5)</p>
                {summary.run_mode === 'online_partial' && <Badge color="warn">Low Confidence</Badge>}
              </div>
              <div className="space-y-2">
                {dims.map(([dim, score]) => (
                  <div key={dim} className="flex items-center gap-2">
                    <span className="text-sm w-5 text-center">{SCORE_ICONS[dim] ?? '?'}</span>
                    <span className="text-xs font-mono w-28 text-ghost capitalize">{dim}</span>
                    <div className="flex-1 bg-surface rounded-sm overflow-hidden h-4">
                      <div className="h-full rounded-sm transition-all duration-700 flex items-center pl-2"
                        style={{ width: `${(score / 5) * 100}%`, background: `linear-gradient(90deg, ${SCORE_COLORS[dim] ?? '#52a8ff'}80, ${SCORE_COLORS[dim] ?? '#52a8ff'})` }}>
                        {score >= 1 && <span className="text-[10px] font-mono font-bold text-obsidian/80">{score.toFixed(1)}</span>}
                      </div>
                    </div>
                    {score < 1 && <span className="text-xs font-mono text-ghost w-8">{score.toFixed(1)}</span>}
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="flex flex-col items-center justify-center h-full">
              <p className="text-xs font-display text-ghost-faint text-center">
                {isOffline
                  ? 'API scores cannot be computed while the service is offline. The run environment itself is evaluated, not the API quality.'
                  : 'Insufficient runtime evidence to produce reliable scores. Run more probes or use the Try It playground.'}
              </p>
            </div>
          )}
        </div>
        <div className="p-5 card flex flex-col justify-center gap-3 stagger-3 animate-fade-up">
          <KV label="Endpoints (AST)" value={summary.endpoints_discovered_ast} />
          <KV label="Confirmed (RT)" value={summary.endpoints_confirmed_runtime} />
          <KV label="Coverage" value={`${summary.coverage_pct.toFixed(0)}%`} />
          <KV label="Evidence" value={summary.evidence_count_full} />
          <KV label="Duration" value={`${summary.duration_seconds}s`} />
          <KV label="Run Mode" value={runModeLabel(summary.run_mode)} />
        </div>
      </div>
    </div>
  )
}

function runModeLabel(mode: RunMode): string {
  switch (mode) {
    case 'offline': return 'OFFLINE'
    case 'online_partial': return 'PARTIAL'
    case 'online_good': return 'GOOD'
    case 'online_strong': return 'STRONG'
    default: return mode ?? 'Unknown'
  }
}

function KV({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase">{label}</span>
      <span className="text-sm font-mono font-bold text-zinc-200">{value}</span>
    </div>
  )
}

/* ─── Dashboard Tab ─── */

function DashboardTab({ summary }: { summary: RunSummary }) {
  const findings = summary.top_findings ?? []
  const budget = summary.budget_usage

  return (
    <div className="space-y-8 animate-fade-in">
      {findings.length > 0 ? (
        <Section title={`Top Findings (${findings.length})`}>
          <div className="space-y-2">
            {findings.map((f, i) => (
              <div key={i} className="p-4 card">
                <div className="flex items-center gap-2 mb-1.5">
                  <SeverityBadge severity={f.severity} />
                  <span className="text-xs font-mono text-ghost-faint">{f.id}</span>
                </div>
                <p className="text-sm font-body text-zinc-300">{f.title}</p>
                {(f.evidence_refs?.length ?? 0) > 0 && (
                  <p className="text-[10px] font-mono text-ghost-faint mt-1">{f.evidence_refs!.length} evidence refs</p>
                )}
              </div>
            ))}
          </div>
        </Section>
      ) : (
        <div className="p-8 card border-ok/20 bg-ok-muted text-center">
          <p className="text-sm text-ok font-display font-semibold">No critical findings detected</p>
        </div>
      )}

      {budget && (
        <Section title="Budget Usage">
          <div className="grid grid-cols-2 gap-4">
            <div className="p-4 card">
              <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Total Requests</p>
              <p className="text-2xl font-display font-bold text-zinc-200">{budget.requests_total}</p>
              <p className="text-xs font-mono text-ghost mt-1">in {budget.duration_seconds}s</p>
            </div>
            <div className="p-4 card">
              <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">By Category</p>
              <div className="space-y-1.5">
                {Object.entries(budget.by_category ?? {}).sort((a, b) => b[1] - a[1]).map(([cat, count]) => (
                  <div key={cat} className="flex items-center gap-2">
                    <span className="text-xs font-mono w-24 text-ghost capitalize">{cat.replace('_', ' ')}</span>
                    <div className="flex-1 bg-surface rounded-sm overflow-hidden h-3">
                      <div className="h-full rounded-sm" style={{ width: `${budget.requests_total > 0 ? (count / budget.requests_total) * 100 : 0}%`, background: 'linear-gradient(90deg, #52a8ff80, #52a8ff)' }} />
                    </div>
                    <span className="text-[10px] font-mono text-ghost w-10 text-right">{count}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </Section>
      )}

      <Section title="Run Info">
        <div className="p-4 card grid grid-cols-3 gap-4">
          <div>
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Status</p>
            <StatusBadge status={summary.status} />
          </div>
          <div>
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Run ID</p>
            <p className="text-xs font-mono text-zinc-300">{summary.run_id.slice(0, 12)}...</p>
          </div>
          <div>
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Duration</p>
            <p className="text-xs font-mono text-zinc-300">{summary.duration_seconds}s</p>
          </div>
        </div>
      </Section>
    </div>
  )
}

function RawDataTab({ runId }: { runId: string | null }) {
  const [search, setSearch] = useState('')
  const [rowLimit, setRowLimit] = useState(120)
  const { data, isLoading, error } = useQuery({
    queryKey: ['raw-dossier-full', runId],
    queryFn: () => api.runs.artifact(runId!, 'dossier_full') as Promise<any>,
    enabled: !!runId,
    retry: 1,
  })

  if (!runId) return <div className="p-8 card text-xs text-ghost">No run selected.</div>
  if (isLoading) return <LoadingState label="raw dossier" retries={0} />
  if (error || !data) return <FailedState label="raw dossier" />

  const runtime = data.runtime ?? {}
  const scoring = data.scoring ?? {}
  const summary = data.run_summary ?? {}
  const steps = data.step_results ?? []
  const metrics = runtime.derived_metrics ?? {}
  const profile = data.target_profile ?? {}
  const auth = runtime.auth_matrix ?? {}
  const smoke = runtime.smoke_results ?? []
  const fuzz = runtime.fuzz_results ?? []
  const logic = runtime.logic_results ?? []
  const abuse = runtime.abuse_results ?? []
  const confirmations = data.confirmations ?? []
  const signatures = runtime.error_signatures ?? []
  const findings = data.findings_raw ?? []

  const smokePassed = smoke.filter((x: any) => x.passed).length
  const fuzzCrashes = fuzz.filter((x: any) => x.crashed).length
  const logicAnomalies = logic.filter((x: any) => x.anomaly).length
  const rateLimitTests = abuse.filter((x: any) => x.test_type === 'rate_limit').length
  const abuseRateLimit = abuse.filter((x: any) => x.test_type === 'rate_limit' && x.got_429).length
  const confirmed = confirmations.filter((x: any) => x.classification === 'confirmed').length
  const authDenied = (auth.entries ?? []).filter((x: any) => x.no_auth === 'denied').length
  const authAllowed = (auth.entries ?? []).filter((x: any) => x.no_auth === 'allowed').length

  const filteredSmoke = filterRows(smoke.slice(0, rowLimit).map((s: any) => ({
    method: s.method,
    path: s.path,
    passed: String(s.passed),
    status_code: s.status_code ?? '',
    evidence_refs: (s.evidence_refs ?? []).join(', '),
    block_reason: s.block_reason ?? '',
  })), search)

  const filteredAuth = filterRows((auth.entries ?? []).slice(0, rowLimit).map((a: any) => ({
    method: a.method,
    path: a.path,
    no_auth: a.no_auth,
    invalid_auth: a.invalid_auth,
    valid_auth: a.valid_auth ?? '',
    evidence_refs: (a.evidence_refs ?? []).join(', '),
  })), search)

  const filteredFuzz = filterRows(fuzz.slice(0, rowLimit).map((f: any) => ({
    endpoint_id: f.endpoint_id,
    category: f.category,
    sub_category: f.sub_category ?? '',
    status: f.status ?? '',
    crashed: String(!!f.crashed),
    timeout: String(!!f.timeout),
    input_desc: f.input_desc,
    evidence_ref: f.evidence_ref,
  })), search)

  const filteredLogic = filterRows(logic.slice(0, rowLimit).map((l: any) => ({
    endpoint_id: l.endpoint_id,
    test_type: l.test_type,
    passed: String(!!l.passed),
    anomaly: String(!!l.anomaly),
    description: l.description,
    evidence_refs: (l.evidence_refs ?? []).join(', '),
  })), search)

  const filteredAbuse = filterRows(abuse.slice(0, rowLimit).map((a: any) => ({
    endpoint_id: a.endpoint_id,
    test_type: a.test_type,
    burst_size: a.burst_size ?? '',
    got_429: String(!!a.got_429),
    degraded: String(!!a.degraded),
    crashed: String(!!a.crashed),
    evidence_refs: (a.evidence_refs ?? []).join(', '),
  })), search)

  const filteredFindings = filterRows(findings.slice(0, rowLimit).map((f: any) => ({
    finding_id: f.finding_id,
    severity: f.severity,
    category: f.category,
    taxonomy_id: f.taxonomy_id,
    classification: f.classification,
    confidence: f.confidence,
    title: f.title,
    evidence_refs: (f.evidence_refs ?? []).join(', '),
  })), search)

  const statusHistEntries = Object.entries(metrics.status_histogram ?? {})
    .map(([k, v]) => ({ status: Number(k), count: Number(v) }))
    .sort((a, b) => a.status - b.status)
  const statusBars = statusHistEntries.map((x) => ({
    label: String(x.status),
    value: x.count,
    color: x.status >= 500 ? '#ff4f5e' : x.status >= 400 ? '#ffb224' : '#3dd68c',
  }))
  const statusSparkValues = statusHistEntries.map((x) => x.count)
  const statusSparkColors = statusHistEntries.map((x) => x.status >= 500 ? '#ff4f5e' : x.status >= 400 ? '#ffb224' : '#3dd68c')

  const severityCount = findings.reduce((acc: Record<string, number>, f: any) => {
    const sev = (f.severity ?? 'info').toLowerCase()
    acc[sev] = (acc[sev] ?? 0) + 1
    return acc
  }, {})
  const severityBars = ['critical', 'high', 'medium', 'low', 'info'].map((sev) => ({
    label: sev,
    value: severityCount[sev] ?? 0,
    color: sev === 'critical' ? '#ff2d55' : sev === 'high' ? '#ff4f5e' : sev === 'medium' ? '#ffb224' : sev === 'low' ? '#52a8ff' : '#8892b0',
  }))

  const validationDonut = [
    { label: 'Smoke OK', value: smokePassed, color: '#3dd68c' },
    { label: 'Smoke Fail', value: Math.max(smoke.length - smokePassed, 0), color: '#ffb224' },
    { label: 'Fuzz Crash', value: fuzzCrashes, color: '#ff4f5e' },
    { label: 'Logic Anomaly', value: logicAnomalies, color: '#a78bfa' },
  ]
  const authStack = [
    { label: 'Denied', value: authDenied, color: '#3dd68c' },
    { label: 'Allowed', value: authAllowed, color: '#ff4f5e' },
    { label: 'Unknown', value: Math.max((auth.entries ?? []).length - authDenied - authAllowed, 0), color: '#8892b0' },
  ]

  return (
    <div className="space-y-6 animate-fade-in">
      <Section title="Raw QA Overview">
        <div className="card p-4 space-y-4">
          <div className="grid grid-cols-4 gap-3">
            <RawKpi label="Total Requests" value={metrics.total_requests} />
            <RawKpi label="Success Rate" value={pct(metrics.success_rate)} />
            <RawKpi label="Error Rate" value={pct(metrics.error_rate)} />
            <RawKpi label="Coverage" value={pct(metrics.coverage_pct / 100)} />
            <RawKpi label="Useful Coverage" value={metrics.useful_coverage_pct != null ? pct(metrics.useful_coverage_pct / 100) : 'N/A'} />
            <RawKpi label="P50 / P95 / P99" value={`${metrics.p50_ms ?? 0} / ${metrics.p95_ms ?? 0} / ${metrics.p99_ms ?? 0} ms`} />
            <RawKpi label="Endpoints Tested" value={`${metrics.endpoints_tested ?? 0}/${metrics.endpoints_total ?? 0}`} />
            <RawKpi label="Evidence Count" value={summary.evidence_count_full ?? 0} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <ValidationBar label="Smoke Pass Rate" ok={smokePassed} total={smoke.length} goodWhenHigh />
            <ValidationBar label="Protected Endpoints Denied w/o Auth" ok={authDenied} total={(auth.entries ?? []).length} goodWhenHigh />
            <ValidationBar label="Fuzz Stability (no crash)" ok={fuzz.length - fuzzCrashes} total={fuzz.length} goodWhenHigh />
            <ValidationBar label="Logic Stability (no anomaly)" ok={logic.length - logicAnomalies} total={logic.length} goodWhenHigh />
            <ValidationBar label="Rate Limit Detection (429 seen)" ok={abuseRateLimit} total={rateLimitTests} goodWhenHigh />
            <ValidationBar label="Finding Confirmation Rate" ok={confirmed} total={confirmations.length} goodWhenHigh />
          </div>
        </div>
      </Section>

      <Section title="Graphs">
        <div className="grid grid-cols-3 gap-3">
          <div className="card p-4">
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Validation Mix</p>
            <div className="flex items-center gap-4">
              <DonutChart segments={validationDonut} size={120} stroke={14}>
                <span className="text-xs font-mono text-zinc-200">{summary.evidence_count_full ?? 0}</span>
                <span className="text-[10px] font-mono text-ghost">evidence</span>
              </DonutChart>
              <div className="flex-1">
                <DonutLegend segments={validationDonut} />
              </div>
            </div>
          </div>
          <div className="card p-4">
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Auth Matrix Distribution</p>
            <StackedBar segments={authStack} />
            <div className="mt-2">
              <DonutLegend segments={authStack.map(s => ({ ...s }))} />
            </div>
          </div>
          <div className="card p-4">
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Latency (ms)</p>
            <HBarChart
              bars={[
                { label: 'P50', value: Number(metrics.p50_ms ?? 0), color: '#52a8ff' },
                { label: 'P95', value: Number(metrics.p95_ms ?? 0), color: '#ffb224' },
                { label: 'P99', value: Number(metrics.p99_ms ?? 0), color: '#ff4f5e' },
              ]}
              height={18}
            />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3 mt-3">
          <div className="card p-4">
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Status Histogram</p>
            {statusBars.length > 0 ? (
              <>
                <HBarChart bars={statusBars} height={16} />
                <div className="mt-2 flex items-center gap-2">
                  <span className="text-[10px] font-mono text-ghost">spark:</span>
                  <SparkBars values={statusSparkValues} colors={statusSparkColors} />
                </div>
              </>
            ) : (
              <p className="text-xs font-mono text-ghost">No status histogram data.</p>
            )}
          </div>
          <div className="card p-4">
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Findings by Severity</p>
            <HBarChart bars={severityBars} height={16} />
          </div>
        </div>
      </Section>

      <Section title="Data Controls">
        <div className="card p-4 flex items-center gap-3">
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filtrar tablas (path, endpoint_id, finding, status...)"
            className="flex-1 bg-surface border border-edge rounded-sm px-3 py-1.5 text-xs font-mono text-zinc-200 placeholder:text-ghost-faint focus:outline-none focus:border-accent"
          />
          <label className="text-xs font-mono text-ghost">
            Rows:
            <select
              value={rowLimit}
              onChange={(e) => setRowLimit(Number(e.target.value))}
              className="ml-2 bg-surface border border-edge rounded-sm px-2 py-1 text-xs font-mono text-zinc-200"
            >
              <option value={60}>60</option>
              <option value={120}>120</option>
              <option value={240}>240</option>
              <option value={500}>500</option>
            </select>
          </label>
        </div>
      </Section>

      <Section title="Execution Steps">
        <RawTable
          columns={['step', 'status', 'duration_ms', 'budget_used', 'error']}
          rows={steps.map((s: any) => ({
            step: s.step,
            status: s.status,
            duration_ms: s.duration_ms,
            budget_used: s.budget_used,
            error: s.error ?? '',
          }))}
          search={search}
        />
      </Section>

      <Section title="Core Validations">
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Smoke Results ({filteredSmoke.length})</summary>
          <div className="mt-3">
            <RawTable columns={['method', 'path', 'passed', 'status_code', 'evidence_refs', 'block_reason']} rows={filteredSmoke} />
          </div>
        </details>
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Auth Matrix ({filteredAuth.length})</summary>
          <div className="mt-3">
            <RawTable columns={['method', 'path', 'no_auth', 'invalid_auth', 'valid_auth', 'evidence_refs']} rows={filteredAuth} />
          </div>
        </details>
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Fuzz Results ({filteredFuzz.length})</summary>
          <div className="mt-3">
            <RawTable columns={['endpoint_id', 'category', 'sub_category', 'status', 'crashed', 'timeout', 'input_desc', 'evidence_ref']} rows={filteredFuzz} />
          </div>
        </details>
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Logic Results ({filteredLogic.length})</summary>
          <div className="mt-3">
            <RawTable columns={['endpoint_id', 'test_type', 'passed', 'anomaly', 'description', 'evidence_refs']} rows={filteredLogic} />
          </div>
        </details>
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Abuse Results ({filteredAbuse.length})</summary>
          <div className="mt-3">
            <RawTable columns={['endpoint_id', 'test_type', 'burst_size', 'got_429', 'degraded', 'crashed', 'evidence_refs']} rows={filteredAbuse} />
          </div>
        </details>
      </Section>

      <Section title="Findings & Signals">
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Findings Raw ({filteredFindings.length})</summary>
          <div className="mt-3">
            <RawTable columns={['finding_id', 'severity', 'category', 'taxonomy_id', 'classification', 'confidence', 'title', 'evidence_refs']} rows={filteredFindings} />
          </div>
        </details>
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Error Signatures ({signatures.length})</summary>
          <div className="mt-3">
            <RawTable
              columns={['signature_id', 'status', 'content_type', 'count', 'pattern', 'sample_evidence_refs']}
              rows={filterRows(signatures.map((s: any) => ({
                signature_id: s.signature_id,
                status: s.status,
                content_type: s.content_type,
                count: s.count,
                pattern: s.pattern,
                sample_evidence_refs: (s.sample_evidence_refs ?? []).join(', '),
              })), search)}
            />
          </div>
        </details>
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Confirmations ({confirmations.length})</summary>
          <div className="mt-3">
            <RawTable
              columns={['finding_id', 'classification', 'original_evidence_ref', 'replay_evidence_ref', 'variation_evidence_ref', 'notes']}
              rows={filterRows(confirmations.map((c: any) => ({
                finding_id: c.finding_id,
                classification: c.classification,
                original_evidence_ref: c.original_evidence_ref,
                replay_evidence_ref: c.replay_evidence_ref,
                variation_evidence_ref: c.variation_evidence_ref ?? '',
                notes: c.notes ?? '',
              })), search)}
            />
          </div>
        </details>
      </Section>

      <Section title="Raw JSON (Expand on demand)">
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Run / Preflight JSON</summary>
          <RawJson data={{
            run_mode: data.run_mode,
            run_mode_detail: data.run_mode_detail,
            target_profile: profile,
            openapi_validation: metrics.openapi_validation,
            budget_usage: summary.budget_usage,
          }} />
        </details>
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Derived Metrics JSON</summary>
          <RawJson data={metrics} />
        </details>
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Scoring JSON</summary>
          <RawJson data={scoring} />
        </details>
        <details className="card p-3">
          <summary className="cursor-pointer text-xs font-display font-semibold tracking-wider text-ghost uppercase">Status Histogram JSON</summary>
          <RawJson data={metrics.status_histogram ?? {}} />
        </details>
      </Section>
    </div>
  )
}

function RawKpi({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="p-3 bg-obsidian/40 border border-edge rounded-sm">
      <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">{label}</p>
      <p className="text-sm font-mono text-zinc-100 break-all">{value}</p>
    </div>
  )
}

function ValidationBar({ label, ok, total, goodWhenHigh }: { label: string; ok: number; total: number; goodWhenHigh: boolean }) {
  const ratio = total > 0 ? ok / total : 0
  const pctValue = Math.round(ratio * 100)
  const good = goodWhenHigh ? ratio >= 0.7 : ratio <= 0.3
  const color = good ? '#3dd68c' : ratio >= 0.4 ? '#ffb224' : '#ff4f5e'
  return (
    <div className="p-3 bg-obsidian/40 border border-edge rounded-sm">
      <div className="flex items-center justify-between mb-1">
        <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase">{label}</p>
        <span className="text-[10px] font-mono text-zinc-300">{ok}/{total}</span>
      </div>
      <div className="h-2 bg-surface rounded-sm overflow-hidden">
        <div className="h-full rounded-sm transition-all duration-500" style={{ width: `${pctValue}%`, backgroundColor: color }} />
      </div>
      <p className="text-[10px] font-mono text-ghost mt-1">{pctValue}%</p>
    </div>
  )
}

function RawJson({ data }: { data: unknown }) {
  return (
    <pre className="mt-3 p-4 bg-obsidian/40 border border-edge rounded-sm text-xs font-mono text-zinc-300 overflow-auto max-h-[28rem] whitespace-pre-wrap">
      {JSON.stringify(data, null, 2)}
    </pre>
  )
}

function RawTable({ columns, rows, search }: { columns: string[]; rows: Record<string, any>[]; search?: string }) {
  const displayRows = search ? filterRows(rows, search) : rows
  if (!displayRows || displayRows.length === 0) return <div className="p-4 bg-obsidian/40 border border-edge rounded-sm text-xs text-ghost">No data.</div>
  return (
    <div className="bg-obsidian/40 border border-edge rounded-sm overflow-auto max-h-[28rem]">
      <table className="w-full text-xs font-mono">
        <thead>
          <tr className="text-ghost bg-surface/80">
            {columns.map((c) => (
              <th key={c} className="text-left py-2 px-3 whitespace-nowrap">{c}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {displayRows.map((r, i) => (
            <tr key={i} className="border-t border-edge/50 align-top">
              {columns.map((c) => (
                <td key={c} className="py-1.5 px-3 text-zinc-300 break-all">{String(r[c] ?? '')}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function filterRows(rows: Record<string, any>[], query: string): Record<string, any>[] {
  const q = query.trim().toLowerCase()
  if (!q) return rows
  return rows.filter((r) => Object.values(r).some((v) => String(v ?? '').toLowerCase().includes(q)))
}

function pct(v?: number): string {
  if (v == null || Number.isNaN(v)) return '0.00%'
  return `${(v * 100).toFixed(2)}%`
}

/* ─── Markdown Renderer ─── */

function MarkdownDoc({ content }: { content: string }) {
  return (
    <div className="animate-fade-in">
      <article className="prose prose-invert prose-sm max-w-none
        prose-headings:font-display prose-headings:tracking-tight
        prose-h1:text-2xl prose-h1:text-accent prose-h1:border-b prose-h1:border-edge prose-h1:pb-3 prose-h1:mb-6
        prose-h2:text-lg prose-h2:text-zinc-200 prose-h2:mt-8 prose-h2:mb-3
        prose-h3:text-sm prose-h3:text-zinc-300 prose-h3:mt-5 prose-h3:mb-2
        prose-p:text-sm prose-p:text-zinc-400 prose-p:leading-relaxed
        prose-a:text-accent prose-a:no-underline hover:prose-a:underline
        prose-code:text-xs prose-code:text-accent prose-code:bg-surface prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded-sm prose-code:border prose-code:border-edge
        prose-pre:bg-obsidian prose-pre:border prose-pre:border-edge prose-pre:rounded-md
        prose-table:text-xs
        prose-th:text-ghost prose-th:bg-surface/80 prose-th:font-display prose-th:tracking-wide prose-th:uppercase prose-th:text-[10px] prose-th:py-2 prose-th:px-3
        prose-td:py-1.5 prose-td:px-3 prose-td:text-zinc-300 prose-td:border-edge
        prose-strong:text-zinc-200
        prose-li:text-sm prose-li:text-zinc-400
        prose-blockquote:border-accent/40 prose-blockquote:text-zinc-400
      ">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
      </article>
    </div>
  )
}

/* ─── Documentation Tab ─── */

function DocsTab({ data, loading, failed, retries }: { data: LLMReport | null; loading: boolean; failed: boolean; retries: number }) {
  if (failed) return <FailedState label="documentation" />
  if (loading || !data) return <LoadingState label="documentation" retries={retries} />

  const d = data as any

  if (d.format === 'markdown' && d.content) {
    return <MarkdownDoc content={d.content} />
  }

  const svc = d.service_identity
  const arch = d.architecture_from_ast
  const qs = d.quickstart
  const auth = d.auth
  const resources = d.resources ?? []
  const models = d.data_models ?? []
  const tours = d.guided_tour ?? []
  const playbook = d.testing_playbook
  const facts = d.facts ?? []
  const questions = d.open_questions ?? []

  return (
    <div className="space-y-8 animate-fade-in">
      {svc && (
        <Section title={svc.service_name ?? 'Service Identity'}>
          <div className="p-5 card space-y-3">
            <div className="flex flex-wrap gap-2">
              {svc.framework && <Badge color="info">{svc.framework}</Badge>}
              {svc.domain && <Badge color="ghost">{svc.domain}</Badge>}
              {svc.versioning && <Badge color="ghost">{svc.versioning}</Badge>}
            </div>
            {svc.intended_consumers && <p className="text-xs font-body text-ghost">{svc.intended_consumers}</p>}
            {(svc.base_paths?.length ?? 0) > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-2">
                {svc.base_paths.map((p: string, i: number) => (
                  <span key={i} className="px-2 py-0.5 rounded-sm text-[10px] font-mono bg-surface border border-edge text-zinc-300">{p}</span>
                ))}
              </div>
            )}
            {svc.content_types && (
              <div className="flex gap-4 text-[10px] font-mono text-ghost-faint mt-1">
                {svc.content_types.consumes?.length > 0 && <span>Consumes: {svc.content_types.consumes.join(', ')}</span>}
                {svc.content_types.produces?.length > 0 && <span>Produces: {svc.content_types.produces.join(', ')}</span>}
              </div>
            )}
          </div>
        </Section>
      )}

      {arch && (
        <Section title="Architecture (from AST)">
          {(arch.route_groups ?? []).map((g: any, i: number) => (
            <div key={i} className="p-4 card mb-2">
              <div className="flex items-center gap-2 mb-1">
                <span className="text-sm font-mono font-bold text-accent">{g.group_prefix}</span>
                <span className="text-[10px] font-mono text-ghost">{g.endpoints_count} endpoints</span>
                {(g.middlewares?.length ?? 0) > 0 && <Badge color="warn">{g.middlewares.join(', ')}</Badge>}
              </div>
              {g.notes && <p className="text-xs font-body text-ghost leading-relaxed">{g.notes}</p>}
            </div>
          ))}
          {arch.auth_and_middleware_notes && (
            <div className="p-4 card mt-2">
              <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Auth & Middleware Notes</p>
              <p className="text-xs font-body text-ghost leading-relaxed">{arch.auth_and_middleware_notes}</p>
            </div>
          )}
          {(arch.discrepancies?.length ?? 0) > 0 && (
            <div className="mt-2 space-y-2">
              {arch.discrepancies.map((disc: any, i: number) => (
                <div key={i} className="p-4 card border-warn/30">
                  <p className="text-xs font-body text-warn mb-1">{disc.description}</p>
                  {disc.impact && <p className="text-xs font-body text-ghost">{disc.impact}</p>}
                </div>
              ))}
            </div>
          )}
        </Section>
      )}

      {qs && (
        <Section title="Quickstart">
          <div className="p-5 card space-y-3">
            {qs.base_url && <p className="text-xs font-mono text-accent">{qs.base_url}</p>}
            {qs.auth_setup && <p className="text-xs font-body text-ghost leading-relaxed">{qs.auth_setup}</p>}
            {(qs.smoke_test_steps ?? []).map((s: any, i: number) => (
              <div key={i} className="p-3 bg-obsidian rounded-sm border border-edge/50">
                <div className="flex items-center gap-2 mb-1">
                  <span className="text-[10px] font-mono font-bold text-accent w-5">{s.step}.</span>
                  <span className="text-xs font-body text-zinc-300">{s.goal}</span>
                </div>
                {s.request_ref && <span className="text-[10px] font-mono text-ghost-faint">ref: {s.request_ref}</span>}
                {s.expected && <p className="text-[10px] font-mono text-ghost mt-1">{s.expected}</p>}
              </div>
            ))}
          </div>
        </Section>
      )}

      {auth && (
        <Section title="Authentication">
          <div className="p-5 card space-y-3">
            {auth.how_to_authenticate && <p className="text-xs font-body text-zinc-300 leading-relaxed">{auth.how_to_authenticate}</p>}
            {(auth.observed_mechanisms ?? []).length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {auth.observed_mechanisms.map((m: string, i: number) => <Badge key={i} color="warn">{m}</Badge>)}
              </div>
            )}
            {(auth.auth_matrix_summary ?? []).length > 0 && (
              <div className="overflow-x-auto mt-2">
                <table className="w-full text-xs">
                  <thead><tr className="text-ghost bg-surface/80 font-display tracking-wide uppercase text-[10px]">
                    <th className="text-left py-2 px-3">Method</th>
                    <th className="text-left py-2 px-3">Path</th>
                    <th className="text-center py-2 px-3">Auth Required</th>
                  </tr></thead>
                  <tbody>
                    {auth.auth_matrix_summary.slice(0, 10).map((row: any, i: number) => (
                      <tr key={i} className="border-t border-edge/50">
                        <td className="py-1.5 px-3 font-mono text-info">{row.method}</td>
                        <td className="py-1.5 px-3 font-mono text-zinc-300">{row.path}</td>
                        <td className="py-1.5 px-3 text-center">
                          <span className={`text-[10px] font-mono ${row.requires_auth === 'yes' ? 'text-warn' : row.requires_auth === 'no' ? 'text-ok' : 'text-ghost'}`}>
                            {row.requires_auth}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {(auth.open_questions ?? []).length > 0 && (
              <div className="space-y-1 mt-2">
                {auth.open_questions.map((q: any, i: number) => (
                  <p key={i} className="text-xs font-body text-ghost-faint">? {q.question ?? q}</p>
                ))}
              </div>
            )}
          </div>
        </Section>
      )}

      {resources.length > 0 && (
        <Section title={`API Resources (${resources.length})`}>
          <div className="space-y-4">
            {resources.map((res: any, i: number) => (
              <div key={i} className="card overflow-hidden">
                <div className="px-5 py-3 border-b border-edge bg-surface/60">
                  <h4 className="text-sm font-display font-semibold text-accent">{res.name}</h4>
                  {res.purpose && <p className="text-xs font-body text-ghost mt-1">{res.purpose}</p>}
                </div>
                <div className="divide-y divide-edge/50">
                  {(res.endpoints ?? []).map((ep: any, j: number) => (
                    <div key={j} className="px-5 py-3">
                      <div className="flex items-center gap-2 mb-1">
                        <MethodBadge method={ep.method} />
                        <span className="text-xs font-mono text-zinc-300">{ep.path}</span>
                      </div>
                      {ep.what_it_does && <p className="text-xs font-body text-ghost leading-relaxed">{ep.what_it_does}</p>}
                      {(ep.common_errors ?? []).length > 0 && (
                        <div className="flex flex-wrap gap-1 mt-1.5">
                          {ep.common_errors.map((e: any, k: number) => (
                            <span key={k} className="text-[10px] font-mono px-1.5 py-0.5 rounded-sm bg-danger-muted text-danger border border-danger/20">
                              {typeof e === 'string' ? e : `${e.status}: ${e.description ?? e.reason ?? ''}`}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}

      {models.length > 0 && (
        <Section title={`Data Models (${models.length})`}>
          <div className="grid grid-cols-2 gap-3">
            {models.map((m: any, i: number) => (
              <div key={i} className="p-4 card">
                <h4 className="text-sm font-display font-semibold text-accent mb-1">{m.name}</h4>
                {m.business_role && <p className="text-xs font-body text-ghost mb-2">{m.business_role}</p>}
                <div className="space-y-1">
                  {(m.fields ?? []).map((f: any, j: number) => (
                    <div key={j} className="flex items-center gap-2 text-xs">
                      <span className="text-zinc-300 font-mono">{f.name}</span>
                      <span className="px-1.5 py-0.5 rounded-sm bg-surface text-ghost text-[10px] font-mono">{f.type}</span>
                      {f.meaning && <span className="text-ghost-faint truncate max-w-[200px]">{f.meaning}</span>}
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}

      {tours.length > 0 && (
        <Section title={`Guided Tours (${tours.length})`}>
          {tours.map((tour: any, i: number) => (
            <div key={i} className="p-5 card mb-3">
              <h4 className="text-sm font-display font-semibold text-accent mb-1">{tour.flow_name}</h4>
              {tour.when_you_need_this && <p className="text-xs font-body text-ghost mb-3">{tour.when_you_need_this}</p>}
              <div className="space-y-2">
                {(tour.steps ?? []).map((s: any, j: number) => (
                  <div key={j} className="p-3 bg-obsidian rounded-sm border border-edge/50">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-[10px] font-mono font-bold text-accent w-5">{s.step}.</span>
                      <span className="text-xs font-body text-zinc-300">{s.goal}</span>
                    </div>
                    <div className="flex items-center gap-2 text-[10px] font-mono">
                      {s.method && <span className="text-info">{s.method}</span>}
                      {s.path && <span className="text-zinc-400">{s.path}</span>}
                    </div>
                    {s.what_to_check_in_response && <p className="text-[10px] font-body text-ghost mt-1">{s.what_to_check_in_response}</p>}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </Section>
      )}

      {playbook && (
        <Section title="Testing Playbook">
          <div className="grid grid-cols-2 gap-3">
            {['contract_checks', 'negative_tests', 'security_sanity_checks', 'performance_sanity_checks'].map(cat => {
              const items = playbook[cat] ?? []
              if (items.length === 0) return null
              return (
                <div key={cat} className="p-4 card">
                  <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">{cat.replace(/_/g, ' ')}</p>
                  <div className="space-y-1.5">
                    {items.map((t: any, i: number) => (
                      <div key={i} className="text-xs font-body text-zinc-300">
                        <span className="text-ghost mr-1">{i + 1}.</span>
                        {t.name ?? t.description ?? t.test ?? JSON.stringify(t)}
                      </div>
                    ))}
                  </div>
                </div>
              )
            })}
          </div>
        </Section>
      )}

      {facts.length > 0 && (
        <Section title={`Facts (${facts.length})`}>
          <div className="space-y-2">
            {facts.map((f: any, i: number) => (
              <div key={i} className="p-4 card flex items-start gap-3">
                <ConfidenceDot confidence={f.confidence} />
                <p className="text-xs font-body text-zinc-300 leading-relaxed">{f.text}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {questions.length > 0 && <OpenQuestions items={questions} />}
    </div>
  )
}

/* ─── Audit Tab ─── */

function AuditTab({ data, loading, failed, retries }: { data: LLMReport | null; loading: boolean; failed: boolean; retries: number }) {
  if (failed) return <FailedState label="audit" />
  if (loading || !data) return <LoadingState label="audit" retries={retries} />

  const d = data as any
  const scores = d.scores ?? {}
  const exec = d.executive_summary
  const discrepancies = d.ast_vs_runtime_discrepancies ?? []
  const authMatrix = d.auth_matrix
  const findings = d.findings ?? []
  const hotspots = d.endpoint_risk_hotspots ?? []
  const plan = d.remediation_plan
  const questions = d.open_questions ?? []

  return (
    <div className="space-y-8 animate-fade-in">
      {Object.keys(scores).length > 0 && (
        <Section title="LLM Scores">
          <div className="grid grid-cols-3 gap-3">
            {Object.entries(scores).map(([dim, sc]: [string, any]) => (
              <div key={dim} className="p-4 card">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-display font-semibold capitalize" style={{ color: SCORE_COLORS[dim] ?? '#52a8ff' }}>
                    {SCORE_ICONS[dim] ?? '?'} {dim}
                  </span>
                  <span className="text-lg font-display font-bold" style={{ color: SCORE_COLORS[dim] ?? '#52a8ff' }}>
                    {sc.score_0_to_5}/5
                  </span>
                </div>
                <p className="text-xs font-body text-ghost leading-relaxed">{sc.rationale}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {exec && (
        <Section title="Executive Summary">
          <div className={`p-5 card border-l-4 ${exec.overall_risk === 'critical' ? 'border-l-danger' : exec.overall_risk === 'high' ? 'border-l-danger/70' : exec.overall_risk === 'medium' ? 'border-l-warn' : 'border-l-ok'}`}>
            <div className="flex items-center gap-2 mb-3">
              <SeverityBadge severity={exec.overall_risk} />
              <span className="text-sm font-display font-semibold">Overall Risk</span>
            </div>
            <p className="text-sm font-body text-zinc-300 leading-relaxed mb-4">{exec.summary}</p>

            {(exec.top_risks ?? []).length > 0 && (
              <div className="space-y-2 mb-4">
                <p className="text-[10px] font-display font-semibold tracking-wider text-danger uppercase">Top Risks</p>
                {exec.top_risks.map((r: any, i: number) => (
                  <div key={i} className="p-3 bg-danger-muted/30 rounded-sm border border-danger/20">
                    <p className="text-xs font-display font-semibold text-zinc-200 mb-1">{r.title}</p>
                    <p className="text-xs font-body text-ghost">{r.impact}</p>
                    {r.why_now && <p className="text-xs font-body text-warn mt-1">{r.why_now}</p>}
                  </div>
                ))}
              </div>
            )}

            {(exec.what_is_working ?? []).length > 0 && (
              <div className="space-y-2">
                <p className="text-[10px] font-display font-semibold tracking-wider text-ok uppercase">What's Working</p>
                {exec.what_is_working.map((w: any, i: number) => (
                  <div key={i} className="p-3 bg-ok-muted/30 rounded-sm border border-ok/20">
                    <p className="text-xs font-body text-zinc-300">{w.text}</p>
                    {w.why_it_matters && <p className="text-xs font-body text-ghost mt-1">{w.why_it_matters}</p>}
                  </div>
                ))}
              </div>
            )}
          </div>
        </Section>
      )}

      {discrepancies.length > 0 && (
        <Section title={`AST vs Runtime Discrepancies (${discrepancies.length})`}>
          <div className="space-y-2">
            {discrepancies.map((disc: any, i: number) => (
              <div key={i} className="p-4 card border-warn/30">
                <div className="flex items-center gap-2 mb-1">
                  <SeverityBadge severity={disc.risk ?? 'medium'} />
                </div>
                <p className="text-xs font-body text-zinc-300">{disc.description}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {authMatrix && (
        <Section title="Auth Matrix">
          <div className="p-5 card">
            {authMatrix.high_level && <p className="text-xs font-body text-ghost leading-relaxed mb-3">{authMatrix.high_level}</p>}
            {(authMatrix.notable_exposures ?? []).length > 0 && (
              <div className="space-y-2">
                {authMatrix.notable_exposures.map((exp: any, i: number) => (
                  <div key={i} className="p-3 bg-danger-muted/30 rounded-sm border border-danger/20">
                    <p className="text-xs font-body text-danger">{exp.description ?? JSON.stringify(exp)}</p>
                  </div>
                ))}
              </div>
            )}
            {(authMatrix.notable_exposures ?? []).length === 0 && (
              <p className="text-xs font-mono text-ok">No notable exposures detected</p>
            )}
          </div>
        </Section>
      )}

      {findings.length > 0 && (
        <Section title={`Findings (${findings.length})`}>
          <div className="space-y-3">
            {findings.map((f: any, i: number) => (
              <div key={i} className="card overflow-hidden">
                <div className="px-5 py-3 border-b border-edge bg-surface/60 flex items-center gap-2">
                  <SeverityBadge severity={f.severity} />
                  <span className="text-xs font-mono text-ghost-faint">{f.id}</span>
                  <span className="text-xs font-mono text-ghost-faint">{f.category}</span>
                  {f.classification && <Badge color={f.classification === 'confirmed' ? 'ok' : 'ghost'}>{f.classification}</Badge>}
                </div>
                <div className="px-5 py-4 space-y-3">
                  <h4 className="text-sm font-display font-semibold text-zinc-200">{f.title}</h4>
                  {f.what_we_observed && (
                    <div>
                      <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Observed</p>
                      <p className="text-xs font-body text-zinc-300 leading-relaxed">{f.what_we_observed}</p>
                    </div>
                  )}
                  {f.why_it_matters && (
                    <div>
                      <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Impact</p>
                      <p className="text-xs font-body text-ghost leading-relaxed">{f.why_it_matters}</p>
                    </div>
                  )}
                  {(f.how_to_reproduce ?? []).length > 0 && (
                    <div>
                      <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Reproduce</p>
                      <div className="space-y-1">
                        {f.how_to_reproduce.map((s: any, j: number) => (
                          <div key={j} className="p-2 bg-obsidian rounded-sm border border-edge/50 text-xs font-mono">
                            <span className="text-accent mr-2">{s.step}.</span>
                            <span className="text-ghost">{s.expected}</span>
                            {s.request_ref && <span className="text-ghost-faint ml-2">ref:{s.request_ref.slice(0, 8)}</span>}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                  {f.remediation && (
                    <div>
                      <p className="text-[10px] font-display font-semibold tracking-wider text-accent uppercase mb-1">Remediation</p>
                      <p className="text-xs font-body text-accent-dim leading-relaxed">{f.remediation}</p>
                    </div>
                  )}
                  {f.confidence != null && (
                    <div className="flex items-center gap-2">
                      <ConfidenceDot confidence={f.confidence} />
                      <span className="text-[10px] font-mono text-ghost">{Math.round(f.confidence * 100)}% confidence</span>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}

      {hotspots.length > 0 && (
        <Section title={`Endpoint Risk Hotspots (${hotspots.length})`}>
          <div className="space-y-2">
            {hotspots.map((h: any, i: number) => (
              <div key={i} className="p-4 card">
                <div className="flex items-center gap-2 mb-1">
                  <MethodBadge method={h.method ?? ''} />
                  <span className="text-xs font-mono text-zinc-300">{h.path ?? h.endpoint}</span>
                  <SeverityBadge severity={h.risk ?? 'medium'} />
                </div>
                {h.reason && <p className="text-xs font-body text-ghost">{h.reason}</p>}
              </div>
            ))}
          </div>
        </Section>
      )}

      {plan && (
        <Section title="Remediation Plan">
          <div className="grid grid-cols-3 gap-4">
            {[['in_72_hours', '72 Hours', 'danger'], ['in_2_weeks', '2 Weeks', 'warn'], ['in_2_months', '2 Months', 'info']].map(([key, label, color]) => {
              const items = plan[key] ?? []
              if (items.length === 0) return null
              return (
                <div key={key} className="p-4 card">
                  <p className={`text-[10px] font-display font-semibold tracking-wider uppercase mb-3 text-${color}`}>{label}</p>
                  <div className="space-y-2">
                    {items.map((item: any, i: number) => (
                      <div key={i}>
                        <p className="text-xs font-body text-zinc-300 mb-0.5">{item.action}</p>
                        {item.why && <p className="text-[10px] font-body text-ghost">{item.why}</p>}
                      </div>
                    ))}
                  </div>
                </div>
              )
            })}
          </div>
        </Section>
      )}

      {questions.length > 0 && <OpenQuestions items={questions} />}
    </div>
  )
}

/* ─── Endpoints Tab ─── */

function EndpointsTab({ index, runId }: { index: IntelIndex | null; runId: string | null }) {
  const [filter, setFilter] = useState({ domain: '', method: '', auth: '', search: '' })
  const [expandedId, setExpandedId] = useState<string | null>(null)

  if (!index || !runId) return <LoadingState label="endpoint intelligence" retries={0} />

  const domains = index.domains ?? []
  const allEndpoints = index.endpoints ?? []

  const filtered = allEndpoints.filter(ep => {
    if (filter.domain && ep.domain !== filter.domain) return false
    if (filter.method && ep.method !== filter.method) return false
    if (filter.auth && ep.auth_required !== filter.auth) return false
    if (filter.search) {
      const q = filter.search.toLowerCase()
      if (!ep.path.toLowerCase().includes(q) && !ep.operation_id.toLowerCase().includes(q) && !ep.summary.toLowerCase().includes(q)) return false
    }
    return true
  })

  const methods = [...new Set(allEndpoints.map(e => e.method))].sort()
  const authValues = [...new Set(allEndpoints.map(e => e.auth_required))]

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex flex-wrap gap-2 items-center">
        <select value={filter.domain} onChange={e => setFilter(f => ({ ...f, domain: e.target.value }))}
          className="bg-surface border border-edge rounded-sm px-3 py-1.5 text-xs font-mono text-zinc-300 focus:outline-none focus:border-accent">
          <option value="">All Domains</option>
          {domains.map(d => <option key={d.domain_name} value={d.domain_name}>{d.domain_name} ({d.endpoint_count})</option>)}
        </select>
        <select value={filter.method} onChange={e => setFilter(f => ({ ...f, method: e.target.value }))}
          className="bg-surface border border-edge rounded-sm px-3 py-1.5 text-xs font-mono text-zinc-300 focus:outline-none focus:border-accent">
          <option value="">All Methods</option>
          {methods.map(m => <option key={m} value={m}>{m}</option>)}
        </select>
        <select value={filter.auth} onChange={e => setFilter(f => ({ ...f, auth: e.target.value }))}
          className="bg-surface border border-edge rounded-sm px-3 py-1.5 text-xs font-mono text-zinc-300 focus:outline-none focus:border-accent">
          <option value="">Any Auth</option>
          {authValues.map(a => <option key={a} value={a}>{a}</option>)}
        </select>
        <input type="text" placeholder="Search path, operation, summary..." value={filter.search}
          onChange={e => setFilter(f => ({ ...f, search: e.target.value }))}
          className="flex-1 min-w-[200px] bg-surface border border-edge rounded-sm px-3 py-1.5 text-xs font-mono text-zinc-300 placeholder:text-ghost-faint focus:outline-none focus:border-accent" />
        <span className="text-[10px] font-mono text-ghost">{filtered.length} of {allEndpoints.length}</span>
      </div>

      <div className="space-y-1">
        {filtered.map(ep => (
          <EndpointRow key={ep.endpoint_id} ep={ep} expanded={expandedId === ep.endpoint_id}
            onToggle={() => setExpandedId(expandedId === ep.endpoint_id ? null : ep.endpoint_id)}
            runId={runId} />
        ))}
        {filtered.length === 0 && (
          <div className="p-8 card text-center text-xs text-ghost">No endpoints match filters.</div>
        )}
      </div>
    </div>
  )
}

function EndpointRow({ ep, expanded, onToggle, runId }: { ep: IntelEndpointIndex; expanded: boolean; onToggle: () => void; runId: string }) {
  return (
    <div className="card overflow-hidden">
      <button onClick={onToggle} className="w-full px-4 py-3 flex items-center gap-3 text-left hover:bg-surface/80 transition-colors">
        <MethodBadge method={ep.method} />
        <span className="text-xs font-mono text-zinc-200 flex-1">{ep.path}</span>
        <span className="text-[10px] font-mono text-ghost max-w-[200px] truncate hidden sm:block">{ep.summary}</span>
        <AuthBadge auth={ep.auth_required} />
        {ep.has_evidence && <span className="w-1.5 h-1.5 rounded-full bg-ok shrink-0" title="Has runtime evidence" />}
        <ConfidenceBar confidence={ep.confidence} />
        <span className="text-[10px] font-mono text-ghost">{ep.command_count} cmd</span>
        <svg xmlns="http://www.w3.org/2000/svg" className={`h-3 w-3 text-ghost transition-transform ${expanded ? 'rotate-180' : ''}`} viewBox="0 0 20 20" fill="currentColor">
          <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
        </svg>
      </button>
      {expanded && <EndpointDetail endpointId={ep.endpoint_id} runId={runId} />}
    </div>
  )
}

function EndpointDetail({ endpointId, runId }: { endpointId: string; runId: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ['endpoint-detail', runId, endpointId],
    queryFn: () => api.runs.endpointDetail(runId, endpointId),
  })
  const { data: scripts } = useQuery({
    queryKey: ['endpoint-scripts', runId, endpointId],
    queryFn: () => api.runs.endpointScripts(runId, endpointId),
  })

  if (isLoading || !data) return <div className="p-4 flex justify-center"><Spinner /></div>

  const ep = data.endpoint
  const dom = data.domain

  return (
    <div className="border-t border-edge px-5 py-4 space-y-5 bg-obsidian/30 animate-fade-in">
      <div className="flex items-center gap-2 flex-wrap">
        <Badge color="info">{dom}</Badge>
        <span className="text-xs font-mono text-ghost">{ep.operation_id}</span>
        {(ep.tags ?? []).map((t: string) => <Badge key={t} color="ghost">{t}</Badge>)}
      </div>

      {/* What it does */}
      <div>
        <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">What It Does</p>
        <p className="text-sm font-body text-zinc-200 mb-1">{ep.what_it_does.summary}</p>
        <p className="text-xs font-body text-ghost leading-relaxed">{ep.what_it_does.detailed}</p>
        <div className="flex items-center gap-2 mt-2">
          <ConfidenceBar confidence={ep.what_it_does.confidence} />
          <span className="text-[10px] font-mono text-ghost">{Math.round(ep.what_it_does.confidence * 100)}% confidence</span>
        </div>
        {(ep.what_it_does.facts ?? []).length > 0 && (
          <div className="mt-2 space-y-1">
            {ep.what_it_does.facts.map((f: any, i: number) => (
              <div key={i} className="flex items-start gap-2 text-xs"><span className="text-ok shrink-0">FACT</span><span className="text-zinc-300">{f.text}</span></div>
            ))}
          </div>
        )}
        {(ep.what_it_does.inferences ?? []).length > 0 && (
          <div className="mt-2 space-y-1">
            {ep.what_it_does.inferences.map((inf: any, i: number) => (
              <div key={i} className="flex items-start gap-2 text-xs"><span className="text-warn shrink-0">INFER</span><span className="text-ghost">{inf.text}</span><span className="text-ghost-faint">({Math.round(inf.confidence * 100)}%)</span></div>
            ))}
          </div>
        )}
      </div>

      {/* Auth */}
      <div className="flex items-center gap-3">
        <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase">Auth</p>
        <AuthBadge auth={ep.auth.required} />
        <span className="text-[10px] font-mono text-ghost">from: {ep.auth.from}</span>
        <span className="text-[10px] font-mono text-ghost">mechanism: {ep.auth.mechanism}</span>
      </div>
      {ep.auth.notes && <p className="text-xs font-body text-warn -mt-3">{ep.auth.notes}</p>}

      {/* Inputs */}
      {((ep.inputs.path_params ?? []).length > 0 || (ep.inputs.query_params ?? []).length > 0 || (ep.inputs.headers ?? []).length > 0 || ep.inputs.body) && (
        <div>
          <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Inputs</p>
          <div className="grid grid-cols-2 gap-3">
            {(ep.inputs.path_params ?? []).length > 0 && (
              <div className="p-3 bg-surface rounded-sm border border-edge/50">
                <p className="text-[10px] font-display font-semibold text-ghost uppercase mb-1">Path Params</p>
                {ep.inputs.path_params!.map((p: any, i: number) => (
                  <div key={i} className="flex items-center gap-2 text-xs">
                    <span className="font-mono text-accent">{p.name}</span>
                    <span className="text-ghost-faint">{p.type}</span>
                    <span className="text-ghost truncate">{p.meaning}</span>
                  </div>
                ))}
              </div>
            )}
            {(ep.inputs.query_params ?? []).length > 0 && (
              <div className="p-3 bg-surface rounded-sm border border-edge/50">
                <p className="text-[10px] font-display font-semibold text-ghost uppercase mb-1">Query Params</p>
                {ep.inputs.query_params!.map((p: any, i: number) => (
                  <div key={i} className="flex items-center gap-2 text-xs">
                    <span className="font-mono text-info">{p.name}</span>
                    <span className="text-ghost-faint">{p.type}</span>
                  </div>
                ))}
              </div>
            )}
            {ep.inputs.body && (
              <div className="p-3 bg-surface rounded-sm border border-edge/50 col-span-2">
                <p className="text-[10px] font-display font-semibold text-ghost uppercase mb-1">Body ({ep.inputs.body.content_type})</p>
                {(ep.inputs.body.required_fields ?? []).map((f: any, i: number) => (
                  <div key={i} className="flex items-center gap-2 text-xs">
                    <span className="font-mono text-zinc-300">{f.field_path}</span>
                    <span className="text-ghost-faint">{f.type}</span>
                    {f.meaning && <span className="text-ghost truncate">{f.meaning}</span>}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Outputs */}
      {((ep.outputs.responses ?? []).length > 0 || (ep.outputs.common_errors ?? []).length > 0) && (
        <div>
          <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Outputs</p>
          <div className="flex flex-wrap gap-2">
            {(ep.outputs.responses ?? []).map((r: any, i: number) => (
              <div key={i} className="px-3 py-2 bg-surface rounded-sm border border-edge/50">
                <span className="text-xs font-mono font-bold text-ok mr-2">{r.status}</span>
                <span className="text-[10px] text-ghost">{r.content_type}</span>
                <p className="text-xs text-zinc-300 mt-0.5">{r.what_you_get}</p>
              </div>
            ))}
            {(ep.outputs.common_errors ?? []).map((e: any, i: number) => (
              <div key={i} className="px-3 py-2 bg-danger-muted/20 rounded-sm border border-danger/20">
                <span className="text-xs font-mono font-bold text-danger mr-2">{e.status}</span>
                <span className="text-xs text-ghost">{e.meaning}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Ready Commands */}
      <div>
        <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Ready Commands</p>
        {ep.how_to_query.goal && <p className="text-xs font-body text-ghost mb-2">{ep.how_to_query.goal}</p>}
        <div className="space-y-2">
          {(ep.how_to_query.ready_commands ?? []).map((cmd: any, i: number) => (
            <CommandBlock key={i} name={cmd.name} command={cmd.command} basedOn={cmd.based_on} notes={cmd.notes} />
          ))}
        </div>
        {(ep.how_to_query.query_variants ?? []).length > 0 && (
          <div className="mt-3">
            <p className="text-[10px] font-display font-semibold text-ghost uppercase mb-1">Variants</p>
            {ep.how_to_query.query_variants!.map((v: any, i: number) => (
              <CommandBlock key={i} name={v.variant_name} command={v.command} basedOn={v.source}
                notes={`${v.description} (${Math.round(v.confidence * 100)}% confidence)`} />
            ))}
          </div>
        )}
        {(ep.how_to_query.warnings ?? []).length > 0 && (
          <div className="mt-2 space-y-1">
            {ep.how_to_query.warnings!.map((w: string, i: number) => (
              <p key={i} className="text-[10px] font-body text-warn">⚠ {w}</p>
            ))}
          </div>
        )}
      </div>

      {/* Try It - Request Builder */}
      <RequestBuilder endpointId={endpointId} runId={runId} endpoint={ep} />

      {/* Scripts */}
      {scripts && (
        <div>
          <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Shell Scripts</p>
          <div className="flex flex-wrap gap-2">
            {scripts.happy_path && <ScriptDownload label="Happy Path" content={scripts.happy_path} filename={`${endpointId}__happy_path.sh`} />}
            {scripts.no_auth && <ScriptDownload label="No Auth" content={scripts.no_auth} filename={`${endpointId}__no_auth.sh`} />}
            {scripts.variants && <ScriptDownload label="Variants" content={scripts.variants} filename={`${endpointId}__variants.sh`} />}
            {scripts.http_file && <ScriptDownload label=".http File" content={scripts.http_file} filename={`${endpointId}.http`} />}
          </div>
        </div>
      )}

      {/* Security */}
      {((ep.security_notes.exposures ?? []).length > 0 || (ep.security_notes.ast_code_patterns_related ?? []).length > 0) && (
        <div>
          <p className="text-[10px] font-display font-semibold tracking-wider text-danger uppercase mb-2">Security Notes</p>
          {(ep.security_notes.exposures ?? []).map((e: any, i: number) => (
            <div key={i} className="p-3 bg-danger-muted/20 rounded-sm border border-danger/20 mb-1">
              <SeverityBadge severity={e.severity} />
              <span className="text-xs text-zinc-300 ml-2">{e.text}</span>
            </div>
          ))}
          {(ep.security_notes.ast_code_patterns_related ?? []).map((p: any, i: number) => (
            <div key={i} className="p-2 bg-surface rounded-sm text-xs text-ghost mb-1">
              AST pattern: <span className="font-mono text-warn">{p.pattern}</span>
            </div>
          ))}
        </div>
      )}

      {/* Suggested Tests */}
      {(ep.tests_you_should_run ?? []).length > 0 && (
        <div>
          <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Suggested Tests</p>
          <div className="space-y-1">
            {ep.tests_you_should_run!.map((t: any, i: number) => (
              <div key={i} className="flex items-center gap-2 text-xs">
                <span className={`px-1.5 py-0.5 rounded-sm text-[10px] font-mono ${t.importance === 'high' ? 'bg-danger-muted text-danger' : 'bg-surface text-ghost'}`}>{t.importance}</span>
                <span className="font-mono text-zinc-300">{t.name}</span>
                <span className="text-ghost">{t.why}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

/* ─── Try It - Request Builder + Response Viewer ─── */

function RequestBuilder({ endpointId, runId, endpoint }: { endpointId: string; runId: string; endpoint: any }) {
  const [expanded, setExpanded] = useState(false)
  const [sending, setSending] = useState(false)
  const [response, setResponse] = useState<PlaygroundResponse | null>(null)
  const [history, setHistory] = useState<PlaygroundResponse[]>([])
  const [showRaw, setShowRaw] = useState(false)

  const baseUrl = endpoint.path_template ?? ''
  const method = endpoint.method ?? 'GET'
  const authInfo = endpoint.auth ?? {}

  const [url, setUrl] = useState('')
  const [headers, setHeaders] = useState<Record<string, string>>({})
  const [body, setBody] = useState('')
  const [authMode, setAuthMode] = useState<'none' | 'bearer' | 'api_key' | 'cookie'>(
    authInfo.mechanism === 'jwt' || authInfo.mechanism === 'bearer' ? 'bearer'
    : authInfo.mechanism === 'api_key' ? 'api_key'
    : authInfo.mechanism === 'cookie' ? 'cookie' : 'none'
  )
  const [authValue, setAuthValue] = useState('')
  const [timeout, setTimeout_] = useState(10000)

  useEffect(() => {
    const firstCmd = endpoint.how_to_query?.ready_commands?.[0]
    if (firstCmd?.command) {
      const curlMatch = firstCmd.command.match(/curl\s.*?['"]?(https?:\/\/[^\s'"]+)['"]?/)
      if (curlMatch) setUrl(curlMatch[1])
      else setUrl(baseUrl)
    }
  }, [endpoint, baseUrl])

  const { data: authProfiles } = useQuery({
    queryKey: ['auth-profiles', runId],
    queryFn: () => api.playground.authProfiles(runId),
    enabled: expanded,
  })

  const sendRequest = async () => {
    setSending(true)
    setResponse(null)
    try {
      const finalHeaders = { ...headers }
      if (authMode === 'bearer' && authValue) finalHeaders['Authorization'] = `Bearer ${authValue}`
      else if (authMode === 'api_key' && authValue) finalHeaders['X-API-Key'] = authValue
      else if (authMode === 'cookie' && authValue) finalHeaders['Cookie'] = authValue

      if (body && !finalHeaders['Content-Type']) finalHeaders['Content-Type'] = 'application/json'

      const resp = await api.playground.send(runId, {
        endpoint_id: endpointId,
        method,
        url,
        headers: finalHeaders,
        body: body || undefined,
        timeout_ms: timeout,
      })
      setResponse(resp)
      setHistory(prev => [resp, ...prev].slice(0, 10))
    } catch (err: any) {
      setResponse({ evidence_id: '', latency_ms: 0, error: err.message })
    }
    setSending(false)
  }

  const replay = async (evidenceId: string) => {
    setSending(true)
    setResponse(null)
    try {
      const resp = await api.playground.replay(runId, evidenceId)
      setResponse(resp)
      setHistory(prev => [resp, ...prev].slice(0, 10))
    } catch (err: any) {
      setResponse({ evidence_id: '', latency_ms: 0, error: err.message })
    }
    setSending(false)
  }

  return (
    <div className="border border-accent/30 rounded-sm overflow-hidden">
      <button onClick={() => setExpanded(!expanded)}
        className="w-full px-4 py-2.5 flex items-center justify-between bg-surface/60 hover:bg-surface/80 transition-colors">
        <div className="flex items-center gap-2">
          <span className="text-sm">&#9889;</span>
          <span className="text-xs font-display font-semibold text-accent tracking-wide uppercase">Try It</span>
          <span className="text-[10px] font-mono text-ghost">Send a real request</span>
        </div>
        <svg xmlns="http://www.w3.org/2000/svg" className={`h-3 w-3 text-ghost transition-transform ${expanded ? 'rotate-180' : ''}`} viewBox="0 0 20 20" fill="currentColor">
          <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
        </svg>
      </button>

      {expanded && (
        <div className="p-4 space-y-4 bg-obsidian/40 animate-fade-in">
          {/* URL */}
          <div className="flex gap-2">
            <span className={`px-2 py-1.5 rounded-sm text-xs font-mono font-bold border border-edge bg-surface ${
              method === 'GET' ? 'text-ok' : method === 'POST' ? 'text-info' : method === 'DELETE' ? 'text-danger' : 'text-warn'
            }`}>{method}</span>
            <input type="text" value={url} onChange={e => setUrl(e.target.value)} placeholder="Full URL"
              className="flex-1 bg-surface border border-edge rounded-sm px-3 py-1.5 text-xs font-mono text-zinc-200 placeholder:text-ghost-faint focus:outline-none focus:border-accent" />
            <button onClick={sendRequest} disabled={sending || !url}
              className={`px-4 py-1.5 rounded-sm text-xs font-display font-bold tracking-wide transition-all ${
                sending ? 'bg-surface text-ghost cursor-wait' : 'bg-accent text-obsidian hover:bg-accent-bright'
              }`}>
              {sending ? 'Sending...' : 'SEND'}
            </button>
          </div>

          {/* Auth */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <p className="text-[10px] font-display font-semibold text-ghost uppercase mb-1">Auth</p>
              <div className="flex gap-1">
                {(['none', 'bearer', 'api_key', 'cookie'] as const).map(m => (
                  <button key={m} onClick={() => setAuthMode(m)}
                    className={`px-2 py-1 rounded-sm text-[10px] font-mono border transition-colors ${
                      authMode === m ? 'border-accent text-accent bg-surface' : 'border-edge text-ghost bg-surface/50 hover:border-ghost'
                    }`}>{m === 'none' ? 'None' : m === 'bearer' ? 'Bearer' : m === 'api_key' ? 'API Key' : 'Cookie'}</button>
                ))}
              </div>
              {authMode !== 'none' && (
                <input type="password" value={authValue} onChange={e => setAuthValue(e.target.value)}
                  placeholder={authMode === 'bearer' ? 'Token value' : authMode === 'api_key' ? 'API key value' : 'Cookie string'}
                  className="mt-2 w-full bg-surface border border-edge rounded-sm px-3 py-1.5 text-xs font-mono text-zinc-200 placeholder:text-ghost-faint focus:outline-none focus:border-accent" />
              )}
              {(authProfiles ?? []).length > 0 && (
                <div className="mt-2">
                  <p className="text-[10px] text-ghost mb-1">Saved profiles:</p>
                  <div className="flex flex-wrap gap-1">
                    {authProfiles!.map((p: AuthProfile) => (
                      <button key={p.id} onClick={() => { setAuthMode(p.mechanism as any); setAuthValue(p.masked_value) }}
                        className="px-2 py-0.5 rounded-sm text-[10px] font-mono bg-surface border border-edge text-ghost hover:border-accent hover:text-accent transition-colors">
                        {p.name} ({p.mechanism})
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>
            <div>
              <p className="text-[10px] font-display font-semibold text-ghost uppercase mb-1">Headers</p>
              <HeaderEditor headers={headers} onChange={setHeaders} />
            </div>
          </div>

          {/* Body (for POST/PUT/PATCH) */}
          {['POST', 'PUT', 'PATCH'].includes(method) && (
            <div>
              <p className="text-[10px] font-display font-semibold text-ghost uppercase mb-1">Body (JSON)</p>
              <textarea value={body} onChange={e => setBody(e.target.value)} rows={4}
                placeholder={endpoint.inputs?.body?.required_fields?.length
                  ? JSON.stringify(Object.fromEntries(endpoint.inputs.body.required_fields.map((f: any) => [f.field_path, `<${f.type}>`])), null, 2)
                  : '{}'}
                className="w-full bg-surface border border-edge rounded-sm px-3 py-2 text-xs font-mono text-zinc-200 placeholder:text-ghost-faint focus:outline-none focus:border-accent resize-y" />
            </div>
          )}

          <div className="flex items-center gap-3 text-[10px] font-mono text-ghost">
            <label>Timeout:
              <input type="number" value={timeout} onChange={e => setTimeout_(Number(e.target.value))} min={1000} max={30000} step={1000}
                className="ml-1 w-16 bg-surface border border-edge rounded-sm px-1.5 py-0.5 text-xs font-mono text-zinc-200 focus:outline-none" />
              ms
            </label>
          </div>

          {/* Response Viewer */}
          {response && (
            <div className={`border rounded-sm ${response.error ? 'border-danger/40' : 'border-ok/30'} overflow-hidden`}>
              <div className={`px-4 py-2 flex items-center justify-between ${response.error ? 'bg-danger-muted' : 'bg-ok-muted/30'}`}>
                <div className="flex items-center gap-3">
                  {response.status ? (
                    <span className={`text-sm font-mono font-bold ${response.status < 300 ? 'text-ok' : response.status < 400 ? 'text-info' : response.status < 500 ? 'text-warn' : 'text-danger'}`}>
                      {response.status}
                    </span>
                  ) : (
                    <span className="text-sm font-mono font-bold text-danger">ERROR</span>
                  )}
                  <span className="text-[10px] font-mono text-ghost">{response.latency_ms}ms</span>
                  {response.size != null && <span className="text-[10px] font-mono text-ghost">{response.size}B</span>}
                  {response.content_type && <span className="text-[10px] font-mono text-ghost">{response.content_type}</span>}
                </div>
                <div className="flex items-center gap-2">
                  <button onClick={() => setShowRaw(!showRaw)}
                    className="px-2 py-0.5 rounded-sm text-[10px] font-mono border border-edge text-ghost hover:text-accent hover:border-accent transition-colors">
                    {showRaw ? 'Pretty' : 'Raw'}
                  </button>
                  {response.evidence_id && (
                    <button onClick={() => replay(response.evidence_id)}
                      className="px-2 py-0.5 rounded-sm text-[10px] font-mono border border-edge text-ghost hover:text-accent hover:border-accent transition-colors">
                      Replay
                    </button>
                  )}
                </div>
              </div>

              {response.error && (
                <div className="px-4 py-3">
                  <p className="text-xs font-mono text-danger">{response.error}</p>
                </div>
              )}

              {response.headers && Object.keys(response.headers).length > 0 && (
                <details className="border-t border-edge/50">
                  <summary className="px-4 py-1.5 text-[10px] font-mono text-ghost cursor-pointer hover:text-zinc-300">
                    Response Headers ({Object.keys(response.headers).length})
                  </summary>
                  <div className="px-4 py-2 space-y-0.5">
                    {Object.entries(response.headers).map(([k, v]) => (
                      <div key={k} className="flex gap-2 text-[10px] font-mono">
                        <span className="text-info">{k}:</span>
                        <span className="text-ghost">{v}</span>
                      </div>
                    ))}
                  </div>
                </details>
              )}

              {response.body && (
                <div className="border-t border-edge/50">
                  <pre className="p-4 text-xs font-mono text-zinc-300 max-h-80 overflow-auto whitespace-pre-wrap">
                    {showRaw ? response.body : tryFormatJSON(response.body)}
                  </pre>
                </div>
              )}

              {response.evidence_id && (
                <div className="px-4 py-2 border-t border-edge/50 flex items-center gap-2">
                  <span className="text-[10px] font-mono text-ghost-faint">Evidence ID: {response.evidence_id.slice(0, 16)}...</span>
                </div>
              )}
            </div>
          )}

          {/* History */}
          {history.length > 1 && (
            <details>
              <summary className="text-[10px] font-mono text-ghost cursor-pointer hover:text-zinc-300">
                Request History ({history.length})
              </summary>
              <div className="mt-1 space-y-1">
                {history.map((h, i) => (
                  <div key={i} className="flex items-center gap-2 text-[10px] font-mono">
                    <span className={h.status && h.status < 400 ? 'text-ok' : 'text-danger'}>{h.status ?? 'ERR'}</span>
                    <span className="text-ghost">{h.latency_ms}ms</span>
                    {h.evidence_id && (
                      <button onClick={() => replay(h.evidence_id)} className="text-accent hover:underline">replay</button>
                    )}
                  </div>
                ))}
              </div>
            </details>
          )}
        </div>
      )}
    </div>
  )
}

function HeaderEditor({ headers, onChange }: { headers: Record<string, string>; onChange: (h: Record<string, string>) => void }) {
  const [newKey, setNewKey] = useState('')
  const [newVal, setNewVal] = useState('')

  const add = () => {
    if (newKey.trim()) {
      onChange({ ...headers, [newKey.trim()]: newVal })
      setNewKey('')
      setNewVal('')
    }
  }

  const remove = (key: string) => {
    const copy = { ...headers }
    delete copy[key]
    onChange(copy)
  }

  return (
    <div className="space-y-1">
      {Object.entries(headers).map(([k, v]) => (
        <div key={k} className="flex items-center gap-1 text-[10px] font-mono">
          <span className="text-info">{k}:</span>
          <span className="text-ghost flex-1 truncate">{v}</span>
          <button onClick={() => remove(k)} className="text-danger hover:text-danger/80">x</button>
        </div>
      ))}
      <div className="flex gap-1">
        <input value={newKey} onChange={e => setNewKey(e.target.value)} placeholder="Header"
          className="w-28 bg-surface border border-edge rounded-sm px-1.5 py-0.5 text-[10px] font-mono text-zinc-200 placeholder:text-ghost-faint focus:outline-none focus:border-accent" />
        <input value={newVal} onChange={e => setNewVal(e.target.value)} placeholder="Value" onKeyDown={e => e.key === 'Enter' && add()}
          className="flex-1 bg-surface border border-edge rounded-sm px-1.5 py-0.5 text-[10px] font-mono text-zinc-200 placeholder:text-ghost-faint focus:outline-none focus:border-accent" />
        <button onClick={add} className="px-1.5 py-0.5 rounded-sm text-[10px] font-mono border border-edge text-ghost hover:text-accent hover:border-accent">+</button>
      </div>
    </div>
  )
}

function tryFormatJSON(s: string): string {
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

function CommandBlock({ name, command, basedOn, notes }: { name: string; command: string; basedOn: string; notes?: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(command)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  const basedOnColor = basedOn === 'evidence' ? 'text-ok' : basedOn === 'schema_inferred' ? 'text-info' : 'text-ghost'

  return (
    <div className="relative group">
      <div className="flex items-center gap-2 mb-1">
        <span className="text-[10px] font-mono text-ghost">{name}</span>
        <span className={`text-[10px] font-mono ${basedOnColor}`}>{basedOn}</span>
      </div>
      <pre className="p-3 bg-obsidian rounded-sm border border-edge/50 text-xs font-mono text-accent-dim overflow-x-auto whitespace-pre-wrap leading-relaxed">{command}</pre>
      <button onClick={copy}
        className="absolute top-8 right-2 px-2 py-1 rounded-sm text-[10px] font-mono bg-surface border border-edge text-ghost opacity-0 group-hover:opacity-100 transition-opacity hover:text-accent hover:border-accent">
        {copied ? 'Copied!' : 'Copy'}
      </button>
      {notes && <p className="text-[10px] font-body text-ghost-faint mt-1">{notes}</p>}
    </div>
  )
}

function ScriptDownload({ label, content, filename }: { label: string; content: string; filename: string }) {
  const download = () => {
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <button onClick={download}
      className="px-3 py-1.5 rounded-sm text-[10px] font-mono bg-surface border border-edge text-ghost hover:text-accent hover:border-accent transition-colors">
      ↓ {label}
    </button>
  )
}

function AuthBadge({ auth }: { auth: string }) {
  const cls = auth === 'yes' ? 'bg-warn-muted text-warn border-warn/30' : auth === 'no' ? 'bg-ok-muted text-ok border-ok/30' : 'bg-surface text-ghost border-edge'
  return <span className={`px-1.5 py-0.5 rounded-sm text-[10px] font-mono border ${cls}`}>{auth === 'yes' ? 'auth' : auth === 'no' ? 'open' : 'auth?'}</span>
}

function ConfidenceBar({ confidence }: { confidence: number }) {
  const pct = Math.round(confidence * 100)
  const color = pct >= 80 ? '#3dd68c' : pct >= 50 ? '#52a8ff' : pct >= 30 ? '#ffb224' : '#ff4f5e'
  return (
    <div className="w-12 h-1.5 bg-surface rounded-sm overflow-hidden shrink-0" title={`${pct}% confidence`}>
      <div className="h-full rounded-sm" style={{ width: `${pct}%`, backgroundColor: color }} />
    </div>
  )
}

/* ─── Shared Components ─── */

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="section-title"><span>{title}</span></h3>
      {children}
    </div>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  const colors: Record<string, string> = {
    critical: 'bg-danger text-obsidian', high: 'bg-danger-muted text-danger border border-danger/30',
    medium: 'bg-warn-muted text-warn border border-warn/30', low: 'bg-surface text-ghost border border-edge',
    info: 'bg-surface text-ghost-faint border border-edge',
  }
  return <span className={`badge ${colors[severity] ?? colors.info}`}>{severity}</span>
}

function StatusBadge({ status }: { status: string }) {
  const cls = status === 'completed' ? 'bg-ok-muted text-ok border-ok/20' : status === 'failed' ? 'bg-danger-muted text-danger border-danger/20' : 'bg-warn-muted text-warn border-warn/20'
  return <span className={`badge border ${cls}`}>{status}</span>
}

function MethodBadge({ method }: { method: string }) {
  const colors: Record<string, string> = {
    GET: 'text-ok', POST: 'text-info', PUT: 'text-warn', PATCH: 'text-warn', DELETE: 'text-danger',
  }
  return <span className={`text-[10px] font-mono font-bold ${colors[method] ?? 'text-ghost'}`}>{method}</span>
}

function Badge({ color, children }: { color: string; children: React.ReactNode }) {
  const cls: Record<string, string> = {
    info: 'bg-info-muted text-info border-info/20', warn: 'bg-warn-muted text-warn border-warn/20',
    ok: 'bg-ok-muted text-ok border-ok/20', ghost: 'bg-surface text-ghost border-edge',
    danger: 'bg-danger-muted text-danger border-danger/20',
  }
  return <span className={`px-2 py-0.5 rounded-sm text-[10px] font-mono border ${cls[color] ?? cls.ghost}`}>{children}</span>
}

function ConfidenceDot({ confidence }: { confidence: number }) {
  const pct = Math.round((confidence ?? 0) * 100)
  const c = pct >= 90 ? '#3dd68c' : pct >= 70 ? '#52a8ff' : pct >= 50 ? '#ffb224' : '#ff4f5e'
  return (
    <div className="shrink-0 w-8 h-8 rounded-sm flex items-center justify-center text-[10px] font-mono font-bold"
      style={{ background: `${c}18`, color: c }}>
      {pct}
    </div>
  )
}

function OpenQuestions({ items }: { items: any[] }) {
  return (
    <Section title={`Open Questions (${items.length})`}>
      <div className="space-y-2">
        {items.map((q: any, i: number) => (
          <div key={i} className="p-4 card">
            <p className="text-sm font-body text-zinc-300">{q.question}</p>
            {q.why_missing && <p className="text-xs font-mono text-ghost mt-1">{q.why_missing}</p>}
            {q.priority && <Badge color={q.priority === 'high' ? 'danger' : q.priority === 'medium' ? 'warn' : 'ghost'}>{q.priority}</Badge>}
          </div>
        ))}
      </div>
    </Section>
  )
}

function LoadingState({ label, retries }: { label: string; retries: number }) {
  return (
    <div className="p-10 text-center animate-fade-in">
      <Spinner />
      <p className="text-sm font-display text-ghost mt-4">Generating {label}...</p>
      <p className="text-xs font-mono text-ghost-faint mt-1">Running in background, may take 1-3 minutes</p>
      <div className="mt-4 w-48 mx-auto bg-surface rounded-sm h-1.5 overflow-hidden">
        <div className="h-full rounded-sm transition-all duration-1000"
          style={{ width: `${Math.min((retries / 24) * 100, 95)}%`, background: 'linear-gradient(90deg, #00cc8860, #00ffaa)' }} />
      </div>
    </div>
  )
}

function FailedState({ label }: { label: string }) {
  return (
    <div className="p-10 text-center animate-fade-in">
      <p className="text-sm text-danger font-display font-semibold mb-1">Could not generate {label}</p>
      <p className="text-xs font-mono text-ghost">The LLM timed out or failed. Dashboard data is still available.</p>
    </div>
  )
}
