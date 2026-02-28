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

  if (tLoading) return <div className="p-6"><Spinner /></div>

  const currentPhase = logs.length > 0 ? logs[logs.length - 1].phase : ''
  const lastExec = [...logs].reverse().find(l => l.phase === 'execute' && l.current)

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <button onClick={() => navigate('/targets')} className="text-xs text-gray-500 hover:text-gray-300 mb-4 block">&larr; All Targets</button>
      <div className="mb-2">
        <h1 className="text-xl font-semibold">{target?.name}</h1>
      </div>
      <p className="text-sm text-gray-500 mb-6">{target?.source.type}: {target?.source.value}</p>

      {running && (
        <div className="mb-6 rounded-lg border border-gray-800 bg-gray-950 overflow-hidden">
          <div className="px-4 py-2 border-b border-gray-800 flex items-center justify-between bg-gray-900/50">
            <div className="flex items-center gap-2">
              <Spinner />
              <span className="text-sm font-medium text-gray-300">
                {phaseLabel(currentPhase)}
              </span>
            </div>
            {lastExec && (
              <span className="text-xs text-gray-500 font-mono">{lastExec.current}/{lastExec.total} requests</span>
            )}
          </div>
          {lastExec && (
            <div className="px-4 py-1 border-b border-gray-800/50">
              <div className="w-full bg-gray-800 rounded-full h-1.5">
                <div
                  className="bg-blue-500 h-1.5 rounded-full transition-all duration-150"
                  style={{ width: `${(lastExec.current! / lastExec.total!) * 100}%` }}
                />
              </div>
            </div>
          )}
          <div ref={logRef} className="max-h-72 overflow-y-auto p-3 font-mono text-xs leading-relaxed">
            {logs.map((log, i) => (
              <LogLine key={i} event={log} />
            ))}
          </div>
        </div>
      )}

      {error && (
        <div className="p-4 rounded-lg border border-red-800 bg-red-900/20 text-red-400 text-sm mb-4">
          {error}
        </div>
      )}

      {analysis && (
        <>
          <ScoreHeader analysis={analysis} />

          <div className="flex gap-1 mt-6 mb-4 border-b border-gray-800">
            <TabBtn active={tab === 'dashboard'} onClick={() => setTab('dashboard')}>Dashboard</TabBtn>
            <TabBtn active={tab === 'docs'} onClick={() => setTab('docs')}>
              Documentation
              {!documentation && !!runId && !docFailed && <span className="ml-1.5 inline-block w-1.5 h-1.5 rounded-full bg-blue-500 animate-pulse" />}
            </TabBtn>
            <TabBtn active={tab === 'analysis'} onClick={() => setTab('analysis')}>
              Analysis
              {!interpretation && !!runId && !analysisFailed && <span className="ml-1.5 inline-block w-1.5 h-1.5 rounded-full bg-blue-500 animate-pulse" />}
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
    init: 'text-gray-500',
    discovery: 'text-cyan-400',
    probes: 'text-purple-400',
    execute: 'text-blue-400',
    ingest: 'text-gray-400',
    evaluate: 'text-yellow-400',
    interpret: 'text-green-400',
    done: 'text-green-300',
  }

  const isExecLine = event.phase === 'execute' && event.current
  const color = phaseColors[event.phase] ?? 'text-gray-400'

  if (isExecLine) {
    const msg = event.message
    const arrow = msg.indexOf('→')
    const statusPart = arrow > 0 ? msg.slice(arrow + 1).trim() : ''
    const isError = statusPart.startsWith('5') || statusPart.startsWith('ERR')
    const isClientError = statusPart.startsWith('4')

    return (
      <div className="flex items-center gap-1 py-0.5 opacity-80 hover:opacity-100">
        <span className="text-gray-600 w-14 text-right shrink-0">{event.current}/{event.total}</span>
        <span className={`${isError ? 'text-red-400' : isClientError ? 'text-yellow-400' : 'text-green-400/70'}`}>{msg}</span>
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
      className={`px-4 py-2 text-sm font-medium border-b-2 transition ${active ? 'border-blue-500 text-white' : 'border-transparent text-gray-500 hover:text-gray-300'}`}>
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
      { label: '2xx', value: 0, color: '#4ade80' },
      { label: '3xx', value: 0, color: '#60a5fa' },
      { label: '4xx', value: 0, color: '#facc15' },
      { label: '5xx', value: 0, color: '#ef4444' },
    ]
  )

  const probeSegments = [
    { label: 'Injection', value: ps?.injection_probes ?? 0, color: '#ef4444' },
    { label: 'Malformed', value: ps?.malformed_probes ?? 0, color: '#facc15' },
    { label: 'Boundary', value: ps?.boundary_probes ?? 0, color: '#fb923c' },
    { label: 'Method', value: ps?.method_tamper_probes ?? 0, color: '#a78bfa' },
    { label: 'Hidden EP', value: ps?.hidden_endpoint_probes ?? 0, color: '#f472b6' },
    { label: 'Large', value: ps?.large_payload_probes ?? 0, color: '#60a5fa' },
    { label: 'CT Mismatch', value: ps?.content_type_probes ?? 0, color: '#22d3ee' },
    { label: 'No-Auth', value: ps?.no_auth_probes ?? 0, color: '#f87171' },
  ]

  return (
    <div className="mt-4 space-y-4">
      <div className="grid grid-cols-4 gap-4">
        {/* Overall score ring */}
        <div className="p-4 rounded-xl border border-gray-800 bg-gray-900/50 flex flex-col items-center justify-center">
          <ScoreRing score={analysis.score} grade={analysis.grade} size={130} stroke={12} />
          <span className="text-xs text-gray-500 mt-2">Overall Score ({analysis.grade})</span>
        </div>

        {/* Response distribution donut */}
        <div className="p-4 rounded-xl border border-gray-800 bg-gray-900/50 flex flex-col items-center gap-3">
          <DonutChart segments={statusSegments} size={100} stroke={14}>
            <span className="text-lg font-bold text-white">{analysis.performance.total_requests}</span>
            <span className="text-[10px] text-gray-500">requests</span>
          </DonutChart>
          <DonutLegend segments={statusSegments} />
        </div>

        {/* Security + Coverage rings side by side */}
        <div className="p-4 rounded-xl border border-gray-800 bg-gray-900/50 flex flex-col items-center gap-3">
          <div className="flex gap-4">
            <div className="flex flex-col items-center gap-1">
              <PercentRing value={sec.score} size={60} stroke={6}
                color={sec.score >= 90 ? '#4ade80' : sec.score >= 70 ? '#facc15' : sec.score >= 50 ? '#fb923c' : '#ef4444'} />
              <span className="text-[10px] text-gray-500">Security</span>
            </div>
            <div className="flex flex-col items-center gap-1">
              <PercentRing value={cov.coverage_rate * 100} size={60} stroke={6}
                color={cov.coverage_rate >= 0.9 ? '#4ade80' : cov.coverage_rate >= 0.7 ? '#60a5fa' : cov.coverage_rate >= 0.5 ? '#facc15' : '#ef4444'} />
              <span className="text-[10px] text-gray-500">Coverage</span>
            </div>
          </div>
          <div className="flex flex-col items-center gap-1">
            <PercentRing value={analysis.contract.compliance_rate * 100} size={60} stroke={6}
              color={analysis.contract.compliance_rate >= 0.95 ? '#4ade80' : analysis.contract.compliance_rate >= 0.8 ? '#facc15' : '#ef4444'} />
            <span className="text-[10px] text-gray-500">Contract</span>
          </div>
        </div>

        {/* Latency bars */}
        <div className="p-4 rounded-xl border border-gray-800 bg-gray-900/50 flex flex-col justify-center gap-2">
          <p className="text-xs text-gray-500 font-semibold mb-1">Latency</p>
          <HBarChart bars={[
            { label: 'P50', value: analysis.performance.p50_ms, color: '#4ade80', suffix: 'ms' },
            { label: 'P95', value: analysis.performance.p95_ms, color: '#facc15', suffix: 'ms' },
            { label: 'P99', value: analysis.performance.p99_ms, color: '#ef4444', suffix: 'ms' },
          ]} height={20} />
          <p className="text-[10px] text-gray-600 mt-1">
            Success rate: {(analysis.performance.success_rate * 100).toFixed(1)}%
          </p>
        </div>
      </div>

      {/* Probes summary bar */}
      {ps && ps.total_probes > 0 && (
        <div className="p-4 rounded-xl border border-gray-800 bg-gray-900/50">
          <div className="flex items-center justify-between mb-3">
            <p className="text-xs text-gray-500 font-semibold">{ps.total_probes} Probes Executed</p>
            <p className="text-xs text-gray-600">{analysis.discovery.endpoints_count} endpoints, {analysis.discovery.framework} framework</p>
          </div>
          <StackedBar segments={probeSegments} height={24} />
          <div className="mt-2">
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
    { label: 'High', value: sec.summary.high, color: '#ef4444' },
    { label: 'Medium', value: sec.summary.medium, color: '#facc15' },
    { label: 'Low', value: sec.summary.low, color: '#6b7280' },
  ]

  const slowest = (perf.slowest_endpoints ?? []).slice(0, 8)
  const maxLatency = Math.max(...slowest.map(e => e.timing_ms), perf.p99_ms, 1)

  return (
    <div className="space-y-6">
      {/* Performance + Security row */}
      <div className="grid grid-cols-2 gap-4">
        {/* Status code details */}
        <Section title="Status Codes">
          <div className="space-y-1.5">
            {Object.entries(perf.status_histogram ?? {}).sort().map(([code, count]) => {
              const total = perf.total_requests || 1
              const pct = (count / total) * 100
              const color = code.startsWith('2') ? '#4ade80' : code.startsWith('3') ? '#60a5fa' : code.startsWith('4') ? '#facc15' : '#ef4444'
              return (
                <div key={code} className="flex items-center gap-2">
                  <span className="text-xs font-mono w-8 text-gray-400">{code}</span>
                  <div className="flex-1 bg-gray-800/60 rounded-full overflow-hidden h-5">
                    <div className="h-full rounded-full flex items-center pl-2 transition-all duration-500"
                      style={{ width: `${Math.max(pct, 3)}%`, background: color }}>
                      {pct > 12 && <span className="text-[10px] font-bold text-black/70">{count}</span>}
                    </div>
                  </div>
                  {pct <= 12 && <span className="text-xs text-gray-500">{count}</span>}
                  <span className="text-[10px] text-gray-600 w-10 text-right">{pct.toFixed(1)}%</span>
                </div>
              )
            })}
          </div>
        </Section>

        {/* Security severity breakdown */}
        <Section title={`Security Findings (${sec.summary.total})`}>
          <div className="flex gap-4 items-start">
            <DonutChart segments={severitySegments} size={90} stroke={12}>
              <span className="text-lg font-bold text-white">{sec.summary.total}</span>
            </DonutChart>
            <div className="flex-1 space-y-2">
              {severitySegments.map((s, i) => (
                <div key={i} className="flex items-center gap-2">
                  <span className="w-2.5 h-2.5 rounded-full shrink-0" style={{ background: s.color }} />
                  <span className="text-xs text-gray-400 flex-1">{s.label}</span>
                  <span className="text-sm font-bold" style={{ color: s.value > 0 ? s.color : '#4b5563' }}>{s.value}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="mt-3">
            <StackedBar segments={severitySegments} height={20} />
          </div>
        </Section>
      </div>

      {/* Slowest endpoints chart */}
      {slowest.length > 0 && (
        <Section title="Slowest Endpoints">
          <HBarChart bars={slowest.map(ep => ({
            label: `${ep.method} ${ep.path.length > 20 ? ep.path.slice(0, 20) + '...' : ep.path}`,
            value: ep.timing_ms,
            max: maxLatency,
            color: ep.timing_ms > perf.p95_ms ? '#ef4444' : ep.timing_ms > perf.p50_ms ? '#facc15' : '#4ade80',
            suffix: 'ms',
          }))} height={22} />
          <div className="flex gap-4 mt-2">
            <span className="text-[10px] text-gray-600 flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-green-400" /> &lt; P50 ({perf.p50_ms}ms)</span>
            <span className="text-[10px] text-gray-600 flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-yellow-400" /> P50–P95</span>
            <span className="text-[10px] text-gray-600 flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-red-400" /> &gt; P95 ({perf.p95_ms}ms)</span>
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
            <p className="text-xs text-gray-500 mb-2">By Method</p>
            {(cov.by_method ?? []).map(m => (
              <div key={m.method} className="flex items-center gap-2 mb-1.5">
                <span className="text-xs font-mono w-14 text-gray-400">{m.method}</span>
                <div className="flex-1 bg-gray-800/60 rounded-full overflow-hidden h-4">
                  <div className="bg-blue-500/80 h-full rounded-full transition-all duration-500 flex items-center justify-end pr-1.5"
                    style={{ width: `${Math.max(m.rate * 100, 2)}%` }}>
                    {m.rate > 0.2 && <span className="text-[9px] font-bold text-white/90">{(m.rate * 100).toFixed(0)}%</span>}
                  </div>
                </div>
                <span className="text-[10px] text-gray-500 w-12 text-right">{m.tested}/{m.total}</span>
              </div>
            ))}
          </div>
          <div>
            <p className="text-xs text-gray-500 mb-2">Status Codes Observed</p>
            <div className="flex flex-wrap gap-1.5">
              {(cov.status_codes_observed ?? []).map(sc => (
                <span key={sc.code} className={`px-2.5 py-1 rounded text-xs font-mono transition-colors ${sc.observed ? 'bg-green-900/40 text-green-400 border border-green-800/50' : 'bg-gray-800/40 text-gray-600 border border-gray-800/50'}`}>
                  {sc.code}
                </span>
              ))}
            </div>
            {(cov.untested ?? []).length > 0 && (
              <div className="mt-3">
                <p className="text-xs text-gray-500 mb-1">Untested Endpoints</p>
                <div className="flex flex-wrap gap-1">
                  {cov.untested.slice(0, 6).map((u, i) => (
                    <span key={i} className="px-2 py-0.5 rounded text-[10px] font-mono bg-red-900/20 text-red-400 border border-red-900/30">
                      {u.method} {u.path}
                    </span>
                  ))}
                  {cov.untested.length > 6 && <span className="text-[10px] text-gray-600">+{cov.untested.length - 6} more</span>}
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
              <div key={i} className="p-3 rounded-lg border border-gray-800 bg-gray-900/30">
                <h4 className="text-sm font-semibold text-blue-400 mb-2">{m.name}</h4>
                <div className="space-y-1">
                  {m.fields.map((f, j) => (
                    <div key={j} className="flex items-center gap-2 text-xs">
                      <span className="text-gray-300 font-mono">{f.name}</span>
                      <span className="px-1.5 py-0.5 rounded bg-gray-800 text-gray-500 text-[10px]">{f.json_type}</span>
                      {f.example && <span className="text-gray-600 truncate max-w-[180px] italic">{f.example}</span>}
                    </div>
                  ))}
                </div>
                <p className="text-[10px] text-gray-600 mt-2">Seen from: {m.seen_from.join(', ')}</p>
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
              <div key={i} className="p-3 rounded-lg border border-gray-800 bg-gray-900/30 hover:bg-gray-900/50 transition-colors">
                <div className="flex items-center gap-2 mb-1">
                  <SeverityBadge severity={f.severity} />
                  <span className="text-xs font-mono text-gray-600">{f.id}</span>
                  <span className="text-sm font-medium">{f.title}</span>
                </div>
                <p className="text-xs text-gray-400">{f.description}</p>
                {f.endpoint && <p className="text-xs text-gray-600 mt-1 font-mono">{f.endpoint}</p>}
                <p className="text-xs text-blue-400 mt-1">{f.remediation}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {/* Contract violations */}
      <Section title={`Contract (${(analysis.contract.compliance_rate * 100).toFixed(0)}% compliant)`}>
        {(analysis.contract.violations ?? []).length === 0 ? (
          <div className="p-4 rounded-lg border border-green-800/50 bg-green-900/10 text-center">
            <p className="text-sm text-green-400 font-medium">All contract checks passed</p>
            <p className="text-xs text-gray-500 mt-1">{analysis.contract.total_checks} checks executed</p>
          </div>
        ) : (
          <div className="space-y-2">
            {(analysis.contract.violations ?? []).map((v, i) => (
              <div key={i} className="p-3 rounded-lg border border-gray-800 bg-gray-900/30">
                <p className="text-sm font-mono mb-1">{v.endpoint} <span className="text-gray-500">({v.status_code})</span></p>
                <p className="text-xs text-gray-400">{v.description}</p>
                <div className="flex gap-3 mt-1.5">
                  <span className="text-xs px-2 py-0.5 rounded bg-red-900/30 text-red-400">Got: {v.actual}</span>
                  <span className="text-xs px-2 py-0.5 rounded bg-green-900/30 text-green-400">Expected: {v.expected}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>

      {/* Endpoint behavior table */}
      {(beh.endpoint_behavior ?? []).length > 0 && (
        <Section title="Endpoint Behavior">
          <div className="overflow-x-auto rounded-lg border border-gray-800">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-gray-500 bg-gray-900/70">
                  <th className="text-left py-2.5 px-3">Endpoint</th>
                  <th className="text-right py-2.5 px-3">Requests</th>
                  <th className="text-left py-2.5 px-3 w-36">Latency</th>
                  <th className="text-right py-2.5 px-3">Errors</th>
                  <th className="text-center py-2.5 px-3">Auth</th>
                  <th className="text-left py-2.5 px-3">Responses</th>
                </tr>
              </thead>
              <tbody>
                {beh.endpoint_behavior.map((ep, i) => {
                  const epMaxLatency = Math.max(...beh.endpoint_behavior.map(e => e.avg_latency_ms), 1)
                  const pct = (ep.avg_latency_ms / epMaxLatency) * 100
                  const latColor = ep.avg_latency_ms > perf.p95_ms ? '#ef4444' : ep.avg_latency_ms > perf.p50_ms ? '#facc15' : '#4ade80'
                  return (
                    <tr key={i} className="border-t border-gray-800/50 hover:bg-gray-900/30 transition-colors">
                      <td className="py-2 px-3 font-mono text-gray-300">{ep.endpoint}</td>
                      <td className="py-2 px-3 text-right text-gray-400">{ep.request_count}</td>
                      <td className="py-2 px-3">
                        <div className="flex items-center gap-2">
                          <div className="flex-1 bg-gray-800/60 rounded-full h-2 overflow-hidden">
                            <div className="h-full rounded-full transition-all" style={{ width: `${pct}%`, background: latColor }} />
                          </div>
                          <span className="text-[10px] text-gray-500 w-12 text-right">{ep.avg_latency_ms}ms</span>
                        </div>
                      </td>
                      <td className="py-2 px-3 text-right">
                        {ep.error_count > 0 ? <span className="text-red-400 font-semibold">{ep.error_count}</span> : <span className="text-gray-600">0</span>}
                      </td>
                      <td className="py-2 px-3 text-center">
                        {ep.requires_auth
                          ? <span className="px-1.5 py-0.5 rounded bg-yellow-900/30 text-yellow-400 text-[10px]">auth</span>
                          : <span className="text-gray-700">—</span>}
                      </td>
                      <td className="py-2 px-3">
                        <div className="flex gap-1 flex-wrap">
                          {Object.entries(ep.status_codes).sort().map(([code, count]) => {
                            const n = Number(code)
                            const cls = n < 300 ? 'bg-green-900/30 text-green-400' : n < 400 ? 'bg-blue-900/30 text-blue-400' : n < 500 ? 'bg-yellow-900/30 text-yellow-400' : 'bg-red-900/30 text-red-400'
                            return <span key={code} className={`px-1.5 py-0.5 rounded text-[10px] font-mono ${cls}`}>{code}:{count}</span>
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
  const config: Record<string, { border: string; bg: string; text: string; dot: string; icon: string }> = {
    good:    { border: 'border-green-800/60', bg: 'bg-green-900/10', text: 'text-green-400', dot: 'bg-green-400', icon: 'check' },
    mixed:   { border: 'border-yellow-800/60', bg: 'bg-yellow-900/10', text: 'text-yellow-400', dot: 'bg-yellow-400', icon: 'warn' },
    poor:    { border: 'border-red-800/60', bg: 'bg-red-900/10', text: 'text-red-400', dot: 'bg-red-400', icon: 'x' },
    unknown: { border: 'border-gray-700', bg: 'bg-gray-900/30', text: 'text-gray-400', dot: 'bg-gray-500', icon: '?' },
  }
  const c = config[obs.quality] ?? config.unknown
  return (
    <div className={`p-3 rounded-lg border ${c.border} ${c.bg} transition-colors hover:brightness-110`}>
      <div className="flex items-center gap-2 mb-1.5">
        <span className={`w-2 h-2 rounded-full ${c.dot}`} />
        <span className="text-xs font-semibold text-gray-300 flex-1">{title}</span>
        <span className={`text-[10px] font-bold uppercase px-1.5 py-0.5 rounded ${c.text} bg-black/20`}>{obs.quality}</span>
      </div>
      <p className="text-xs text-gray-400 leading-relaxed">{obs.summary}</p>
      {obs.tested > 0 && <p className="text-[10px] text-gray-600 mt-1">{obs.tested} probes tested</p>}
    </div>
  )
}

function LLMLoadingState({ label, loading, failed, retries }: { label: string; loading: boolean; failed: boolean; retries: number }) {
  if (failed) {
    return (
      <div className="p-8 text-center">
        <p className="text-sm text-red-400 font-medium mb-1">Could not generate {label}</p>
        <p className="text-xs text-gray-500">The LLM timed out or failed. Dashboard data is still available above.</p>
      </div>
    )
  }
  if (loading) {
    return (
      <div className="p-8 text-center">
        <Spinner />
        <p className="text-sm text-gray-400 mt-3">Generating {label}...</p>
        <p className="text-xs text-gray-600 mt-1">Running in background, may take 1-3 minutes</p>
        <div className="mt-3 w-48 mx-auto bg-gray-800 rounded-full h-1.5 overflow-hidden">
          <div className="bg-blue-500/60 h-full rounded-full transition-all duration-1000"
            style={{ width: `${Math.min((retries / 24) * 100, 95)}%` }} />
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
    <div className="space-y-6">
      {ov && (
        <Section title={ov.service_name}>
          <p className="text-sm text-gray-300 mb-2">{ov.description}</p>
          <div className="flex gap-4 mt-2">
            <span className="text-xs px-2.5 py-1 rounded bg-blue-900/30 text-blue-400 border border-blue-800/40">{ov.framework}</span>
            <span className="text-xs text-gray-500">{ov.total_endpoints} endpoints</span>
          </div>
          {ov.architecture_notes && (
            <div className="mt-3 p-3 rounded border border-gray-800 bg-gray-900/30">
              <p className="text-xs text-gray-500 font-semibold mb-1">Architecture</p>
              <p className="text-xs text-gray-400 leading-relaxed">{ov.architecture_notes}</p>
            </div>
          )}
        </Section>
      )}

      {models.length > 0 && (
        <Section title={`Data Models (${models.length})`}>
          <div className="grid grid-cols-2 gap-3">
            {models.map((m, i) => (
              <div key={i} className="p-3 rounded border border-gray-800 bg-gray-900/30">
                <h4 className="text-sm font-semibold text-blue-400 mb-1">{m.name}</h4>
                <p className="text-xs text-gray-400 mb-2">{m.description}</p>
                <div className="space-y-1 mb-2">
                  {m.fields.map((f, j) => <p key={j} className="text-xs font-mono text-gray-300">{f}</p>)}
                </div>
                <p className="text-xs text-gray-600">Used by: {m.used_by.join(', ')}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {flows.length > 0 && (
        <Section title={`Service Flows (${flows.length})`}>
          <div className="space-y-4">
            {flows.map((f, i) => (
              <div key={i} className="p-4 rounded border border-gray-800 bg-gray-900/30">
                <div className="flex items-center gap-2 mb-2">
                  <ImportanceBadge importance={f.importance} />
                  <h4 className="font-medium">{f.name}</h4>
                </div>
                <p className="text-sm text-gray-300 mb-2">{f.description}</p>
                <p className="text-xs text-gray-400 mb-3 leading-relaxed">{f.sequence}</p>
                <div className="flex flex-wrap gap-1 mb-3">
                  {f.endpoints.map((ep, j) => <span key={j} className="px-2 py-0.5 bg-gray-800 rounded text-xs font-mono">{ep}</span>)}
                </div>
                {(f.example_requests ?? []).length > 0 && (
                  <div className="space-y-2">
                    <p className="text-xs text-gray-500 font-semibold">Example Requests:</p>
                    {(f.example_requests ?? []).map((ex, k) => (
                      <div key={k} className="bg-gray-950 rounded p-3">
                        <p className="text-xs text-gray-400 mb-1">{ex.step}</p>
                        <div className="font-mono text-xs">
                          <span className="text-blue-400">{ex.method}</span> <span className="text-gray-300">{ex.path}</span>
                          {ex.headers && Object.keys(ex.headers).length > 0 && (
                            <div className="text-gray-600 mt-1">{Object.entries(ex.headers).map(([k, v]) => <p key={k}>{k}: {v}</p>)}</div>
                          )}
                          {ex.body != null && <pre className="text-gray-500 mt-1 whitespace-pre-wrap">{typeof ex.body === 'string' ? ex.body : JSON.stringify(ex.body, null, 2)}</pre>}
                        </div>
                        <div className="mt-1 text-xs">
                          <span className="text-green-400">Expected: {ex.expected_status}</span>
                          {ex.expected_response_snippet != null && <pre className="text-gray-500 mt-1 whitespace-pre-wrap">{typeof ex.expected_response_snippet === 'string' ? ex.expected_response_snippet : JSON.stringify(ex.expected_response_snippet, null, 2)}</pre>}
                          {ex.notes && <p className="text-yellow-400/70 mt-1">{ex.notes}</p>}
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
            <div key={i} className="p-3 rounded border border-gray-800 bg-gray-900/30 mb-2">
              <p className="text-sm text-gray-300">{q.question}</p>
              {q.why_missing && <p className="text-xs text-gray-500 mt-1">{q.why_missing}</p>}
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
    <div className="space-y-6">
      {secAssess && (
        <Section title="Security Assessment">
          <div className={`p-4 rounded border ${secAssess.overall_risk === 'critical' ? 'border-red-700 bg-red-900/10' : secAssess.overall_risk === 'high' ? 'border-red-800 bg-red-900/10' : secAssess.overall_risk === 'medium' ? 'border-yellow-800 bg-yellow-900/10' : 'border-green-800 bg-green-900/10'}`}>
            <div className="flex items-center gap-2 mb-2">
              <SeverityBadge severity={secAssess.overall_risk} />
              <span className="text-sm font-semibold">Overall Risk</span>
            </div>
            <p className="text-sm text-gray-300 mb-3">{secAssess.summary}</p>
            {secAssess.attack_surface && <p className="text-xs text-gray-400 mb-3">{secAssess.attack_surface}</p>}
            <div className="grid grid-cols-2 gap-3">
              {secAssess.critical_findings?.length > 0 && (
                <div>
                  <p className="text-xs text-red-400 font-semibold mb-1">Critical Findings</p>
                  {secAssess.critical_findings.map((f, i) => <p key={i} className="text-xs text-gray-400 mb-1">- {f}</p>)}
                </div>
              )}
              {secAssess.positive_findings?.length > 0 && (
                <div>
                  <p className="text-xs text-green-400 font-semibold mb-1">Positive Findings</p>
                  {secAssess.positive_findings.map((f, i) => <p key={i} className="text-xs text-gray-400 mb-1">- {f}</p>)}
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
              <div key={i} className="p-3 rounded border border-gray-800 bg-gray-900/30 flex items-start gap-3">
                <div className="shrink-0 mt-0.5">
                  <div className="w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold"
                    style={{ background: `rgba(${Math.round((1 - f.confidence) * 255)}, ${Math.round(f.confidence * 200)}, 80, 0.15)`, color: `rgba(${Math.round((1 - f.confidence) * 255)}, ${Math.round(f.confidence * 200)}, 80, 0.9)` }}>
                    {Math.round(f.confidence * 100)}
                  </div>
                </div>
                <p className="text-xs text-gray-300 leading-relaxed">{f.text}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {inferences.length > 0 && (
        <Section title={`Inferences (${inferences.length})`}>
          <div className="space-y-2">
            {inferences.map((inf, i) => (
              <div key={i} className="p-3 rounded border border-gray-800 bg-gray-900/30">
                <div className="flex items-center gap-2 mb-1">
                  <span className="text-xs px-2 py-0.5 bg-purple-900/30 text-purple-400 rounded font-mono">{inf.rule_of_inference}</span>
                  <span className="text-[10px] text-gray-600">{Math.round(inf.confidence * 100)}% confidence</span>
                </div>
                <p className="text-xs text-gray-300 leading-relaxed">{inf.text}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {improvements.length > 0 && (
        <Section title={`Improvements (${improvements.length})`}>
          <div className="space-y-2">
            {improvements.map((imp, i) => (
              <div key={i} className="p-3 rounded border border-gray-800 bg-gray-900/30">
                <div className="flex items-center gap-2 mb-1">
                  <SeverityBadge severity={imp.severity} />
                  <span className="text-xs px-2 py-0.5 bg-gray-800 rounded">{imp.category}</span>
                  <span className="text-sm font-medium">{imp.title}</span>
                </div>
                <p className="text-xs text-gray-400">{imp.description}</p>
                <p className="text-xs text-blue-400 mt-1">{imp.remediation}</p>
              </div>
            ))}
          </div>
        </Section>
      )}

      {tests.length > 0 && (
        <Section title={`Suggested Tests (${tests.length})`}>
          <div className="space-y-3">
            {tests.map((t, i) => (
              <div key={i} className="p-4 rounded border border-gray-800 bg-gray-900/30">
                <div className="flex items-center gap-2 mb-2">
                  <ImportanceBadge importance={t.importance} />
                  <h4 className="text-sm font-medium">{t.name}</h4>
                  <span className="text-xs text-gray-600">({t.flow})</span>
                </div>
                <p className="text-xs text-gray-300 mb-3">{t.description}</p>
                <div className="bg-gray-950 rounded p-3 font-mono text-xs">
                  <span className="text-blue-400">{t.request.method}</span> <span className="text-gray-300">{t.request.path}</span>
                  {t.request.body != null && <pre className="text-gray-500 mt-1 whitespace-pre-wrap">{String(JSON.stringify(t.request.body, null, 2))}</pre>}
                </div>
                <div className="mt-2 text-xs">
                  <span className="text-green-400">Expected: {t.expected.status}</span>
                  <span className="text-gray-500 ml-2">{t.expected.description}</span>
                </div>
              </div>
            ))}
          </div>
        </Section>
      )}

      {questions.length > 0 && (
        <Section title="Open Questions">
          {questions.map((q, i) => (
            <div key={i} className="p-3 rounded border border-gray-800 bg-gray-900/30 mb-2">
              <p className="text-sm text-gray-300">{q.question}</p>
              {q.why_missing && <p className="text-xs text-gray-500 mt-1">{q.why_missing}</p>}
            </div>
          ))}
        </Section>
      )}
    </div>
  )
}

function AssessmentCard({ title, text }: { title: string; text: string }) {
  return (
    <div className="p-3 rounded border border-gray-800 bg-gray-900/30">
      <p className="text-xs font-semibold text-gray-400 mb-1">{title}</p>
      <p className="text-xs text-gray-300">{text}</p>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3 flex items-center gap-2">
        <span className="h-px flex-1 bg-gray-800" />
        <span>{title}</span>
        <span className="h-px flex-1 bg-gray-800" />
      </h3>
      {children}
    </div>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  const colors: Record<string, string> = {
    critical: 'bg-red-600 text-white', high: 'bg-red-900/60 text-red-300',
    medium: 'bg-yellow-900/60 text-yellow-300', low: 'bg-gray-700 text-gray-300', info: 'bg-gray-800 text-gray-400',
  }
  return <span className={`px-2 py-0.5 rounded text-xs font-medium ${colors[severity] ?? colors.info}`}>{severity}</span>
}

function ImportanceBadge({ importance }: { importance: string }) {
  const colors: Record<string, string> = {
    critical: 'bg-red-600 text-white', high: 'bg-orange-900/60 text-orange-300',
    medium: 'bg-blue-900/60 text-blue-300', low: 'bg-gray-700 text-gray-300',
  }
  return <span className={`px-2 py-0.5 rounded text-xs font-medium ${colors[importance] ?? colors.medium}`}>{importance}</span>
}
