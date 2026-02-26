import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type ExecResult, type ScenarioFile, type Target } from "../../lib/api";
import { Terminal } from "../../components/Terminal";

type Tab = "generate" | "run" | "enrich";

export function Execute() {
  const [tab, setTab] = useState<Tab>("generate");

  const tabClass = (t: Tab) =>
    `px-4 py-2 text-sm font-medium rounded-t-lg transition-colors ${
      tab === t
        ? "bg-surface-raised text-accent border border-border-subtle border-b-surface-raised -mb-px"
        : "text-text-muted hover:text-text-secondary"
    }`;

  return (
    <div className="space-y-6">
      <div className="animate-fade-in">
        <h1 className="text-2xl font-bold tracking-tight">Ejecutar</h1>
        <p className="text-text-secondary mt-1">Comandos de Toollab desde el browser</p>
      </div>

      <div className="flex gap-1 border-b border-border-subtle">
        <button onClick={() => setTab("generate")} className={tabClass("generate")}>
          Generate
        </button>
        <button onClick={() => setTab("run")} className={tabClass("run")}>
          Run
        </button>
        <button onClick={() => setTab("enrich")} className={tabClass("enrich")}>
          Enrich
        </button>
      </div>

      {tab === "generate" && <GeneratePanel />}
      {tab === "run" && <RunPanel />}
      {tab === "enrich" && <EnrichPanel />}
    </div>
  );
}

