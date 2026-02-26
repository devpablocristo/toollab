import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, type FullAuditResult, type Target } from "../../lib/api";
import { Terminal } from "../../components/Terminal";

export function Execute() {
  const qc = useQueryClient();
  const { data: targets = [] } = useQuery({ queryKey: ["targets"], queryFn: api.listTargets });

  const [selectedTargetId, setSelectedTargetId] = useState("");
  const [showTargetForm, setShowTargetForm] = useState(false);
  const [newTargetName, setNewTargetName] = useState("");
  const [newTargetBaseURL, setNewTargetBaseURL] = useState("");
  const [newTargetDescription, setNewTargetDescription] = useState("");
  const [mode, setMode] = useState("smoke");
  const [result, setResult] = useState<FullAuditResult | null>(null);

  const selectedTarget = targets.find((t) => t.id === selectedTargetId);

  useEffect(() => {
    if (!selectedTargetId && targets.length > 0) {
      setSelectedTargetId(targets[0].id);
    }
  }, [targets, selectedTargetId]);

  const createTargetMutation = useMutation({
    mutationFn: () =>
      api.createTarget({
        name: newTargetName,
        base_url: newTargetBaseURL,
        description: newTargetDescription || undefined,
      }),
    onSuccess: (created) => {
      setSelectedTargetId(created.id);
      setShowTargetForm(false);
      setNewTargetName("");
      setNewTargetBaseURL("");
      setNewTargetDescription("");
      qc.invalidateQueries({ queryKey: ["targets"] });
    },
  });

  const removeTargetMutation = useMutation({
    mutationFn: (id: string) => api.deleteTarget(id),
    onSuccess: (_, id) => {
      if (selectedTargetId === id) {
        setSelectedTargetId("");
      }
      qc.invalidateQueries({ queryKey: ["targets"] });
    },
  });

  const fullAuditMutation = useMutation({
    mutationFn: () =>
      api.execFullAudit({
        base_url: selectedTarget?.base_url ?? "",
        target_id: selectedTarget?.id,
        target_name: selectedTarget?.name,
        mode,
      }),
    onSuccess: (data) => {
      setResult(data);
      qc.invalidateQueries({ queryKey: ["runs"] });
      qc.invalidateQueries({ queryKey: ["stats"] });
    },
  });

  const primaryButtonClass =
    "bg-accent text-surface px-4 py-2 rounded-lg text-sm font-semibold hover:bg-accent-dim transition-colors disabled:opacity-40";

  return (
    <div className="space-y-6">
      <div className="animate-fade-in">
        <h1 className="text-2xl font-bold tracking-tight">Auditar</h1>
        <p className="text-text-secondary mt-1">
          Configura todo en una sola vista y ejecuta una auditoría completa con un solo botón.
        </p>
      </div>

      <div className="bg-surface-raised border border-border-subtle rounded-xl p-5 space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-text-secondary">Targets</h2>
          <button
            onClick={() => setShowTargetForm((v) => !v)}
            className={primaryButtonClass}
          >
            {showTargetForm ? "Cancelar" : "+ Nuevo target"}
          </button>
        </div>

        {showTargetForm && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              createTargetMutation.mutate();
            }}
            className="space-y-3"
          >
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <input
                value={newTargetName}
                onChange={(e) => setNewTargetName(e.target.value)}
                placeholder="Nombre del target"
                className="bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
                required
              />
              <input
                value={newTargetBaseURL}
                onChange={(e) => setNewTargetBaseURL(e.target.value)}
                placeholder="http://localhost:8080"
                className="bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
                required
              />
            </div>
            <input
              value={newTargetDescription}
              onChange={(e) => setNewTargetDescription(e.target.value)}
              placeholder="Descripción (opcional)"
              className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
            />
            <button
              type="submit"
              disabled={createTargetMutation.isPending || !newTargetName || !newTargetBaseURL}
              className={primaryButtonClass}
            >
              {createTargetMutation.isPending ? "Guardando..." : "Guardar target"}
            </button>
          </form>
        )}

        <div className="space-y-2">
          <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">
            Target
          </label>
          <div className="flex items-center gap-2">
            <select
              value={selectedTargetId}
              onChange={(e) => setSelectedTargetId(e.target.value)}
              className="w-full bg-surface border border-border-subtle rounded-lg px-3 py-2 text-sm focus:border-accent outline-none"
            >
              {targets.length === 0 ? (
                <option value="">Sin targets</option>
              ) : (
                targets.map((t: Target) => (
                  <option key={t.id} value={t.id}>
                    {t.name} ({t.base_url})
                  </option>
                ))
              )}
            </select>
            {selectedTarget && (
              <button
                onClick={() => removeTargetMutation.mutate(selectedTarget.id)}
                disabled={removeTargetMutation.isPending}
                className={primaryButtonClass}
              >
                Eliminar
              </button>
            )}
          </div>
          {selectedTarget?.description && (
            <p className="text-xs text-text-muted">{selectedTarget.description}</p>
          )}
        </div>

        <div className="pt-2 border-t border-border-subtle space-y-3">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-xs font-mono text-text-muted uppercase tracking-widest mb-1.5">
                Modo
              </label>
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

          <button
            onClick={() => fullAuditMutation.mutate()}
            disabled={fullAuditMutation.isPending || !selectedTarget}
            className={`w-full ${primaryButtonClass} py-3`}
          >
            {fullAuditMutation.isPending ? "Ejecutando auditoría completa..." : "Generar reporte completo"}
          </button>
        </div>
      </div>

      {result && (
        <div className="space-y-3">
          {result.run_id && (
            <div className="bg-pass/10 border border-pass/20 rounded-xl p-4 text-sm">
              <span className="text-pass font-semibold">Run generado: </span>
              <Link to={`/runs/${result.run_id}`} className="text-accent font-mono hover:underline">
                {result.run_id}
              </Link>
            </div>
          )}
          {result.steps?.map((step, idx) => (
            <Terminal key={`${step.step}-${idx}`} output={step.output ?? ""} error={step.error} />
          ))}
        </div>
      )}
    </div>
  );
}
