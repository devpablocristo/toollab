import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { api } from "../../lib/api";
import { VerdictBadge } from "../../components/VerdictBadge";

type Tab = "datos" | "analisis";

export function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const [tab, setTab] = useState<Tab>("datos");

  const { data: run, isLoading } = useQuery({
    queryKey: ["run", id],
    queryFn: () => api.getRun(id!),
    enabled: !!id,
    refetchInterval: 3000,
  });

  const { data: summary } = useQuery({
    queryKey: ["run-summary", id],
    queryFn: () => api.getRunSummary(id!),
    enabled: !!id,
    retry: false,
  });

  const { data: model } = useQuery({
    queryKey: ["run-model", id],
    queryFn: () => api.getRunModel(id!),
    enabled: !!id,
    retry: false,
  });

  const { data: audit } = useQuery({
    queryKey: ["run-audit", id],
    queryFn: () => api.getRunAudit(id!),
    enabled: !!id,
    retry: false,
  });

  const { data: scenarios } = useQuery({
    queryKey: ["run-scenarios", id],
    queryFn: () => api.getRunScenarios(id!),
    enabled: !!id,
    retry: false,
  });

  const { data: logs } = useQuery({
    queryKey: ["run-logs", id],
    queryFn: () => api.getRunLogs(id!),
    enabled: !!id,
    retry: false,
  });

  const { data: llm } = useQuery({
    queryKey: ["run-llm", id],
    queryFn: () => api.getRunLLM(id!),
    enabled: !!id,
    retry: false,
    refetchInterval: 3000,
  });

  if (isLoading) return <p className="text-text-muted font-mono animate-pulse">Cargando…</p>;
  if (!run) return <p className="text-fail font-mono">Run no encontrado</p>;

  const endpoints = model?.endpoints ?? [];
  const findings = audit?.findings ?? [];
  const scenarioItems = scenarios?.items ?? [];
  const logItems = logs?.items ?? [];
  const domainGroups = model?.domain_groups ?? [];
  const endpointByID = new Map(endpoints.map((ep) => [ep.id, ep]));
  const topFlows = (model?.flows ?? [])
    .map((flow) => ({ flow, ep: endpointByID.get(flow.endpoint_id) }))
    .filter((x) => !!x.ep)
    .slice(0, 12);

  return (
    <div className="space-y-8">
      <div className="flex items-start justify-between animate-fade-in">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-2xl font-bold tracking-tight">{run.source_ref}</h1>
            <VerdictBadge verdict={run.status} />
          </div>
          <p className="font-mono text-text-secondary text-sm">{run.id}</p>
        </div>
        <p className="text-text-muted text-sm">{new Date(run.created_at).toLocaleString()}</p>
      </div>

      <div className="flex gap-2">
        <button
          onClick={() => setTab("datos")}
          className={`px-5 py-2.5 text-sm font-semibold rounded-lg transition-colors ${
            tab === "datos"
              ? "bg-accent/10 text-accent border border-accent/20"
              : "text-text-secondary hover:text-text-primary hover:bg-surface-overlay"
          }`}
        >
          Datos puros
        </button>
        <button
          onClick={() => setTab("analisis")}
          className={`px-5 py-2.5 text-sm font-semibold rounded-lg transition-colors ${
            tab === "analisis"
              ? "bg-accent/10 text-accent border border-accent/20"
              : "text-text-secondary hover:text-text-primary hover:bg-surface-overlay"
          }`}
        >
          Análisis de los datos
        </button>
      </div>

      {tab === "datos" && (
        <div className="space-y-6 animate-fade-in">
          <section className="lab-card p-5">
            <h2 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-3">Resumen</h2>
            {summary ? (
              <div className="grid grid-cols-2 md:grid-cols-5 gap-3 text-sm">
                <Stat label="Servicio" value={summary.service_name} />
                <Stat label="Endpoints" value={summary.endpoint_count} />
                <Stat label="Tipos" value={summary.type_count} />
                <Stat label="Dependencias" value={summary.dependency_count} />
                <Stat label="Estado run" value={run.status} />
              </div>
            ) : (
              <p className="text-text-muted text-sm">Aún sin summary.</p>
            )}
          </section>

          <section className="lab-card p-5">
            <h2 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-3">Endpoints detectados</h2>
            <div className="space-y-2 max-h-72 overflow-auto">
              {endpoints.map((ep) => (
                <div key={ep.id} className="bg-surface-raised border border-border-subtle rounded-lg px-3 py-2">
                  <p className="font-mono text-sm text-accent">{ep.method} {ep.path}</p>
                  <p className="text-xs text-text-secondary">{ep.handler_pkg}.{ep.handler_name}</p>
                </div>
              ))}
              {endpoints.length === 0 && <p className="text-text-muted text-sm">Sin endpoints.</p>}
            </div>
          </section>

          <section className="lab-card p-5">
            <h2 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-3">Grupos de dominio</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
              {domainGroups.map((g) => (
                <div key={g.name} className="bg-surface-raised border border-border-subtle rounded-lg px-3 py-2">
                  <p className="text-sm text-text-primary">{g.name}</p>
                  <p className="text-xs text-text-secondary">{g.endpoint_ids.length} endpoints</p>
                </div>
              ))}
              {domainGroups.length === 0 && <p className="text-text-muted text-sm">Sin grupos detectados.</p>}
            </div>
          </section>

          <section className="lab-card p-5">
            <h2 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-3">Top flujos detectados</h2>
            <div className="space-y-2 max-h-72 overflow-auto">
              {topFlows.map(({ flow, ep }) => (
                <div key={flow.id} className="bg-surface-raised border border-border-subtle rounded-lg px-3 py-2">
                  <p className="font-mono text-sm text-accent">{ep?.method} {ep?.path}</p>
                  <p className="text-xs text-text-secondary">
                    {flow.steps.map((s) => `${s.from}→${s.to}`).join(" · ")}
                  </p>
                </div>
              ))}
              {topFlows.length === 0 && <p className="text-text-muted text-sm">Sin flujos detectados.</p>}
            </div>
          </section>

          <section className="lab-card p-5">
            <h2 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-3">Hallazgos de auditoría</h2>
            <div className="space-y-2 max-h-72 overflow-auto">
              {findings.map((f) => (
                <div key={f.id} className="bg-surface-raised border border-border-subtle rounded-lg px-3 py-2">
                  <p className="text-sm text-text-primary">{f.title}</p>
                  <p className="text-xs text-text-secondary">{f.severity} · {f.category}</p>
                </div>
              ))}
              {findings.length === 0 && <p className="text-text-muted text-sm">Sin hallazgos.</p>}
            </div>
          </section>

          <section className="lab-card p-5">
            <h2 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-3">Escenarios sugeridos</h2>
            <div className="space-y-2 max-h-72 overflow-auto">
              {scenarioItems.map((s) => (
                <div key={s.id} className="bg-surface-raised border border-border-subtle rounded-lg px-3 py-2">
                  <p className="font-mono text-sm text-accent">{s.method} {s.path}</p>
                  <p className="text-xs text-text-secondary">riesgo: {s.risk_category} · esperado: {s.expected_status}</p>
                </div>
              ))}
              {scenarioItems.length === 0 && <p className="text-text-muted text-sm">Sin escenarios.</p>}
            </div>
          </section>

          <section className="lab-card p-5">
            <h2 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-3">Logs del pipeline</h2>
            <pre className="text-xs font-mono text-text-secondary max-h-64 overflow-auto whitespace-pre-wrap">
              {logItems.join("\n") || "Sin logs."}
            </pre>
          </section>
        </div>
      )}

      {tab === "analisis" && (
        <div className="lab-card p-5 animate-fade-in">
          {llm ? (
            <div className="space-y-3">
              <p className="text-xs text-text-muted font-mono">provider: {llm.provider} · model: {llm.model || "n/a"}</p>
              <p className="text-sm text-text-primary">{llm.functional_summary}</p>
              {llm.domain_groups?.length > 0 && (
                <div>
                  <p className="text-xs font-mono text-text-muted uppercase tracking-widest mb-1">Dominios</p>
                  <p className="text-sm text-text-secondary">{llm.domain_groups.join(" · ")}</p>
                </div>
              )}
              {llm.risk_hypotheses?.length > 0 && (
                <div>
                  <p className="text-xs font-mono text-text-muted uppercase tracking-widest mb-1">Riesgos</p>
                  <ul className="text-sm text-text-secondary list-disc list-inside space-y-1">
                    {llm.risk_hypotheses.map((r, i) => <li key={i}>{r}</li>)}
                  </ul>
                </div>
              )}
              {llm.suggested_test_scenarios?.length > 0 && (
                <div>
                  <p className="text-xs font-mono text-text-muted uppercase tracking-widest mb-1">Sugerencias de pruebas</p>
                  <ul className="text-sm text-text-secondary list-disc list-inside space-y-1">
                    {llm.suggested_test_scenarios.map((s, i) => <li key={i}>{s}</li>)}
                  </ul>
                </div>
              )}
              {llm.raw && (
                <div className="pt-2 border-t border-border-subtle">
                  <p className="text-xs font-mono text-text-muted uppercase tracking-widest mb-2">Reporte completo</p>
                  <div className="llm-prose text-sm text-text-secondary">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{llm.raw}</ReactMarkdown>
                  </div>
                </div>
              )}
            </div>
          ) : (
            <p className="text-text-muted text-sm">Aún no hay análisis LLM para este run.</p>
          )}
        </div>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="bg-surface-raised border border-border-subtle rounded-lg px-3 py-2.5">
      <p className="text-[10px] text-text-muted uppercase tracking-widest">{label}</p>
      <p className="font-mono text-sm text-text-primary mt-0.5 truncate">{value}</p>
    </div>
  );
}
