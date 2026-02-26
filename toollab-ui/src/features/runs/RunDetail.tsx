import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from "recharts";
import { api } from "../../lib/api";
import { VerdictBadge } from "../../components/VerdictBadge";
import { StatCard } from "../../components/StatCard";

type Tab = "comprehension" | "overview" | "security" | "coverage" | "contract" | "interpretation";

function TabButton({ label, active, onClick, badge }: { label: string; active: boolean; onClick: () => void; badge?: string }) {
  return (
    <button
      onClick={onClick}
      className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
        active
          ? "bg-accent/10 text-accent border border-accent/20"
          : "text-text-secondary hover:text-text-primary hover:bg-surface-overlay"
      }`}
    >
      {label}
      {badge && (
        <span className={`ml-2 text-xs px-1.5 py-0.5 rounded-full ${
          badge === "A" || badge === "B" ? "bg-pass/10 text-pass"
          : badge === "C" ? "bg-warning/10 text-warning"
          : "bg-fail/10 text-fail"
        }`}>{badge}</span>
      )}
    </button>
  );
}

export function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const qc = useQueryClient();
  const [tab, setTab] = useState<Tab>("comprehension");

  const { data, isLoading } = useQuery({
    queryKey: ["run", id],
    queryFn: () => api.getRun(id!),
    enabled: !!id,
  });

  const interpret = useMutation({
    mutationFn: () => api.execInterpret({ run_id: id! }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["run", id] }),
  });

  const { data: auditData } = useQuery({
    queryKey: ["run-audit", id],
    queryFn: () => api.getRunAudit(id!),
    enabled: !!id,
    retry: false,
  });

  const { data: coverageData } = useQuery({
    queryKey: ["run-coverage", id],
    queryFn: () => api.getRunCoverage(id!),
    enabled: !!id,
    retry: false,
  });

  const { data: contractData } = useQuery({
    queryKey: ["run-contract", id],
    queryFn: () => api.getRunContract(id!),
    enabled: !!id,
    retry: false,
  });

  const { data: comprehensionData } = useQuery({
    queryKey: ["run-comprehension", id],
    queryFn: () => api.getRunComprehension(id!),
    enabled: !!id,
    retry: false,
  });

  if (isLoading) return <p className="text-text-muted font-mono animate-pulse">Cargando…</p>;
  if (!data) return <p className="text-fail font-mono">Run no encontrado</p>;

  const { run, assertions, interpretation } = data;
  const histogram: Record<string, number> = JSON.parse(run.status_histogram || "{}");

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-start justify-between animate-fade-in">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-2xl font-bold tracking-tight">Run</h1>
            <VerdictBadge verdict={run.verdict} />
          </div>
          <p className="font-mono text-text-secondary text-sm">{run.id}</p>
        </div>
        <div className="flex items-center gap-3">
          {!interpretation && (
            <button
              onClick={() => interpret.mutate()}
              disabled={interpret.isPending}
              className="bg-surface-raised border border-accent/30 text-accent px-4 py-2 rounded-lg font-semibold text-sm hover:bg-accent/10 transition-colors disabled:opacity-40"
            >
              {interpret.isPending ? "Interpretando…" : "Interpretar con LLM"}
            </button>
          )}
          <p className="text-text-muted text-sm">{new Date(run.created_at).toLocaleString()}</p>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 animate-fade-in stagger-1">
        <StatCard label="Requests" value={run.total_requests} />
        <StatCard label="Success" value={`${(run.success_rate * 100).toFixed(1)}%`} accent={run.success_rate > 0.9} />
        <StatCard label="P50" value={`${run.p50_ms}ms`} />
        <StatCard label="P95" value={`${run.p95_ms}ms`} />
        <StatCard label="P99" value={`${run.p99_ms}ms`} />
      </div>

      {/* Tabs */}
      <div className="flex gap-2 flex-wrap animate-fade-in stagger-2">
        <TabButton label="Comprensión" active={tab === "comprehension"} onClick={() => setTab("comprehension")} />
        <TabButton label="Overview" active={tab === "overview"} onClick={() => setTab("overview")} />
        <TabButton label="Seguridad" active={tab === "security"} onClick={() => setTab("security")} badge={auditData?.grade} />
        <TabButton label="Cobertura" active={tab === "coverage"} onClick={() => setTab("coverage")} />
        <TabButton label="Contratos" active={tab === "contract"} onClick={() => setTab("contract")} />
        <TabButton label="Interpretación LLM" active={tab === "interpretation"} onClick={() => setTab("interpretation")} />
      </div>

      {/* Tab Content */}
      {tab === "comprehension" && <ComprehensionTab markdown={comprehensionData?.markdown} />}
      {tab === "overview" && <OverviewTab histogram={histogram} assertions={assertions} run={run} />}
      {tab === "security" && <SecurityTab data={auditData} />}
      {tab === "coverage" && <CoverageTab data={coverageData} />}
      {tab === "contract" && <ContractTab data={contractData} />}
      {tab === "interpretation" && <InterpretationTab interpretation={interpretation} isPending={interpret.isPending} />}
    </div>
  );
}

function OverviewTab({ histogram, assertions, run }: { histogram: Record<string, number>; assertions: any[]; run: any }) {
  return (
    <div className="space-y-8">
      {Object.keys(histogram).length > 0 && (
        <div className="animate-fade-in">
          <h2 className="text-lg font-semibold mb-3">Distribución de Respuestas</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="bg-surface-raised border border-border-subtle rounded-xl p-5">
              <ResponsiveContainer width="100%" height={200}>
                <PieChart>
                  <Pie
                    data={Object.entries(histogram).map(([code, count]) => ({ name: `HTTP ${code}`, value: count }))}
                    cx="50%" cy="50%" innerRadius={55} outerRadius={80} paddingAngle={3} dataKey="value" strokeWidth={0}
                  >
                    {Object.keys(histogram).map((code) => {
                      const color = code.startsWith("2") ? "#00e89d" : code.startsWith("4") ? "#ffb020" : code.startsWith("5") ? "#ff3b5c" : "#6b7a94";
                      return <Cell key={code} fill={color} />;
                    })}
                  </Pie>
                  <Tooltip contentStyle={{ background: "#0c1322", border: "1px solid #1c2840", borderRadius: 8, fontSize: 12 }} itemStyle={{ color: "#e0e6f0" }} />
                </PieChart>
              </ResponsiveContainer>
            </div>
            <div className="flex flex-col gap-2 justify-center">
              {Object.entries(histogram).sort(([a], [b]) => a.localeCompare(b)).map(([code, count]) => {
                const is2xx = code.startsWith("2");
                const is4xx = code.startsWith("4");
                const is5xx = code.startsWith("5");
                const pct = ((count / run.total_requests) * 100).toFixed(1);
                return (
                  <div key={code} className="flex items-center gap-3 bg-surface-raised border border-border-subtle rounded-lg px-4 py-2.5">
                    <span className={`w-2.5 h-2.5 rounded-full ${is2xx ? "bg-pass" : is5xx ? "bg-fail" : is4xx ? "bg-warning" : "bg-text-muted"}`} />
                    <span className="font-mono text-sm text-text-primary w-16">HTTP {code}</span>
                    <div className="flex-1 h-1.5 bg-surface-overlay rounded-full overflow-hidden">
                      <div className={`h-full rounded-full ${is2xx ? "bg-pass" : is5xx ? "bg-fail" : is4xx ? "bg-warning" : "bg-text-muted"}`} style={{ width: `${pct}%`, animation: "bar-fill 0.6s ease-out both" }} />
                    </div>
                    <span className="font-mono text-sm text-text-secondary w-10 text-right">{count}</span>
                    <span className="font-mono text-xs text-text-muted w-12 text-right">{pct}%</span>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      )}

      <div className="animate-fade-in">
        <h2 className="text-lg font-semibold mb-3">Assertions</h2>
        <div className="space-y-2">
          {assertions.map((a: any, i: number) => (
            <div key={i} className={`bg-surface-raised border rounded-lg px-4 py-3 flex items-center justify-between ${a.passed ? "border-pass/10" : "border-fail/20"}`}>
              <div className="flex items-center gap-3">
                <span className={`w-2 h-2 rounded-full ${a.passed ? "bg-pass" : "bg-fail"}`} />
                <span className="font-mono text-sm">{a.rule_id}</span>
                <span className="text-text-secondary text-sm">{a.message}</span>
              </div>
              <div className="font-mono text-xs text-text-muted">{a.observed} / {a.expected}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function SecurityTab({ data }: { data: any }) {
  if (!data) {
    return (
      <div className="lab-card lab-card--neutral p-12 text-center">
        <p className="text-text-muted font-mono">Auditoría de seguridad no disponible para este run.</p>
        <p className="text-text-secondary text-sm mt-2">Los runs nuevos generan el reporte automáticamente.</p>
      </div>
    );
  }

  const gradeColor = data.grade === "A" || data.grade === "B" ? "text-pass" : data.grade === "C" ? "text-warning" : "text-fail";

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 text-center">
          <p className={`text-4xl font-bold ${gradeColor}`}>{data.grade}</p>
          <p className="text-xs text-text-muted mt-1 uppercase tracking-widest">Grade</p>
        </div>
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 text-center">
          <p className="text-4xl font-bold text-text-primary">{data.score}</p>
          <p className="text-xs text-text-muted mt-1 uppercase tracking-widest">Score /100</p>
        </div>
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 text-center">
          <p className="text-4xl font-bold text-text-primary">{data.summary.total}</p>
          <p className="text-xs text-text-muted mt-1 uppercase tracking-widest">Hallazgos</p>
        </div>
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5">
          <div className="flex justify-between text-xs font-mono">
            {data.summary.critical > 0 && <span className="text-fail">Critical: {data.summary.critical}</span>}
            {data.summary.high > 0 && <span className="text-warning">High: {data.summary.high}</span>}
            {data.summary.medium > 0 && <span className="text-text-secondary">Medium: {data.summary.medium}</span>}
            {data.summary.low > 0 && <span className="text-text-muted">Low: {data.summary.low}</span>}
          </div>
          <p className="text-xs text-text-muted mt-2 uppercase tracking-widest text-center">Por severidad</p>
        </div>
      </div>

      {data.findings.length > 0 && (
        <div className="space-y-3">
          <h3 className="text-sm font-semibold text-text-muted uppercase tracking-widest">Hallazgos</h3>
          {data.findings.map((f: any, i: number) => (
            <div key={i} className={`bg-surface-raised border rounded-xl p-5 ${
              f.severity === "critical" ? "border-fail/30" : f.severity === "high" ? "border-warning/30" : "border-border-subtle"
            }`}>
              <div className="flex items-start justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span className={`text-xs font-mono px-2 py-0.5 rounded ${
                    f.severity === "critical" ? "bg-fail/10 text-fail"
                    : f.severity === "high" ? "bg-warning/10 text-warning"
                    : "bg-surface-overlay text-text-muted"
                  }`}>{f.severity}</span>
                  <span className="font-semibold text-sm text-text-primary">{f.title}</span>
                </div>
                <span className="text-xs font-mono text-text-muted">{f.id}</span>
              </div>
              <p className="text-sm text-text-secondary mb-3">{f.description}</p>
              {f.endpoint && <p className="text-xs font-mono text-text-muted mb-2">Endpoint: {f.endpoint}</p>}
              <div className="bg-surface-overlay rounded-lg px-3 py-2">
                <p className="text-xs text-accent">{f.remediation}</p>
              </div>
            </div>
          ))}
        </div>
      )}

      {data.findings.length === 0 && (
        <div className="lab-card p-12 text-center">
          <p className="text-pass text-2xl font-bold mb-2">Sin hallazgos</p>
          <p className="text-text-secondary text-sm">No se detectaron problemas de seguridad.</p>
        </div>
      )}
    </div>
  );
}

function CoverageTab({ data }: { data: any }) {
  if (!data) {
    return (
      <div className="lab-card lab-card--neutral p-12 text-center">
        <p className="text-text-muted font-mono">Reporte de cobertura no disponible.</p>
      </div>
    );
  }

  const pct = (data.coverage_rate * 100).toFixed(1);

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 text-center">
          <p className={`text-4xl font-bold ${+pct >= 90 ? "text-pass" : +pct >= 70 ? "text-warning" : "text-fail"}`}>{pct}%</p>
          <p className="text-xs text-text-muted mt-1 uppercase tracking-widest">Cobertura</p>
        </div>
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 text-center">
          <p className="text-4xl font-bold text-text-primary">{data.tested_endpoints}</p>
          <p className="text-xs text-text-muted mt-1 uppercase tracking-widest">Testeados</p>
        </div>
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 text-center">
          <p className="text-4xl font-bold text-text-primary">{data.total_endpoints}</p>
          <p className="text-xs text-text-muted mt-1 uppercase tracking-widest">Total</p>
        </div>
      </div>

      <div>
        <h3 className="text-sm font-semibold text-text-muted uppercase tracking-widest mb-3">Endpoints</h3>
        <div className="lab-card overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-text-muted text-[10px] font-mono uppercase tracking-[0.15em]">
                <th className="text-left px-4 py-3">Método</th>
                <th className="text-left px-4 py-3">Path</th>
                <th className="text-right px-4 py-3">Hits</th>
                <th className="text-right px-4 py-3">OK</th>
                <th className="text-right px-4 py-3">Errores</th>
                <th className="text-center px-4 py-3">Estado</th>
              </tr>
            </thead>
            <tbody>
              {data.endpoints.map((ep: any, i: number) => (
                <tr key={i} className="border-b border-border-subtle/50 hover:bg-surface-overlay/30 transition-colors">
                  <td className="px-4 py-2.5 font-mono text-accent text-xs">{ep.method}</td>
                  <td className="px-4 py-2.5 font-mono text-text-secondary text-xs">{ep.path}</td>
                  <td className="px-4 py-2.5 text-right font-mono text-text-secondary">{ep.hits}</td>
                  <td className="px-4 py-2.5 text-right font-mono text-pass">{ep.success}</td>
                  <td className="px-4 py-2.5 text-right font-mono text-fail">{ep.errors}</td>
                  <td className="px-4 py-2.5 text-center">
                    <span className={`w-2 h-2 rounded-full inline-block ${ep.tested ? "bg-pass" : "bg-fail"}`} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {data.by_method.length > 0 && (
        <div>
          <h3 className="text-sm font-semibold text-text-muted uppercase tracking-widest mb-3">Por método HTTP</h3>
          <div className="flex gap-3 flex-wrap">
            {data.by_method.map((m: any) => (
              <div key={m.method} className="bg-surface-raised border border-border-subtle rounded-lg px-4 py-3 text-center min-w-[100px]">
                <p className="font-mono text-accent text-sm font-bold">{m.method}</p>
                <p className="text-xs text-text-muted mt-1">{m.tested}/{m.total} ({(m.rate * 100).toFixed(0)}%)</p>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function ContractTab({ data }: { data: any }) {
  if (!data) {
    return (
      <div className="lab-card lab-card--neutral p-12 text-center">
        <p className="text-text-muted font-mono">Validación de contratos no disponible.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 text-center">
          <p className={`text-3xl font-bold ${data.compliant ? "text-pass" : "text-fail"}`}>
            {data.compliant ? "CONFORME" : "NO CONFORME"}
          </p>
          <p className="text-xs text-text-muted mt-1 uppercase tracking-widest">Estado</p>
        </div>
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 text-center">
          <p className="text-4xl font-bold text-text-primary">{(data.compliance_rate * 100).toFixed(1)}%</p>
          <p className="text-xs text-text-muted mt-1 uppercase tracking-widest">Compliance</p>
        </div>
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 text-center">
          <p className={`text-4xl font-bold ${data.total_violations > 0 ? "text-fail" : "text-pass"}`}>{data.total_violations}</p>
          <p className="text-xs text-text-muted mt-1 uppercase tracking-widest">Violaciones</p>
        </div>
      </div>

      {data.violations.length > 0 && (
        <div className="space-y-3">
          <h3 className="text-sm font-semibold text-text-muted uppercase tracking-widest">Violaciones de Contrato</h3>
          {data.violations.map((v: any, i: number) => (
            <div key={i} className="bg-surface-raised border border-fail/15 rounded-xl p-5">
              <div className="flex items-center gap-2 mb-2">
                <span className="text-xs font-mono bg-fail/10 text-fail px-2 py-0.5 rounded">{v.field}</span>
                <span className="font-mono text-xs text-text-muted">{v.endpoint}</span>
              </div>
              <p className="text-sm text-text-secondary mb-3">{v.description}</p>
              <div className="grid grid-cols-2 gap-3 text-xs font-mono">
                <div className="bg-surface-overlay rounded-lg px-3 py-2">
                  <span className="text-text-muted">Esperado:</span> <span className="text-pass">{v.expected}</span>
                </div>
                <div className="bg-surface-overlay rounded-lg px-3 py-2">
                  <span className="text-text-muted">Actual:</span> <span className="text-fail">{v.actual}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {data.violations.length === 0 && (
        <div className="lab-card p-12 text-center">
          <p className="text-pass text-2xl font-bold mb-2">Sin violaciones</p>
          <p className="text-text-secondary text-sm">Todas las respuestas cumplen con el contrato.</p>
        </div>
      )}
    </div>
  );
}

function ComprehensionTab({ markdown }: { markdown?: string }) {
  if (!markdown) {
    return (
      <div className="lab-card lab-card--neutral p-12 text-center">
        <p className="text-text-muted font-mono">Reporte de comprensión no disponible.</p>
        <p className="text-text-secondary text-sm mt-2">Los runs nuevos lo generan automáticamente.</p>
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <div className="flex items-center gap-3 mb-4">
        <div className="w-8 h-8 rounded-lg bg-accent/10 border border-accent/20 flex items-center justify-center">
          <span className="text-accent text-base font-bold">?</span>
        </div>
        <div>
          <h2 className="text-lg font-semibold">Comprensión del Servicio</h2>
          <p className="text-xs text-text-muted">Todo lo que necesitás saber sobre este servicio, generado desde la evidencia</p>
        </div>
      </div>
      <div className="bg-surface-raised border border-accent/10 rounded-xl overflow-hidden">
        <div className="h-1 bg-gradient-to-r from-accent/40 via-accent/20 to-transparent" />
        <div className="p-8 llm-prose">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{markdown}</ReactMarkdown>
        </div>
      </div>
    </div>
  );
}

function InterpretationTab({ interpretation, isPending }: { interpretation: string | null; isPending: boolean }) {
  return (
    <div className="space-y-4">
      {interpretation && (
        <div className="animate-fade-in">
          <div className="flex items-center gap-3 mb-4">
            <div className="w-8 h-8 rounded-lg bg-accent/10 border border-accent/20 flex items-center justify-center">
              <span className="text-accent text-sm">AI</span>
            </div>
            <div>
              <h2 className="text-lg font-semibold">Interpretación LLM</h2>
              <p className="text-xs text-text-muted">Generada por <span className="font-mono text-accent/70">ollama</span> — análisis automático de la evidencia</p>
            </div>
          </div>
          <div className="bg-surface-raised border border-accent/10 rounded-xl overflow-hidden">
            <div className="h-1 bg-gradient-to-r from-accent/40 via-accent/20 to-transparent" />
            <div className="p-8 llm-prose">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{interpretation}</ReactMarkdown>
            </div>
          </div>
        </div>
      )}

      {isPending && (
        <div className="animate-fade-in bg-surface-raised border border-accent/20 rounded-xl p-6 text-center">
          <div className="inline-block w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mb-3" />
          <p className="text-text-secondary text-sm">Ollama está interpretando los resultados…</p>
          <p className="text-text-muted text-xs mt-1">Esto puede tardar 1-2 min en CPU</p>
        </div>
      )}

      {!interpretation && !isPending && (
        <div className="lab-card lab-card--neutral p-12 text-center">
          <p className="text-text-muted font-mono">Sin interpretación LLM.</p>
          <p className="text-text-secondary text-sm mt-2">Usa el botón "Interpretar con LLM" en la cabecera.</p>
        </div>
      )}
    </div>
  );
}
