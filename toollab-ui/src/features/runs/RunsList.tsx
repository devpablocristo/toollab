import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../../lib/api";
import { VerdictBadge } from "../../components/VerdictBadge";

export function RunsList() {
  const qc = useQueryClient();
  const { data: runs, isLoading } = useQuery({
    queryKey: ["runs"],
    queryFn: () => api.listRuns(),
  });
  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.deleteRun(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["runs"] });
      qc.invalidateQueries({ queryKey: ["stats"] });
    },
  });

  return (
    <div className="space-y-8">
      <div className="animate-fade-in">
        <h1 className="text-3xl font-bold tracking-tight">Runs</h1>
        <p className="text-text-secondary mt-1">Historial de ejecuciones</p>
        <div className="divider-accent mt-5" />
      </div>

      {isLoading && (
        <div className="flex items-center gap-3 text-text-muted font-mono">
          <span className="w-1.5 h-1.5 rounded-full bg-accent animate-pulse" />
          Cargando&hellip;
        </div>
      )}

      {runs && runs.length === 0 && (
        <div className="lab-card lab-card--neutral p-16 text-center animate-fade-in">
          <p className="text-text-muted font-mono text-lg">Sin runs</p>
          <p className="text-text-secondary text-sm mt-3">
            Ingesta un run con{" "}
            <code className="text-accent font-mono bg-accent/5 px-1.5 py-0.5 rounded">
              POST /api/v1/runs/ingest
            </code>
          </p>
        </div>
      )}

      {runs && runs.length > 0 && (
        <div className="space-y-3 animate-fade-in stagger-1">
          {runs.map((r, i) => (
            <div
              key={r.id}
              className={`block lab-card ${r.verdict !== "pass" ? "lab-card--fail" : ""} p-5 group`}
              style={{ animationDelay: `${i * 40}ms` }}
            >
              <div className="flex items-center justify-between">
                <Link to={`/runs/${r.id}`} className="flex items-center justify-between gap-4 flex-1 min-w-0">
                  <div className="flex items-center gap-4 min-w-0">
                    <VerdictBadge verdict={r.verdict} />
                    <span className="font-mono text-sm text-text-secondary group-hover:text-accent transition-colors truncate">
                      {r.id.slice(0, 16)}&hellip;
                    </span>
                  </div>
                  <div className="flex items-center gap-6 text-sm font-mono text-text-secondary shrink-0">
                    <span>{r.total_requests} req</span>
                    <span>{(r.success_rate * 100).toFixed(1)}%</span>
                    <span>P95 {r.p95_ms}ms</span>
                    <span>{r.duration_s}s</span>
                    <span className="text-text-muted text-xs">
                      {new Date(r.created_at).toLocaleString()}
                    </span>
                  </div>
                </Link>
                <button
                  onClick={() => deleteMutation.mutate(r.id)}
                  disabled={deleteMutation.isPending}
                  className="ml-4 text-sm text-text-muted hover:text-fail transition-colors"
                >
                  Eliminar
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
