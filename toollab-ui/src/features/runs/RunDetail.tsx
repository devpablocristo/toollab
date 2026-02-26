import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from "recharts";
import { api, type CatalogEndpoint, type EndpointSchema, type SchemaField, type Target } from "../../lib/api";
import { VerdictBadge } from "../../components/VerdictBadge";
import { StatCard } from "../../components/StatCard";

type Tab = "datos" | "interpretacion";

export function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const [tab, setTab] = useState<Tab>("datos");

  const { data, isLoading } = useQuery({
    queryKey: ["run", id],
    queryFn: () => api.getRun(id!),
    enabled: !!id,
    refetchInterval: 5000,
  });

  const { data: targets = [] } = useQuery({
    queryKey: ["targets"],
    queryFn: api.listTargets,
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

  const { data: endpointsData } = useQuery({
    queryKey: ["run-endpoints", id],
    queryFn: () => api.getRunEndpoints(id!),
    enabled: !!id,
    retry: false,
  });

  if (isLoading) return <p className="text-text-muted font-mono animate-pulse">Cargando…</p>;
  if (!data) return <p className="text-fail font-mono">Run no encontrado</p>;

  const { run, assertions, interpretation } = data;
  const targetName = targets.find((t: Target) => t.id === run.target_id)?.name || "Target desconocido";
  const histogram: Record<string, number> = JSON.parse(run.status_histogram || "{}");

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-start justify-between animate-fade-in">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-2xl font-bold tracking-tight">{targetName}</h1>
            <VerdictBadge verdict={run.verdict} />
          </div>
          <p className="font-mono text-text-secondary text-sm">{run.id}</p>
        </div>
        <div className="flex items-center gap-3">
          <p className="text-text-muted text-sm">{new Date(run.created_at).toLocaleString()}</p>
        </div>
      </div>

      {/* Métricas principales */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 animate-fade-in stagger-1">
        <StatCard label="Requests" value={run.total_requests} />
        <StatCard label="Éxito" value={`${(run.success_rate * 100).toFixed(1)}%`} accent={run.success_rate > 0.9} />
        <StatCard label="P50" value={`${run.p50_ms}ms`} />
        <StatCard label="P95" value={`${run.p95_ms}ms`} />
        <StatCard label="P99" value={`${run.p99_ms}ms`} />
      </div>

      {/* Tabs */}
      <div className="flex gap-2 animate-fade-in stagger-2">
        <button
          onClick={() => setTab("datos")}
          className={`px-5 py-2.5 text-sm font-semibold rounded-lg transition-colors ${
            tab === "datos"
              ? "bg-accent/10 text-accent border border-accent/20"
              : "text-text-secondary hover:text-text-primary hover:bg-surface-overlay"
          }`}
        >
          Datos de la auditoría
        </button>
        <button
          onClick={() => setTab("interpretacion")}
          className={`px-5 py-2.5 text-sm font-semibold rounded-lg transition-colors flex items-center gap-2 ${
            tab === "interpretacion"
              ? "bg-accent/10 text-accent border border-accent/20"
              : "text-text-secondary hover:text-text-primary hover:bg-surface-overlay"
          }`}
        >
          Análisis de los datos
          {interpretation && <span className="w-2 h-2 rounded-full bg-accent" />}
        </button>
      </div>

      {tab === "datos" && (
        <DataTab
          run={run}
          histogram={histogram}
          assertions={assertions}
          auditData={auditData}
          coverageData={coverageData}
          contractData={contractData}
          endpoints={endpointsData?.endpoints}
        />
      )}

      {tab === "interpretacion" && (
        <InterpretationTab
          interpretation={interpretation}
        />
      )}
    </div>
  );
}

/* ═══════════════════════════════════════════════════════
   DATOS PUROS
   ═══════════════════════════════════════════════════════ */

function DataTab({ run, histogram, assertions, auditData, coverageData, contractData, endpoints }: {
  run: any; histogram: Record<string, number>; assertions: any[];
  auditData: any; coverageData: any; contractData: any;
  endpoints?: CatalogEndpoint[];
}) {
  return (
    <div className="space-y-10 animate-fade-in">

      {/* Catálogo de endpoints (Swagger) */}
      {endpoints && endpoints.length > 0 && (
        <Section title={`Endpoints documentados (${endpoints.length})`}>
          <div className="space-y-2">
            {endpoints.map((ep, i) => (
              <EndpointCard key={`${ep.method}-${ep.path}-${i}`} endpoint={ep} />
            ))}
          </div>
        </Section>
      )}

      {/* Distribución HTTP */}
      {Object.keys(histogram).length > 0 && (
        <Section title="Distribución de respuestas HTTP">
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
                      <div className={`h-full rounded-full ${is2xx ? "bg-pass" : is5xx ? "bg-fail" : is4xx ? "bg-warning" : "bg-text-muted"}`} style={{ width: `${pct}%` }} />
                    </div>
                    <span className="font-mono text-sm text-text-secondary w-10 text-right">{count}</span>
                    <span className="font-mono text-xs text-text-muted w-12 text-right">{pct}%</span>
                  </div>
                );
              })}
            </div>
          </div>
        </Section>
      )}

      {/* Cobertura */}
      {coverageData && (
        <Section title="Cobertura de endpoints">
          <div className="grid grid-cols-3 gap-4 mb-4">
            <MiniStat label="Testeados" value={coverageData.tested_endpoints} />
            <MiniStat label="Total" value={coverageData.total_endpoints} />
            <MiniStat label="Cobertura" value={`${(coverageData.coverage_rate * 100).toFixed(1)}%`} />
          </div>
          <div className="lab-card overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-text-muted text-[10px] font-mono uppercase tracking-[0.15em]">
                  <th className="text-left px-4 py-3">Método</th>
                  <th className="text-left px-4 py-3">Path</th>
                  <th className="text-right px-4 py-3">Hits</th>
                  <th className="text-right px-4 py-3">OK</th>
                  <th className="text-right px-4 py-3">Errores</th>
                  <th className="text-center px-4 py-3">Probado</th>
                </tr>
              </thead>
              <tbody>
                {coverageData.endpoints.map((ep: any, i: number) => (
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
        </Section>
      )}

      {/* Seguridad */}
      {auditData && (
        <Section title="Seguridad">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
            <MiniStat label="Nota" value={auditData.grade} />
            <MiniStat label="Puntaje" value={`${auditData.score}/100`} />
            <MiniStat label="Hallazgos" value={auditData.summary.total} />
            <div className="bg-surface-raised border border-border-subtle rounded-xl p-4">
              <div className="flex flex-wrap gap-2 text-xs font-mono">
                {auditData.summary.critical > 0 && <span className="text-fail">Crítico: {auditData.summary.critical}</span>}
                {auditData.summary.high > 0 && <span className="text-warning">Alto: {auditData.summary.high}</span>}
                {auditData.summary.medium > 0 && <span className="text-text-secondary">Medio: {auditData.summary.medium}</span>}
                {auditData.summary.low > 0 && <span className="text-text-muted">Bajo: {auditData.summary.low}</span>}
              </div>
              <p className="text-[10px] text-text-muted mt-1 uppercase tracking-widest">Por severidad</p>
            </div>
          </div>
          {auditData.findings.length > 0 && (
            <div className="lab-card overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-text-muted text-[10px] font-mono uppercase tracking-[0.15em]">
                    <th className="text-left px-4 py-3">Severidad</th>
                    <th className="text-left px-4 py-3">Hallazgo</th>
                    <th className="text-left px-4 py-3">Endpoint</th>
                    <th className="text-left px-4 py-3">Remediación</th>
                  </tr>
                </thead>
                <tbody>
                  {auditData.findings.map((f: any, i: number) => (
                    <tr key={i} className="border-b border-border-subtle/50 hover:bg-surface-overlay/30 transition-colors align-top">
                      <td className="px-4 py-2.5">
                        <span className={`text-xs font-mono px-2 py-0.5 rounded ${
                          f.severity === "critical" ? "bg-fail/10 text-fail"
                          : f.severity === "high" ? "bg-warning/10 text-warning"
                          : "bg-surface-overlay text-text-muted"
                        }`}>{f.severity}</span>
                      </td>
                      <td className="px-4 py-2.5">
                        <p className="text-sm text-text-primary font-medium">{f.title}</p>
                        <p className="text-xs text-text-muted mt-0.5">{f.description}</p>
                      </td>
                      <td className="px-4 py-2.5 font-mono text-xs text-text-secondary">{f.endpoint || "—"}</td>
                      <td className="px-4 py-2.5 text-xs text-accent">{f.remediation}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {auditData.findings.length === 0 && <p className="text-pass text-sm font-mono">Sin hallazgos de seguridad</p>}
        </Section>
      )}

      {/* Contratos */}
      {contractData && (
        <Section title="Validación de contratos">
          <div className="grid grid-cols-3 gap-4 mb-4">
            <MiniStat label="Estado" value={contractData.compliant ? "Conforme" : "No conforme"} />
            <MiniStat label="Cumplimiento" value={`${(contractData.compliance_rate * 100).toFixed(1)}%`} />
            <MiniStat label="Violaciones" value={contractData.total_violations} />
          </div>
          {contractData.violations.length > 0 && (
            <div className="lab-card overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-text-muted text-[10px] font-mono uppercase tracking-[0.15em]">
                    <th className="text-left px-4 py-3">Endpoint</th>
                    <th className="text-left px-4 py-3">Campo</th>
                    <th className="text-left px-4 py-3">Esperado</th>
                    <th className="text-left px-4 py-3">Actual</th>
                    <th className="text-left px-4 py-3">Descripción</th>
                  </tr>
                </thead>
                <tbody>
                  {contractData.violations.map((v: any, i: number) => (
                    <tr key={i} className="border-b border-border-subtle/50 hover:bg-surface-overlay/30 transition-colors align-top">
                      <td className="px-4 py-2.5 font-mono text-xs text-text-secondary">{v.endpoint}</td>
                      <td className="px-4 py-2.5 font-mono text-xs text-fail">{v.field}</td>
                      <td className="px-4 py-2.5 font-mono text-xs text-pass">{v.expected}</td>
                      <td className="px-4 py-2.5 font-mono text-xs text-fail">{v.actual}</td>
                      <td className="px-4 py-2.5 text-xs text-text-muted">{v.description}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {contractData.violations.length === 0 && <p className="text-pass text-sm font-mono">Sin violaciones de contrato</p>}
        </Section>
      )}

      {/* Assertions */}
      {assertions.length > 0 && (
        <Section title="Reglas y assertions">
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
        </Section>
      )}

      {/* Metadata */}
      <Section title="Metadata del run">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <MetaItem label="Duración" value={`${run.duration_s}s`} />
          <MetaItem label="Concurrencia" value={run.concurrency} />
          <MetaItem label="Inicio" value={new Date(run.started_at).toLocaleString()} />
          <MetaItem label="Fin" value={new Date(run.finished_at).toLocaleString()} />
        </div>
      </Section>
    </div>
  );
}

/* ═══════════════════════════════════════════════════════
   ENDPOINT CARD — Detalle expandible del swagger
   ═══════════════════════════════════════════════════════ */

const methodColors: Record<string, string> = {
  GET: "bg-blue-500/10 text-blue-400 border-blue-500/20",
  POST: "bg-green-500/10 text-green-400 border-green-500/20",
  PUT: "bg-amber-500/10 text-amber-400 border-amber-500/20",
  PATCH: "bg-orange-500/10 text-orange-400 border-orange-500/20",
  DELETE: "bg-red-500/10 text-red-400 border-red-500/20",
  HEAD: "bg-purple-500/10 text-purple-400 border-purple-500/20",
  OPTIONS: "bg-gray-500/10 text-gray-400 border-gray-500/20",
};

function EndpointCard({ endpoint: ep }: { endpoint: CatalogEndpoint }) {
  const [open, setOpen] = useState(false);
  const mc = methodColors[ep.method] || "bg-surface-overlay text-text-muted border-border-subtle";
  const hasDetails = (ep.parameters && ep.parameters.length > 0) || ep.request_body || (ep.responses && ep.responses.length > 0);

  return (
    <div className={`border border-border-subtle rounded-xl overflow-hidden transition-all ${ep.deprecated ? "opacity-60" : ""}`}>
      <button
        onClick={() => hasDetails && setOpen(!open)}
        className={`w-full flex items-center gap-3 px-4 py-3 text-left ${hasDetails ? "hover:bg-surface-overlay/30 cursor-pointer" : "cursor-default"} transition-colors`}
      >
        <span className={`px-2 py-0.5 text-[11px] font-mono font-bold rounded border ${mc}`}>
          {ep.method}
        </span>
        <span className="font-mono text-sm text-text-primary flex-1">{ep.path}</span>
        {ep.summary && <span className="text-xs text-text-muted truncate max-w-[300px]">{ep.summary}</span>}
        {ep.deprecated && <span className="text-[10px] text-warning bg-warning/10 px-1.5 py-0.5 rounded">deprecated</span>}
        {ep.tags && ep.tags.map(t => (
          <span key={t} className="text-[10px] text-text-muted bg-surface-overlay px-1.5 py-0.5 rounded">{t}</span>
        ))}
        {hasDetails && (
          <span className={`text-text-muted text-sm transition-transform duration-200 ${open ? "rotate-180" : ""}`}>&#x25BE;</span>
        )}
      </button>

      {open && (
        <div className="border-t border-border-subtle/50 bg-surface/50">
          {ep.description && (
            <div className="px-4 py-3 border-b border-border-subtle/30">
              <p className="text-sm text-text-secondary">{ep.description}</p>
            </div>
          )}

          <div className="grid grid-cols-1 md:grid-cols-2 gap-0 divide-y md:divide-y-0 md:divide-x divide-border-subtle/30">
            {/* Parámetros + Request Body */}
            <div className="p-4 space-y-4">
              {ep.parameters && ep.parameters.length > 0 && (
                <div>
                  <h4 className="text-[10px] font-mono text-text-muted uppercase tracking-widest mb-2">Parámetros</h4>
                  <div className="space-y-1.5">
                    {ep.parameters.map((p, i) => (
                      <div key={i} className="flex items-start gap-2 text-xs">
                        <span className="font-mono text-accent font-medium">{p.name}</span>
                        <span className="text-text-muted">({p.in})</span>
                        <span className="text-text-muted">{p.type}{p.format ? ` / ${p.format}` : ""}</span>
                        {p.required && <span className="text-fail text-[10px]">*</span>}
                        {p.description && <span className="text-text-secondary ml-auto">{p.description}</span>}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {ep.request_body && (
                <div>
                  <h4 className="text-[10px] font-mono text-text-muted uppercase tracking-widest mb-2">
                    Request body
                    <span className="ml-2 text-text-secondary normal-case">{ep.request_body.content_type}</span>
                    {ep.request_body.required && <span className="text-fail ml-1">*</span>}
                  </h4>
                  {ep.request_body.description && (
                    <p className="text-xs text-text-secondary mb-2">{ep.request_body.description}</p>
                  )}
                  {ep.request_body.schema && <SchemaView schema={ep.request_body.schema} />}
                </div>
              )}

              {!ep.parameters?.length && !ep.request_body && (
                <p className="text-xs text-text-muted">Sin parámetros ni body</p>
              )}
            </div>

            {/* Responses */}
            <div className="p-4">
              <h4 className="text-[10px] font-mono text-text-muted uppercase tracking-widest mb-2">Respuestas</h4>
              {ep.responses && ep.responses.length > 0 ? (
                <div className="space-y-3">
                  {ep.responses.map((r, i) => (
                    <div key={i}>
                      <div className="flex items-center gap-2 mb-1">
                        <span className={`font-mono text-xs font-bold ${
                          r.status.startsWith("2") ? "text-pass"
                          : r.status.startsWith("4") ? "text-warning"
                          : r.status.startsWith("5") ? "text-fail"
                          : "text-text-muted"
                        }`}>{r.status}</span>
                        {r.description && <span className="text-xs text-text-secondary">{r.description}</span>}
                      </div>
                      {r.schema && <SchemaView schema={r.schema} />}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-text-muted">Sin respuestas documentadas</p>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function SchemaView({ schema, depth = 0 }: { schema: EndpointSchema; depth?: number }) {
  if (depth > 3) return null;

  if (schema.type === "array" && schema.items) {
    return (
      <div className="text-xs">
        <span className="text-text-muted font-mono">array of:</span>
        <div className="ml-3 mt-1">
          <SchemaView schema={schema.items} depth={depth + 1} />
        </div>
      </div>
    );
  }

  if (!schema.fields || schema.fields.length === 0) {
    return <span className="text-xs font-mono text-text-muted">{schema.type}{schema.format ? ` (${schema.format})` : ""}</span>;
  }

  return (
    <div className="bg-surface-overlay/50 rounded-lg overflow-hidden">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-text-muted text-[9px] font-mono uppercase tracking-widest">
            <th className="text-left px-3 py-1.5">Campo</th>
            <th className="text-left px-3 py-1.5">Tipo</th>
            <th className="text-left px-3 py-1.5">Info</th>
          </tr>
        </thead>
        <tbody>
          {schema.fields.map((f: SchemaField, i: number) => (
            <tr key={i} className="border-t border-border-subtle/20">
              <td className="px-3 py-1.5 font-mono text-accent">
                {f.name}
                {f.required && <span className="text-fail ml-0.5">*</span>}
              </td>
              <td className="px-3 py-1.5 font-mono text-text-muted">
                {f.type}{f.format ? ` (${f.format})` : ""}
                {f.nullable && <span className="text-text-muted/50 ml-1">nullable</span>}
              </td>
              <td className="px-3 py-1.5 text-text-secondary">
                {f.description || (f.enum ? `enum: ${f.enum.join(", ")}` : "") || (f.example !== undefined ? `ej: ${JSON.stringify(f.example)}` : "")}
                {f.items && (
                  <div className="mt-1">
                    <SchemaView schema={f.items} depth={depth + 1} />
                  </div>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/* ═══════════════════════════════════════════════════════
   INTERPRETACIÓN LLM
   ═══════════════════════════════════════════════════════ */

function InterpretationTab({ interpretation }: {
  interpretation: string | null;
}) {
  return (
    <div className="space-y-4 animate-fade-in">
      {interpretation && (
        <div>
          <div className="bg-surface-raised border border-accent/10 rounded-xl overflow-hidden">
            <div className="h-1 bg-gradient-to-r from-accent/40 via-accent/20 to-transparent" />
            <div className="p-8 llm-prose">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{interpretation}</ReactMarkdown>
            </div>
          </div>
        </div>
      )}

      {!interpretation && (
        <div className="bg-surface-raised border border-border-subtle rounded-xl p-12 text-center space-y-4">
          <div className="inline-block w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mb-2" />
          <p className="text-text-muted text-sm">
            El análisis LLM se ejecuta automáticamente al generar el run.<br />
            Estamos esperando el resultado asíncrono.
          </p>
          <p className="text-text-muted text-xs">La vista se actualiza automáticamente cada 5 segundos.</p>
        </div>
      )}
    </div>
  );
}

/* ═══════════════════════════════════════════════════════
   Componentes auxiliares
   ═══════════════════════════════════════════════════════ */

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h2 className="text-sm font-mono text-text-muted uppercase tracking-widest mb-4">{title}</h2>
      {children}
    </div>
  );
}

function MiniStat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="bg-surface-raised border border-border-subtle rounded-xl p-4 text-center">
      <p className="text-2xl font-bold text-text-primary">{value}</p>
      <p className="text-[10px] text-text-muted mt-1 uppercase tracking-widest">{label}</p>
    </div>
  );
}

function MetaItem({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="bg-surface-raised border border-border-subtle rounded-lg px-3 py-2.5">
      <p className="text-[10px] text-text-muted uppercase tracking-widest">{label}</p>
      <p className="font-mono text-sm text-text-primary mt-0.5 truncate" title={String(value)}>{value}</p>
    </div>
  );
}
