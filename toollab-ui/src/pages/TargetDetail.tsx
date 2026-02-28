import { useState, useRef, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import type { Analysis, LLMInterpretation, ProgressEvent } from '../lib/types'
import Spinner from '../components/Spinner'
import { ScoreRing, DonutChart, DonutLegend, HBarChart, StackedBar, PercentRing } from '../components/Charts'

export default function TargetDetail() {
  const { targetId } = useParams<{ targetId: string }>()
  const navigate = useNavigate()
  const { data: target, isLoading: tLoading } = useQuery({ queryKey: ['target', targetId], queryFn: () => api.targets.get(targetId!) })

  const [analysis, setAnalysis] = useState<Analysis | null>(null)
  const [runId, setRunId] = useState<string | null>(null)
  const [tab, setTab] = useState<'dashboard' | 'docs' | 'analysis'>('dashboard')
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [logs, setLogs] = useState<ProgressEvent[]>([])
  const logRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight
    }
  }, [logs])

  const startAnalysis = useCallback(() => {
    if (!targetId || running) return
    setRunning(true)
    setError(null)
    setLogs([])
    setAnalysis(null)
    setRunId(null)

    api.targets.analyzeSSE(targetId, (event) => {
      setLogs(prev => [...prev, event])
    }).then((result) => {
      setAnalysis(result.analysis)
      setRunId(result.run_id)
      setRunning(false)
    }).catch((err) => {
      setError(err.message)
      setRunning(false)
    })
  }, [targetId, running])

  const started = useRef(false)
  useEffect(() => {
    if (target && !started.current) {
      started.current = true
      startAnalysis()
    }
  }, [target, startAnalysis])

  const maxRetries = 90

  const [docRetries, setDocRetries] = useState(0)
  const [docFailed, setDocFailed] = useState(false)
  const { data: documentation } = useQuery({
    queryKey: ['documentation', runId],
    queryFn: async () => {
      try {
        return await api.runs.documentation(runId!)
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

  const [analysisRetries, setAnalysisRetries] = useState(0)
  const [analysisFailed, setAnalysisFailed] = useState(false)
  const { data: interpretation } = useQuery({
    queryKey: ['interpretation', runId],
    queryFn: async () => {
      try {
        return await api.runs.interpretation(runId!)
      } catch (e: any) {
        if ((e?.message ?? '').startsWith('503')) { setAnalysisFailed(true); throw e }
        setAnalysisRetries(prev => prev + 1)
        throw e
      }
    },
    enabled: !!runId && !analysisFailed,
    retry: false,
    refetchInterval: (q) => {
      if (q.state.data || analysisFailed || analysisRetries >= maxRetries) return false
      return 3000
    },
  })

  useEffect(() => {
    if (runId) {
      setDocRetries(0); setDocFailed(false)
      setAnalysisRetries(0); setAnalysisFailed(false)
    }
  }, [runId])

  if (tLoading) return <div className="p-8"><Spinner /></div>

  const currentPhase = logs.length > 0 ? logs[logs.length - 1].phase : ''
  const lastExec = [...logs].reverse().find(l => l.phase === 'execute' && l.current)

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
      <p className="text-xs font-mono text-ghost mb-8">{target?.source.type}: {target?.source.value}</p>

      {running && (
        <div className="mb-8 card overflow-hidden animate-fade-up glow-accent">
          <div className="px-5 py-3 border-b border-edge flex items-center justify-between bg-surface/80">
            <div className="flex items-center gap-3">
              <Spinner />
              <span className="text-sm font-display font-semibold tracking-wide text-accent">
                {phaseLabel(currentPhase)}
              </span>
            </div>
            {lastExec && (
              <span className="text-xs text-ghost font-mono">{lastExec.current}/{lastExec.total} requests</span>
            )}
          </div>
          {lastExec && (
            <div className="px-5 py-1.5 border-b border-edge/50">
              <div className="w-full bg-surface rounded-sm h-1.5">
                <div
                  className="h-1.5 rounded-sm transition-all duration-150"
                  style={{
                    width: `${(lastExec.current! / lastExec.total!) * 100}%`,
                    background: 'linear-gradient(90deg, #00cc88, #00ffaa)',
                    boxShadow: '0 0 8px rgba(0,255,170,0.4)',
                  }}
                />
              </div>
            </div>
          )}
          <div ref={logRef} className="max-h-72 overflow-y-auto p-4 font-mono text-xs leading-relaxed">
            {logs.map((log, i) => (
              <LogLine key={i} event={log} />
            ))}
          </div>
        </div>
      )}

      {error && (
        <div className="p-4 card border-danger/40 bg-danger-muted text-danger text-sm font-mono mb-6 glow-danger">
          {error}
        </div>
      )}

      {analysis && (
        <>
          <ScoreHeader analysis={analysis} />

          <div className="flex gap-1 mt-8 mb-6 border-b border-edge">
            <TabBtn active={tab === 'dashboard'} onClick={() => setTab('dashboard')}>Dashboard</TabBtn>
            <TabBtn active={tab === 'docs'} onClick={() => setTab('docs')}>
              Documentation
              {!documentation && !!runId && !docFailed && <span className="ml-2 inline-block w-1.5 h-1.5 rounded-full bg-accent animate-glow-pulse" />}
            </TabBtn>
            <TabBtn active={tab === 'analysis'} onClick={() => setTab('analysis')}>
              Analysis
              {!interpretation && !!runId && !analysisFailed && <span className="ml-2 inline-block w-1.5 h-1.5 rounded-full bg-accent animate-glow-pulse" />}
            </TabBtn>
          </div>

          {tab === 'dashboard' && <Dashboard analysis={analysis} />}
          {tab === 'docs' && <DocTab data={documentation ?? null} loading={!!runId && !documentation && !docFailed && docRetries < maxRetries} failed={docFailed || docRetries >= maxRetries} retries={docRetries} />}
          {tab === 'analysis' && <AnalysisTab data={interpretation ?? null} loading={!!runId && !interpretation && !analysisFailed && analysisRetries < maxRetries} failed={analysisFailed || analysisRetries >= maxRetries} retries={analysisRetries} />}
        </>
      )}
    </div>
  )
}

function phaseLabel(phase: string): string {
  const labels: Record<string, string> = {
    init: 'Initializing...',
    discovery: 'Discovering endpoints...',
    probes: 'Generating probes...',
    execute: 'Executing requests...',
    ingest: 'Processing evidence...',
    evaluate: 'Running evaluations...',
    interpret: 'Generating AI documentation...',
    done: 'Analysis complete!',
  }
  return labels[phase] ?? phase
}

function LogLine({ event }: { event: ProgressEvent }) {
  const phaseColors: Record<string, string> = {
    init: 'text-ghost',
    discovery: 'text-info',
    probes: 'text-purple-400',
    execute: 'text-accent-dim',
    ingest: 'text-ghost',
    evaluate: 'text-warn',
    interpret: 'text-ok',
    done: 'text-accent',
  }

  const isExecLine = event.phase === 'execute' && event.current
  const color = phaseColors[event.phase] ?? 'text-ghost'

  if (isExecLine) {
    const msg = event.message
    const arrow = msg.indexOf('\u2192')
    const statusPart = arrow > 0 ? msg.slice(arrow + 1).trim() : ''
    const isError = statusPart.startsWith('5') || statusPart.startsWith('ERR')
    const isClientError = statusPart.startsWith('4')

    return (
      <div className="flex items-center gap-1 py-0.5 opacity-70 hover:opacity-100 transition-opacity">
        <span className="text-ghost-faint w-14 text-right shrink-0">{event.current}/{event.total}</span>
        <span className={`${isError ? 'text-danger' : isClientError ? 'text-warn' : 'text-ok/70'}`}>{msg}</span>
      </div>
    )
  }

  return (
    <div className={`py-0.5 ${color}`}>
      {event.message}
    </div>
  )
}

function TabBtn({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button onClick={onClick}
      className={`tab-btn ${active ? 'tab-btn-active' : 'tab-btn-inactive'}`}>
      {children}
    </button>
  )
}

function ScoreHeader({ analysis }: { analysis: Analysis }) {
  const sec = analysis.security
  const cov = analysis.coverage
  const ps = analysis.probes_summary

  const statusSegments = Object.entries(analysis.performance.status_histogram ?? {}).reduce(
    (acc, [code, count]) => {
      if (code.startsWith('2')) acc[0] = { ...acc[0], value: acc[0].value + count }
      else if (code.startsWith('3')) acc[1] = { ...acc[1], value: acc[1].value + count }
      else if (code.startsWith('4')) acc[2] = { ...acc[2], value: acc[2].value + count }
      else acc[3] = { ...acc[3], value: acc[3].value + count }
      return acc
    },
    [
      { label: '2xx', value: 0, color: '#3dd68c' },
      { label: '3xx', value: 0, color: '#52a8ff' },
      { label: '4xx', value: 0, color: '#ffb224' },
      { label: '5xx', value: 0, color: '#ff4f5e' },
    ]
  )

  const probeSegments = [
    { label: 'Injection', value: ps?.injection_probes ?? 0, color: '#ff4f5e' },
    { label: 'Malformed', value: ps?.malformed_probes ?? 0, color: '#ffb224' },
    { label: 'Boundary', value: ps?.boundary_probes ?? 0, color: '#ff8c42' },
    { label: 'Method', value: ps?.method_tamper_probes ?? 0, color: '#a78bfa' },
    { label: 'Hidden EP', value: ps?.hidden_endpoint_probes ?? 0, color: '#f472b6' },
    { label: 'Large', value: ps?.large_payload_probes ?? 0, color: '#52a8ff' },
    { label: 'CT Mismatch', value: ps?.content_type_probes ?? 0, color: '#22d3ee' },
    { label: 'No-Auth', value: ps?.no_auth_probes ?? 0, color: '#f87171' },
  ]

  return (
    <div className="mt-6 space-y-4 animate-fade-up">
      <div className="grid grid-cols-4 gap-4">
        {/* Overall score */}
        <div className="p-5 card flex flex-col items-center justify-center stagger-1 animate-fade-up">
          <ScoreRing score={analysis.score} grade={analysis.grade} size={130} stroke={12} />
          <span className="text-[10px] font-display font-semibold tracking-wider text-ghost mt-3 uppercase">Overall Score ({analysis.grade})</span>
        </div>

        {/* Response distribution */}
        <div className="p-5 card flex flex-col items-center gap-3 stagger-2 animate-fade-up">
          <DonutChart segments={statusSegments} size={100} stroke={14}>
            <span className="text-lg font-display font-bold text-zinc-100">{analysis.performance.total_requests}</span>
            <span className="text-[10px] font-mono text-ghost">requests</span>
          </DonutChart>
          <DonutLegend segments={statusSegments} />
        </div>

        {/* Security + Coverage + Contract rings */}
        <div className="p-5 card flex flex-col items-center gap-3 stagger-3 animate-fade-up">
          <div className="flex gap-4">
            <div className="flex flex-col items-center gap-1">
              <PercentRing value={sec.score} size={60} stroke={6}
                color={sec.score >= 90 ? '#3dd68c' : sec.score >= 70 ? '#ffb224' : sec.score >= 50 ? '#ff8c42' : '#ff4f5e'} />
              <span className="text-[10px] font-display font-semibold text-ghost tracking-wide">Security</span>
            </div>
            <div className="flex flex-col items-center gap-1">
              <PercentRing value={cov.coverage_rate * 100} size={60} stroke={6}
                color={cov.coverage_rate >= 0.9 ? '#3dd68c' : cov.coverage_rate >= 0.7 ? '#52a8ff' : cov.coverage_rate >= 0.5 ? '#ffb224' : '#ff4f5e'} />
              <span className="text-[10px] font-display font-semibold text-ghost tracking-wide">Coverage</span>
            </div>
          </div>
          <div className="flex flex-col items-center gap-1">
            <PercentRing value={analysis.contract.compliance_rate * 100} size={60} stroke={6}
              color={analysis.contract.compliance_rate >= 0.95 ? '#3dd68c' : analysis.contract.compliance_rate >= 0.8 ? '#ffb224' : '#ff4f5e'} />
            <span className="text-[10px] font-display font-semibold text-ghost tracking-wide">Contract</span>
          </div>
        </div>

        {/* Latency bars */}
        <div className="p-5 card flex flex-col justify-center gap-2 stagger-4 animate-fade-up">
          <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Latency</p>
          <HBarChart bars={[
            { label: 'P50', value: analysis.performance.p50_ms, color: '#3dd68c', suffix: 'ms' },
            { label: 'P95', value: analysis.performance.p95_ms, color: '#ffb224', suffix: 'ms' },
            { label: 'P99', value: analysis.performance.p99_ms, color: '#ff4f5e', suffix: 'ms' },
          ]} height={20} />
          <p className="text-[10px] font-mono text-ghost-faint mt-1">
            Success rate: {(analysis.performance.success_rate * 100).toFixed(1)}%
          </p>
        </div>
      </div>

      {/* Probes summary */}
      {ps && ps.total_probes > 0 && (
        <div className="p-5 card animate-fade-up stagger-5">
          <div className="flex items-center justify-between mb-3">
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase">{ps.total_probes} Probes Executed</p>
            <p className="text-xs font-mono text-ghost-faint">{analysis.discovery.endpoints_count} endpoints, {analysis.discovery.framework} framework</p>
          </div>
          <StackedBar segments={probeSegments} height={24} />
          <div className="mt-3">
            <DonutLegend segments={probeSegments} />
          </div>
        </div>
      )}
    </div>
  )
}

function Dashboard({ analysis }: { analysis: Analysis }) {
  const sec = analysis.security
  const perf = analysis.performance
  const cov = analysis.coverage
  const beh = analysis.behavior

  const severitySegments = [
    { label: 'Critical', value: sec.summary.critical, color: '#dc2626' },
    { label: 'High', value: sec.summary.high, color: '#ff4f5e' },
    { label: 'Medium', value: sec.summary.medium, color: '#ffb224' },
    { label: 'Low', value: sec.summary.low, color: '#53566e' },
  ]

  const slowest = (perf.slowest_endpoints ?? []).slice(0, 8)
  const maxLatency = Math.max(...slowest.map(e => e.timing_ms), perf.p99_ms, 1)

  return (
    <div className="space-y-8 animate-fade-in">
      {/* Performance + Security row */}
      <div className="grid grid-cols-2 gap-4">
        <Section title="Status Codes">
          <div className="space-y-1.5">
            {Object.entries(perf.status_histogram ?? {}).sort().map(([code, count]) => {
              const total = perf.total_requests || 1
              const pct = (count / total) * 100
              const color = code.startsWith('2') ? '#3dd68c' : code.startsWith('3') ? '#52a8ff' : code.startsWith('4') ? '#ffb224' : '#ff4f5e'
              return (
                <div key={code} className="flex items-center gap-2">
                  <span className="text-xs font-mono w-8 text-ghost">{code}</span>
                  <div className="flex-1 bg-surface rounded-sm overflow-hidden h-5">
                    <div className="h-full rounded-sm flex items-center pl-2 transition-all duration-500"
                      style={{ width: `${Math.max(pct, 3)}%`, background: `linear-gradient(90deg, ${color}80, ${color})` }}>
                      {pct > 12 && <span className="text-[10px] font-mono font-bold text-obsidian/80">{count}</span>}
                    </div>
                  </div>
                  {pct <= 12 && <span className="text-xs font-mono text-ghost">{count}</span>}
                  <span className="text-[10px] font-mono text-ghost-faint w-10 text-right">{pct.toFixed(1)}%</span>
                </div>
              )
            })}
          </div>
        </Section>

        <Section title={`Security Findings (${sec.summary.total})`}>
          <div className="flex gap-4 items-start">
            <DonutChart segments={severitySegments} size={90} stroke={12}>
              <span className="text-lg font-display font-bold text-zinc-100">{sec.summary.total}</span>
            </DonutChart>
            <div className="flex-1 space-y-2">
              {severitySegments.map((s, i) => (
                <div key={i} className="flex items-center gap-2">
                  <span className="w-2 h-2 rounded-full shrink-0" style={{ background: s.color, boxShadow: s.value > 0 ? `0 0 6px ${s.color}40` : 'none' }} />
                  <span className="text-xs font-body text-ghost flex-1">{s.label}</span>
                  <span className="text-sm font-display font-bold" style={{ color: s.value > 0 ? s.color : '#2e3148' }}>{s.value}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="mt-3">
            <StackedBar segments={severitySegments} height={20} />
          </div>
        </Section>
      </div>

      {/* Slowest endpoints */}
      {slowest.length > 0 && (
        <Section title="Slowest Endpoints">
          <HBarChart bars={slowest.map(ep => ({
            label: `${ep.method} ${ep.path.length > 20 ? ep.path.slice(0, 20) + '...' : ep.path}`,
            value: ep.timing_ms,
            max: maxLatency,
            color: ep.timing_ms > perf.p95_ms ? '#ff4f5e' : ep.timing_ms > perf.p50_ms ? '#ffb224' : '#3dd68c',
            suffix: 'ms',
          }))} height={22} />
          <div className="flex gap-4 mt-3">
            <span className="text-[10px] font-mono text-ghost-faint flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-ok" /> &lt; P50 ({perf.p50_ms}ms)</span>
            <span className="text-[10px] font-mono text-ghost-faint flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-warn" /> P50–P95</span>
            <span className="text-[10px] font-mono text-ghost-faint flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-danger" /> &gt; P95 ({perf.p95_ms}ms)</span>
          </div>
        </Section>
      )}

      {/* Behavior quality grid */}
      <Section title="Behavior Analysis">
        <div className="grid grid-cols-4 gap-3">
          <BehaviorCard title="Input Validation" obs={beh.invalid_input} />
          <BehaviorCard title="Auth Enforcement" obs={beh.missing_auth} />
          <BehaviorCard title="404 Handling" obs={beh.not_found} />
          <BehaviorCard title="Error Consistency" obs={beh.error_consistency} />
        </div>
      </Section>

      {/* Coverage details */}
      <Section title={`Coverage — ${cov.tested_endpoints}/${cov.total_endpoints} endpoints`}>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">By Method</p>
            {(cov.by_method ?? []).map(m => (
              <div key={m.method} className="flex items-center gap-2 mb-1.5">
                <span className="text-xs font-mono w-14 text-ghost">{m.method}</span>
                <div className="flex-1 bg-surface rounded-sm overflow-hidden h-4">
                  <div className="h-full rounded-sm transition-all duration-500 flex items-center justify-end pr-1.5"
                    style={{ width: `${Math.max(m.rate * 100, 2)}%`, background: 'linear-gradient(90deg, #52a8ff80, #52a8ff)' }}>
                    {m.rate > 0.2 && <span className="text-[9px] font-mono font-bold text-obsidian/90">{(m.rate * 100).toFixed(0)}%</span>}
                  </div>
                </div>
                <span className="text-[10px] font-mono text-ghost w-12 text-right">{m.tested}/{m.total}</span>
              </div>
            ))}
          </div>
          <div>
            <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-2">Status Codes Observed</p>
            <div className="flex flex-wrap gap-1.5">
              {(cov.status_codes_observed ?? []).map(sc => (
                <span key={sc.code} className={`px-2.5 py-1 rounded-sm text-xs font-mono transition-colors ${sc.observed ? 'bg-ok-muted text-ok border border-ok/20' : 'bg-surface text-ghost-faint border border-edge'}`}>
                  {sc.code}
                </span>
              ))}
            </div>
            {(cov.untested ?? []).length > 0 && (
              <div className="mt-3">
                <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1">Untested Endpoints</p>
                <div className="flex flex-wrap gap-1">
                  {cov.untested.slice(0, 6).map((u, i) => (
                    <span key={i} className="px-2 py-0.5 rounded-sm text-[10px] font-mono bg-danger-muted text-danger border border-danger/20">
                      {u.method} {u.path}
                    </span>
                  ))}
                  {cov.untested.length > 6 && <span className="text-[10px] font-mono text-ghost-faint">+{cov.untested.length - 6} more</span>}
                </div>
              </div>
            )}
          </div>
        </div>
      </Section>

      {/* Inferred data models */}
      {(beh.inferred_models ?? []).length > 0 && (
        <Section title={`Inferred Data Models (${beh.inferred_models.length})`}>
          <div className="grid grid-cols-2 gap-3">
            {beh.inferred_models.map((m, i) => (
              <div key={i} className="p-4 card">
                <h4 className="text-sm font-display font-semibold text-accent mb-2">{m.name}</h4>
                <div className="space-y-1">
                  {m.fields.map((f, j) => (
                    <div key={j} className="flex items-center gap-2 text-xs">
                      <span className="text-zinc-300 font-mono">{f.name}</span>
                      <span className="px-1.5 py-0.5 rounded-sm bg-surface text-ghost text-[10px] font-mono">{f.json_type}</span>
                      {f.example && <span className="text-ghost-faint truncate max-w-[180px] italic font-mono">{f.example}</span>}
                    </div>
                  ))}
                </div>
                <p className="text-[10px] font-mono text-ghost-faint mt-2">Seen from: {m.seen_from.join(', ')}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {/* Security findings list */}
      {(sec.findings ?? []).length > 0 && (
        <Section title="Findings Detail">
          <div className="space-y-2 max-h-[500px] overflow-y-auto pr-1">
            {(sec.findings ?? []).map((f, i) => (
              <div key={i} className="p-4 card">
                <div className="flex items-center gap-2 mb-1.5">
                  <SeverityBadge severity={f.severity} />
                  <span className="text-xs font-mono text-ghost-faint">{f.id}</span>
                  <span className="text-sm font-display font-medium">{f.title}</span>
                </div>
                <p className="text-xs font-body text-ghost leading-relaxed">{f.description}</p>
                {f.endpoint && <p className="text-xs text-ghost-faint mt-1 font-mono">{f.endpoint}</p>}
                <p className="text-xs text-accent-dim mt-1.5 font-body">{f.remediation}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {/* Contract */}
      <Section title={`Contract (${(analysis.contract.compliance_rate * 100).toFixed(0)}% compliant)`}>
        {(analysis.contract.violations ?? []).length === 0 ? (
          <div className="p-5 card border-ok/20 bg-ok-muted text-center">
            <p className="text-sm text-ok font-display font-semibold">All contract checks passed</p>
            <p className="text-xs font-mono text-ghost mt-1">{analysis.contract.total_checks} checks executed</p>
          </div>
        ) : (
          <div className="space-y-2">
            {(analysis.contract.violations ?? []).map((v, i) => (
              <div key={i} className="p-4 card">
                <p className="text-sm font-mono mb-1 text-zinc-300">{v.endpoint} <span className="text-ghost">({v.status_code})</span></p>
                <p className="text-xs font-body text-ghost">{v.description}</p>
                <div className="flex gap-3 mt-2">
                  <span className="text-xs font-mono px-2.5 py-1 rounded-sm bg-danger-muted text-danger">Got: {v.actual}</span>
                  <span className="text-xs font-mono px-2.5 py-1 rounded-sm bg-ok-muted text-ok">Expected: {v.expected}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>

      {/* Endpoint behavior table */}
      {(beh.endpoint_behavior ?? []).length > 0 && (
        <Section title="Endpoint Behavior">
          <div className="overflow-x-auto card">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-ghost bg-surface/80 font-display tracking-wide uppercase text-[10px]">
                  <th className="text-left py-3 px-4">Endpoint</th>
                  <th className="text-right py-3 px-4">Requests</th>
                  <th className="text-left py-3 px-4 w-36">Latency</th>
                  <th className="text-right py-3 px-4">Errors</th>
                  <th className="text-center py-3 px-4">Auth</th>
                  <th className="text-left py-3 px-4">Responses</th>
                </tr>
              </thead>
              <tbody>
                {beh.endpoint_behavior.map((ep, i) => {
                  const epMaxLatency = Math.max(...beh.endpoint_behavior.map(e => e.avg_latency_ms), 1)
                  const pct = (ep.avg_latency_ms / epMaxLatency) * 100
                  const latColor = ep.avg_latency_ms > perf.p95_ms ? '#ff4f5e' : ep.avg_latency_ms > perf.p50_ms ? '#ffb224' : '#3dd68c'
                  return (
                    <tr key={i} className="border-t border-edge/50 hover:bg-surface-hover transition-colors">
                      <td className="py-2.5 px-4 font-mono text-zinc-300">{ep.endpoint}</td>
                      <td className="py-2.5 px-4 text-right font-mono text-ghost">{ep.request_count}</td>
                      <td className="py-2.5 px-4">
                        <div className="flex items-center gap-2">
                          <div className="flex-1 bg-surface rounded-sm h-2 overflow-hidden">
                            <div className="h-full rounded-sm transition-all" style={{ width: `${pct}%`, background: latColor }} />
                          </div>
                          <span className="text-[10px] font-mono text-ghost w-12 text-right">{ep.avg_latency_ms}ms</span>
                        </div>
                      </td>
                      <td className="py-2.5 px-4 text-right font-mono">
                        {ep.error_count > 0 ? <span className="text-danger font-semibold">{ep.error_count}</span> : <span className="text-ghost-faint">0</span>}
                      </td>
                      <td className="py-2.5 px-4 text-center">
                        {ep.requires_auth
                          ? <span className="px-2 py-0.5 rounded-sm bg-warn-muted text-warn text-[10px] font-display font-semibold tracking-wide">AUTH</span>
                          : <span className="text-ghost-faint">—</span>}
                      </td>
                      <td className="py-2.5 px-4">
                        <div className="flex gap-1 flex-wrap">
                          {Object.entries(ep.status_codes).sort().map(([code, count]) => {
                            const n = Number(code)
                            const cls = n < 300 ? 'bg-ok-muted text-ok' : n < 400 ? 'bg-info-muted text-info' : n < 500 ? 'bg-warn-muted text-warn' : 'bg-danger-muted text-danger'
                            return <span key={code} className={`px-1.5 py-0.5 rounded-sm text-[10px] font-mono ${cls}`}>{code}:{count}</span>
                          })}
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </Section>
      )}
    </div>
  )
}

function BehaviorCard({ title, obs }: { title: string; obs: { quality: string; summary: string; tested: number } }) {
  const config: Record<string, { border: string; bg: string; text: string; dot: string }> = {
    good:    { border: 'border-ok/30', bg: 'bg-ok-muted', text: 'text-ok', dot: 'bg-ok' },
    mixed:   { border: 'border-warn/30', bg: 'bg-warn-muted', text: 'text-warn', dot: 'bg-warn' },
    poor:    { border: 'border-danger/30', bg: 'bg-danger-muted', text: 'text-danger', dot: 'bg-danger' },
    unknown: { border: 'border-edge', bg: 'bg-surface', text: 'text-ghost', dot: 'bg-ghost' },
  }
  const c = config[obs.quality] ?? config.unknown
  return (
    <div className={`p-4 rounded-sm border ${c.border} ${c.bg} transition-colors hover:brightness-110`}>
      <div className="flex items-center gap-2 mb-2">
        <span className={`w-2 h-2 rounded-full ${c.dot}`} style={{ boxShadow: obs.quality !== 'unknown' ? `0 0 6px currentColor` : 'none' }} />
        <span className="text-xs font-display font-semibold text-zinc-300 flex-1 tracking-wide">{title}</span>
        <span className={`text-[10px] font-display font-bold uppercase tracking-wider px-2 py-0.5 rounded-sm ${c.text} bg-obsidian/40`}>{obs.quality}</span>
      </div>
      <p className="text-xs font-body text-ghost leading-relaxed">{obs.summary}</p>
      {obs.tested > 0 && <p className="text-[10px] font-mono text-ghost-faint mt-1.5">{obs.tested} probes tested</p>}
    </div>
  )
}

function LLMLoadingState({ label, loading, failed, retries }: { label: string; loading: boolean; failed: boolean; retries: number }) {
  if (failed) {
    return (
      <div className="p-10 text-center animate-fade-in">
        <p className="text-sm text-danger font-display font-semibold mb-1">Could not generate {label}</p>
        <p className="text-xs font-mono text-ghost">The LLM timed out or failed. Dashboard data is still available above.</p>
      </div>
    )
  }
  if (loading) {
    return (
      <div className="p-10 text-center animate-fade-in">
        <Spinner />
        <p className="text-sm font-display text-ghost mt-4">Generating {label}...</p>
        <p className="text-xs font-mono text-ghost-faint mt-1">Running in background, may take 1-3 minutes</p>
        <div className="mt-4 w-48 mx-auto bg-surface rounded-sm h-1.5 overflow-hidden">
          <div className="h-full rounded-sm transition-all duration-1000"
            style={{
              width: `${Math.min((retries / 24) * 100, 95)}%`,
              background: 'linear-gradient(90deg, #00cc8860, #00ffaa)',
            }} />
        </div>
      </div>
    )
  }
  return null
}

function DocTab({ data, loading, failed, retries }: { data: LLMInterpretation | null; loading: boolean; failed: boolean; retries: number }) {
  if (!data) return <LLMLoadingState label="documentation" loading={loading} failed={failed} retries={retries} />

  const ov = data.overview
  const models = data.data_models ?? []
  const flows = data.flows ?? []
  const questions = data.open_questions ?? []

  return (
    <div className="space-y-8 animate-fade-in">
      {ov && (
        <Section title={ov.service_name}>
          <p className="text-sm font-body text-zinc-300 mb-3">{ov.description}</p>
          <div className="flex gap-4 mt-2">
            <span className="text-xs font-mono px-3 py-1.5 rounded-sm bg-info-muted text-info border border-info/20">{ov.framework}</span>
            <span className="text-xs font-mono text-ghost">{ov.total_endpoints} endpoints</span>
          </div>
          {ov.architecture_notes && (
            <div className="mt-4 p-4 card">
              <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase mb-1.5">Architecture</p>
              <p className="text-xs font-body text-ghost leading-relaxed">{ov.architecture_notes}</p>
            </div>
          )}
        </Section>
      )}

      {models.length > 0 && (
        <Section title={`Data Models (${models.length})`}>
          <div className="grid grid-cols-2 gap-3">
            {models.map((m, i) => (
              <div key={i} className="p-4 card">
                <h4 className="text-sm font-display font-semibold text-accent mb-1.5">{m.name}</h4>
                <p className="text-xs font-body text-ghost mb-2">{m.description}</p>
                <div className="space-y-1 mb-2">
                  {m.fields.map((f, j) => <p key={j} className="text-xs font-mono text-zinc-300">{f}</p>)}
                </div>
                <p className="text-[10px] font-mono text-ghost-faint">Used by: {m.used_by.join(', ')}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {flows.length > 0 && (
        <Section title={`Service Flows (${flows.length})`}>
          <div className="space-y-4">
            {flows.map((f, i) => (
              <div key={i} className="p-5 card">
                <div className="flex items-center gap-2 mb-2">
                  <ImportanceBadge importance={f.importance} />
                  <h4 className="font-display font-semibold tracking-wide">{f.name}</h4>
                </div>
                <p className="text-sm font-body text-zinc-300 mb-2">{f.description}</p>
                <p className="text-xs font-body text-ghost mb-3 leading-relaxed">{f.sequence}</p>
                <div className="flex flex-wrap gap-1 mb-3">
                  {f.endpoints.map((ep, j) => <span key={j} className="px-2.5 py-1 bg-surface rounded-sm text-xs font-mono border border-edge">{ep}</span>)}
                </div>
                {(f.example_requests ?? []).length > 0 && (
                  <div className="space-y-2">
                    <p className="text-[10px] font-display font-semibold tracking-wider text-ghost uppercase">Example Requests:</p>
                    {(f.example_requests ?? []).map((ex, k) => (
                      <div key={k} className="bg-obsidian rounded-sm p-4 border border-edge/50">
                        <p className="text-xs font-body text-ghost mb-1">{ex.step}</p>
                        <div className="font-mono text-xs">
                          <span className="text-info">{ex.method}</span> <span className="text-zinc-300">{ex.path}</span>
                          {ex.headers && Object.keys(ex.headers).length > 0 && (
                            <div className="text-ghost-faint mt-1">{Object.entries(ex.headers).map(([k, v]) => <p key={k}>{k}: {v}</p>)}</div>
                          )}
                          {ex.body != null && <pre className="text-ghost mt-1 whitespace-pre-wrap">{typeof ex.body === 'string' ? ex.body : JSON.stringify(ex.body, null, 2)}</pre>}
                        </div>
                        <div className="mt-1.5 text-xs">
                          <span className="text-ok font-mono">Expected: {ex.expected_status}</span>
                          {ex.expected_response_snippet != null && <pre className="text-ghost mt-1 whitespace-pre-wrap font-mono">{typeof ex.expected_response_snippet === 'string' ? ex.expected_response_snippet : JSON.stringify(ex.expected_response_snippet, null, 2)}</pre>}
                          {ex.notes && <p className="text-warn/70 mt-1 font-body">{ex.notes}</p>}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        </Section>
      )}

      {questions.length > 0 && (
        <Section title="Open Questions">
          {questions.map((q, i) => (
            <div key={i} className="p-4 card mb-2">
              <p className="text-sm font-body text-zinc-300">{q.question}</p>
              {q.why_missing && <p className="text-xs font-mono text-ghost mt-1">{q.why_missing}</p>}
            </div>
          ))}
        </Section>
      )}
    </div>
  )
}

function AnalysisTab({ data, loading, failed, retries }: { data: LLMInterpretation | null; loading: boolean; failed: boolean; retries: number }) {
  if (!data) return <LLMLoadingState label="analysis" loading={loading} failed={failed} retries={retries} />

  const secAssess = data.security_assessment
  const behAssess = data.behavior_assessment
  const facts = data.facts ?? []
  const inferences = data.inferences ?? []
  const improvements = data.improvements ?? []
  const tests = data.tests ?? []
  const questions = data.open_questions ?? []

  return (
    <div className="space-y-8 animate-fade-in">
      {secAssess && (
        <Section title="Security Assessment">
          <div className={`p-5 rounded-sm border ${secAssess.overall_risk === 'critical' ? 'border-danger/40 bg-danger-muted' : secAssess.overall_risk === 'high' ? 'border-danger/30 bg-danger-muted' : secAssess.overall_risk === 'medium' ? 'border-warn/30 bg-warn-muted' : 'border-ok/30 bg-ok-muted'}`}>
            <div className="flex items-center gap-2 mb-3">
              <SeverityBadge severity={secAssess.overall_risk} />
              <span className="text-sm font-display font-semibold tracking-wide">Overall Risk</span>
            </div>
            <p className="text-sm font-body text-zinc-300 mb-3">{secAssess.summary}</p>
            {secAssess.attack_surface && <p className="text-xs font-body text-ghost mb-3">{secAssess.attack_surface}</p>}
            <div className="grid grid-cols-2 gap-4">
              {secAssess.critical_findings?.length > 0 && (
                <div>
                  <p className="text-xs text-danger font-display font-semibold tracking-wide mb-1.5">Critical Findings</p>
                  {secAssess.critical_findings.map((f, i) => <p key={i} className="text-xs font-body text-ghost mb-1">- {f}</p>)}
                </div>
              )}
              {secAssess.positive_findings?.length > 0 && (
                <div>
                  <p className="text-xs text-ok font-display font-semibold tracking-wide mb-1.5">Positive Findings</p>
                  {secAssess.positive_findings.map((f, i) => <p key={i} className="text-xs font-body text-ghost mb-1">- {f}</p>)}
                </div>
              )}
            </div>
          </div>
        </Section>
      )}

      {behAssess && (
        <Section title="Behavior Assessment">
          <div className="grid grid-cols-2 gap-3">
            <AssessmentCard title="Input Validation" text={behAssess.input_validation} />
            <AssessmentCard title="Auth Enforcement" text={behAssess.auth_enforcement} />
            <AssessmentCard title="Error Handling" text={behAssess.error_handling} />
            <AssessmentCard title="Robustness" text={behAssess.robustness} />
          </div>
        </Section>
      )}

      {facts.length > 0 && (
        <Section title={`Observed Facts (${facts.length})`}>
          <div className="space-y-2">
            {facts.map((f, i) => (
              <div key={i} className="p-4 card flex items-start gap-3">
                <div className="shrink-0 mt-0.5">
                  <div className="w-8 h-8 rounded-sm flex items-center justify-center text-xs font-mono font-bold"
                    style={{ background: `rgba(${Math.round((1 - f.confidence) * 255)}, ${Math.round(f.confidence * 200)}, 80, 0.12)`, color: `rgba(${Math.round((1 - f.confidence) * 255)}, ${Math.round(f.confidence * 200)}, 80, 0.9)` }}>
                    {Math.round(f.confidence * 100)}
                  </div>
                </div>
                <p className="text-xs font-body text-zinc-300 leading-relaxed">{f.text}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {inferences.length > 0 && (
        <Section title={`Inferences (${inferences.length})`}>
          <div className="space-y-2">
            {inferences.map((inf, i) => (
              <div key={i} className="p-4 card">
                <div className="flex items-center gap-2 mb-1.5">
                  <span className="text-xs px-2.5 py-1 bg-purple-500/10 text-purple-400 rounded-sm font-mono border border-purple-500/20">{inf.rule_of_inference}</span>
                  <span className="text-[10px] font-mono text-ghost-faint">{Math.round(inf.confidence * 100)}% confidence</span>
                </div>
                <p className="text-xs font-body text-zinc-300 leading-relaxed">{inf.text}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {improvements.length > 0 && (
        <Section title={`Improvements (${improvements.length})`}>
          <div className="space-y-2">
            {improvements.map((imp, i) => (
              <div key={i} className="p-4 card">
                <div className="flex items-center gap-2 mb-1.5">
                  <SeverityBadge severity={imp.severity} />
                  <span className="text-xs font-mono px-2.5 py-1 bg-surface rounded-sm border border-edge">{imp.category}</span>
                  <span className="text-sm font-display font-medium">{imp.title}</span>
                </div>
                <p className="text-xs font-body text-ghost">{imp.description}</p>
                <p className="text-xs font-body text-accent-dim mt-1.5">{imp.remediation}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {tests.length > 0 && (
        <Section title={`Suggested Tests (${tests.length})`}>
          <div className="space-y-3">
            {tests.map((t, i) => (
              <div key={i} className="p-5 card">
                <div className="flex items-center gap-2 mb-2">
                  <ImportanceBadge importance={t.importance} />
                  <h4 className="text-sm font-display font-medium">{t.name}</h4>
                  <span className="text-xs font-mono text-ghost-faint">({t.flow})</span>
                </div>
                <p className="text-xs font-body text-zinc-300 mb-3">{t.description}</p>
                <div className="bg-obsidian rounded-sm p-4 font-mono text-xs border border-edge/50">
                  <span className="text-info">{t.request.method}</span> <span className="text-zinc-300">{t.request.path}</span>
                  {t.request.body != null && <pre className="text-ghost mt-1 whitespace-pre-wrap">{String(JSON.stringify(t.request.body, null, 2))}</pre>}
                </div>
                <div className="mt-2 text-xs font-mono">
                  <span className="text-ok">Expected: {t.expected.status}</span>
                  <span className="text-ghost ml-2 font-body">{t.expected.description}</span>
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}

      {questions.length > 0 && (
        <Section title="Open Questions">
          {questions.map((q, i) => (
            <div key={i} className="p-4 card mb-2">
              <p className="text-sm font-body text-zinc-300">{q.question}</p>
              {q.why_missing && <p className="text-xs font-mono text-ghost mt-1">{q.why_missing}</p>}
            </div>
          ))}
        </Section>
      )}
    </div>
  )
}

function AssessmentCard({ title, text }: { title: string; text: string }) {
  return (
    <div className="p-4 card">
      <p className="text-xs font-display font-semibold tracking-wide text-ghost mb-1.5">{title}</p>
      <p className="text-xs font-body text-zinc-300 leading-relaxed">{text}</p>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="section-title">
        <span>{title}</span>
      </h3>
      {children}
    </div>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  const colors: Record<string, string> = {
    critical: 'bg-danger text-obsidian',
    high: 'bg-danger-muted text-danger border border-danger/30',
    medium: 'bg-warn-muted text-warn border border-warn/30',
    low: 'bg-surface text-ghost border border-edge',
    info: 'bg-surface text-ghost-faint border border-edge',
  }
  return <span className={`badge ${colors[severity] ?? colors.info}`}>{severity}</span>
}

function ImportanceBadge({ importance }: { importance: string }) {
  const colors: Record<string, string> = {
    critical: 'bg-danger text-obsidian',
    high: 'bg-warn-muted text-warn border border-warn/30',
    medium: 'bg-info-muted text-info border border-info/30',
    low: 'bg-surface text-ghost border border-edge',
  }
  return <span className={`badge ${colors[importance] ?? colors.medium}`}>{importance}</span>
}
