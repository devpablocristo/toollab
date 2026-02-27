import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../../lib/api";

export function Execute() {
  const qc = useQueryClient();
  const [localPath, setLocalPath] = useState("/workspace/nexus");
  const [llmEnabled, setLlmEnabled] = useState(true);
  const [createdRunId, setCreatedRunId] = useState<string | null>(null);
  const [lastError, setLastError] = useState<string | null>(null);

  const createRunMutation = useMutation({
    mutationFn: () =>
      api.createRun({
        source_type: "local_path",
        local_path: localPath,
        llm_enabled: llmEnabled,
      }),
    onSuccess: (data) => {
      setCreatedRunId(data.run_id);
      setLastError(null);
      qc.invalidateQueries({ queryKey: ["runs"] });
      qc.invalidateQueries({ queryKey: ["stats"] });
    },
    onError: (err: Error) => {
      setLastError(err.message);
    },
  });

  const primaryButtonClass =
    "bg-accent text-surface px-4 py-2 rounded-lg text-sm font-semibold hover:bg-accent-dim transition-colors disabled:opacity-40";

  return (
    <div className="space-y-6">
      <div className="animate-fade-in">
        <h1 className="text-2xl font-bold tracking-tight">Auditar</h1>
        <p className="text-text-secondary mt-1">
          ToolLab v2: análisis AST-first, determinista, sin pipeline legacy.
        </p>
      </div>

      <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 space-y-4">
        <div>
          <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">
            Source path (local_path)
          </label>
          <input
            value={localPath}
            onChange={(e) => setLocalPath(e.target.value)}
            className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
            placeholder="/workspace/nexus"
          />
        </div>

        <div className="flex items-center gap-2">
          <input
            id="llm-enabled"
            type="checkbox"
            checked={llmEnabled}
            onChange={(e) => setLlmEnabled(e.target.checked)}
            className="accent-accent"
          />
          <label htmlFor="llm-enabled" className="text-sm text-text-secondary">
            Ejecutar interpretación LLM (si está configurada)
          </label>
        </div>

        <div className="pt-2 border-t border-border-subtle">
          <button
            onClick={() => createRunMutation.mutate()}
            disabled={createRunMutation.isPending || !localPath}
            className={`w-full ${primaryButtonClass} py-3`}
          >
            {createRunMutation.isPending ? "Generando run..." : "Generar run"}
          </button>
        </div>
      </div>

      {lastError && (
        <div className="bg-fail/10 border border-fail/20 rounded-xl p-4 text-sm text-fail">
          Error al crear el run: {lastError}
        </div>
      )}

      {createdRunId && (
        <div className="bg-pass/10 border border-pass/20 rounded-xl p-4 text-sm">
          <span className="text-pass font-semibold">Run generado</span>
          <span className="text-text-secondary mx-2">|</span>
          <Link to={`/runs/${createdRunId}`} className="text-accent font-mono hover:underline">
            {createdRunId}
          </Link>
        </div>
      )}
    </div>
  );
}