function GeneratePanel() {
  const [from, setFrom] = useState("toollab");
  const [baseUrl, setBaseUrl] = useState("");
  const [mode, setMode] = useState("smoke");
  const [openapiUrl, setOpenapiUrl] = useState("");
  const [result, setResult] = useState<ExecResult | null>(null);

  const { data: targets } = useQuery({ queryKey: ["targets"], queryFn: api.listTargets });

  const mutation = useMutation({
    mutationFn: () =>
      api.execGenerate({
        from,
        target_base_url: baseUrl,
        mode,
        openapi_url: openapiUrl || undefined,
      }),
    onSuccess: (data) => setResult(data),
  });

  return (
    <div className="space-y-4 animate-fade-in">
      <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 space-y-4">
        <div className="grid grid-cols-3 gap-4">
          <div>
            <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">Source</label>
            <select
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
            >
              <option value="toollab">Toollab Adapter</option>
              <option value="openapi">OpenAPI</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">Target URL</label>
            <div>
              <input
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                placeholder="http://localhost:8080"
                className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
              />
              {targets && targets.length > 0 && (
                <div className="flex gap-1 mt-1.5 flex-wrap">
                  {targets.map((t: Target) => (
                    <button
                      key={t.id}
                      onClick={() => setBaseUrl(t.base_url)}
                      className="text-xs bg-surface-overlay px-2 py-0.5 rounded text-accent hover:bg-accent/10 transition-colors"
                    >
                      {t.name}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
          <div>
            <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">Mode</label>
            <select
              value={mode}
              onChange={(e) => setMode(e.target.value)}
              className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
            >
              <option value="smoke">Smoke</option>
              <option value="load">Load</option>
              <option value="chaos">Chaos</option>
            </select>
          </div>
        </div>

        {from === "openapi" && (
          <input
            value={openapiUrl}
            onChange={(e) => setOpenapiUrl(e.target.value)}
            placeholder="OpenAPI URL (ej: http://localhost:8080/openapi.yaml)"
            className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
          />
        )}

        <button
          onClick={() => mutation.mutate()}
          disabled={mutation.isPending || !baseUrl}
          className="bg-accent text-surface px-5 py-2.5 rounded-lg font-semibold text-sm hover:bg-accent-dim transition-colors disabled:opacity-40"
        >
          {mutation.isPending ? "Generando…" : "Generar escenario"}
        </button>
      </div>

      {result && <Terminal output={result.output} error={result.error} />}
    </div>
  );
}

function RunPanel() {
  const qc = useQueryClient();
  const { data: scenarios } = useQuery({ queryKey: ["scenarios"], queryFn: api.listScenarios });
  const { data: targets } = useQuery({ queryKey: ["targets"], queryFn: api.listTargets });

  const [scenarioPath, setScenarioPath] = useState("");
  const [targetId, setTargetId] = useState("");
  const [result, setResult] = useState<{ exec: ExecResult; run_id?: string } | null>(null);

  const mutation = useMutation({
    mutationFn: () => api.execRun({ scenario_path: scenarioPath, target_id: targetId || undefined }),
    onSuccess: (data) => {
      setResult(data);
      qc.invalidateQueries({ queryKey: ["runs"] });
      qc.invalidateQueries({ queryKey: ["stats"] });
    },
    onError: (err) => {
      setResult({ exec: { success: false, output: "", error: String(err), elapsed: "" } });
    },
  });

  return (
    <div className="space-y-4 animate-fade-in">
      <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 space-y-4">
        <div>
          <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">Escenario</label>
          {scenarios && scenarios.length > 0 ? (
            <div className="space-y-2">
              <div className="flex gap-2 flex-wrap">
                {scenarios.map((s: ScenarioFile) => (
                  <button
                    key={s.path}
                    onClick={() => setScenarioPath(s.path)}
                    className={`text-sm px-3 py-1.5 rounded-lg border transition-colors ${
                      scenarioPath === s.path
                        ? "border-accent bg-accent/10 text-accent"
                        : "border-border-subtle bg-surface hover:border-border text-text-secondary"
                    }`}
                  >
                    {s.name}
                  </button>
                ))}
              </div>
              <input
                value={scenarioPath}
                onChange={(e) => setScenarioPath(e.target.value)}
                placeholder="O escribí la ruta manualmente"
                className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none font-mono"
              />
            </div>
          ) : (
            <input
              value={scenarioPath}
              onChange={(e) => setScenarioPath(e.target.value)}
              placeholder="Ruta al escenario (ej: scenarios/nexus.yaml)"
              className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
            />
          )}
        </div>

        <div>
          <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">Target (para auto-ingest)</label>
          <select
            value={targetId}
            onChange={(e) => setTargetId(e.target.value)}
            className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
          >
            <option value="">Sin auto-ingest</option>
            {targets?.map((t: Target) => (
              <option key={t.id} value={t.id}>{t.name} ({t.base_url})</option>
            ))}
          </select>
        </div>

        <button
          onClick={() => mutation.mutate()}
          disabled={mutation.isPending || !scenarioPath}
          className="bg-accent text-surface px-5 py-2.5 rounded-lg font-semibold text-sm hover:bg-accent-dim transition-colors disabled:opacity-40"
        >
          {mutation.isPending ? "Ejecutando…" : "Ejecutar run"}
        </button>
      </div>

      {result && (
        <div className="space-y-3">
          {result.run_id && (
            <div className="bg-pass/10 border border-pass/20 rounded-xl p-4 text-sm">
              <span className="text-pass font-semibold">
                {(result as any).note ? "Run ya existente: " : "Run ingested: "}
              </span>
              <a href={`/runs/${result.run_id}`} className="text-accent font-mono hover:underline">
                {result.run_id}
              </a>
            </div>
          )}
          {result.exec && (
            <Terminal output={result.exec.output} error={result.exec.error} />
          )}
        </div>
      )}
    </div>
  );
}

function EnrichPanel() {
  const { data: scenarios } = useQuery({ queryKey: ["scenarios"], queryFn: api.listScenarios });
  const { data: targets } = useQuery({ queryKey: ["targets"], queryFn: api.listTargets });

  const [scenarioPath, setScenarioPath] = useState("");
  const [from, setFrom] = useState("toollab");
  const [baseUrl, setBaseUrl] = useState("");
  const [result, setResult] = useState<ExecResult | null>(null);

  const mutation = useMutation({
    mutationFn: () =>
      api.execEnrich({ scenario_path: scenarioPath, from, target_base_url: baseUrl || undefined }),
    onSuccess: (data) => setResult(data),
  });

  return (
    <div className="space-y-4 animate-fade-in">
      <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">Escenario</label>
            {scenarios && scenarios.length > 0 ? (
              <div className="flex gap-2 flex-wrap">
                {scenarios.map((s: ScenarioFile) => (
                  <button
                    key={s.path}
                    onClick={() => setScenarioPath(s.path)}
                    className={`text-sm px-3 py-1.5 rounded-lg border transition-colors ${
                      scenarioPath === s.path
                        ? "border-accent bg-accent/10 text-accent"
                        : "border-border-subtle bg-surface hover:border-border text-text-secondary"
                    }`}
                  >
                    {s.name}
                  </button>
                ))}
              </div>
            ) : (
              <input
                value={scenarioPath}
                onChange={(e) => setScenarioPath(e.target.value)}
                placeholder="Ruta al escenario"
                className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
              />
            )}
          </div>
          <div>
            <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">Target URL</label>
            <input
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder="http://localhost:8080"
              className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
            />
            {targets && targets.length > 0 && (
              <div className="flex gap-1 mt-1.5 flex-wrap">
                {targets.map((t: Target) => (
                  <button
                    key={t.id}
                    onClick={() => setBaseUrl(t.base_url)}
                    className="text-xs bg-surface-overlay px-2 py-0.5 rounded text-accent hover:bg-accent/10 transition-colors"
                  >
                    {t.name}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>

        <div>
          <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">Source</label>
          <select
            value={from}
            onChange={(e) => setFrom(e.target.value)}
            className="bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
          >
            <option value="toollab">Toollab Adapter</option>
            <option value="openapi">OpenAPI</option>
          </select>
        </div>

        <button
          onClick={() => mutation.mutate()}
          disabled={mutation.isPending || !scenarioPath}
          className="bg-accent text-surface px-5 py-2.5 rounded-lg font-semibold text-sm hover:bg-accent-dim transition-colors disabled:opacity-40"
        >
          {mutation.isPending ? "Enriqueciendo…" : "Enriquecer escenario"}
        </button>
      </div>

      {result && <Terminal output={result.output} error={result.error} />}
    </div>
  );
}
