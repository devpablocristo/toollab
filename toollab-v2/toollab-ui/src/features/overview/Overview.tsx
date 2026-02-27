import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { StatCard } from "../../components/StatCard";
import { VerdictBadge } from "../../components/VerdictBadge";
import { Link } from "react-router-dom";

export function Overview() {
  const { data: stats } = useQuery({ queryKey: ["stats"], queryFn: api.getStats });
  const { data: runsWrap } = useQuery({ queryKey: ["runs"], queryFn: () => api.listRuns() });
  const runs = runsWrap?.items ?? [];

  const recent = runs.slice(0, 8);

  return (
    <div className="space-y-10">
      <div className="animate-fade-in">
        <h1 className="text-3xl font-bold tracking-tight">Overview</h1>
        <p className="text-text-secondary mt-1">Estado general de las auditorías API</p>
        <div className="divider-accent mt-5" />
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 animate-fade-in stagger-1">
        <StatCard label="Total runs" value={stats?.total_runs ?? 0} />
        <StatCard label="Queued" value={stats?.queued ?? 0} />
        <StatCard label="Running" value={stats?.running ?? 0} />
        <StatCard label="Succeeded" value={stats?.succeeded ?? 0} accent />
        <StatCard label="Failed" value={stats?.failed ?? 0} />
      </div>

      <div className="animate-fade-in stagger-3">
        <div className="flex items-center justify-between mb-5">
          <h2 className="text-xl font-semibold">Runs recientes</h2>
          <Link to="/runs" className="text-accent text-sm font-medium hover:text-accent-dim transition-colors">
            Ver todos &rarr;
          </Link>
        </div>

        {recent.length === 0 ? (
          <div className="lab-card lab-card--neutral p-16 text-center">
            <p className="text-text-muted font-mono text-lg">Sin runs todavía</p>
            <p className="text-text-secondary text-sm mt-3">
              Crea un run desde <code className="text-accent font-mono bg-accent/5 px-1.5 py-0.5 rounded">Auditar</code>
            </p>
          </div>
        ) : (
          <div className="lab-card overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-text-muted text-[10px] font-mono uppercase tracking-[0.2em]">
                  <th className="text-left px-5 py-3.5">Run ID</th>
                  <th className="text-left px-5 py-3.5">Status</th>
                  <th className="text-left px-5 py-3.5">Source</th>
                  <th className="text-right px-5 py-3.5">Fecha</th>
                </tr>
              </thead>
              <tbody>
                {recent.map((r) => (
                  <tr key={r.id} className="border-b border-border-subtle/50 hover:bg-surface-overlay/30 transition-colors">
                    <td className="px-5 py-3.5">
                      <Link to={`/runs/${r.id}`} className="font-mono text-accent hover:text-glow transition-all">
                        {r.id}
                      </Link>
                    </td>
                    <td className="px-5 py-3.5"><VerdictBadge verdict={r.status} /></td>
                    <td className="px-5 py-3.5 font-mono text-xs text-text-secondary">{r.source_ref}</td>
                    <td className="px-5 py-3.5 text-right text-text-muted text-xs">{new Date(r.created_at).toLocaleDateString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
