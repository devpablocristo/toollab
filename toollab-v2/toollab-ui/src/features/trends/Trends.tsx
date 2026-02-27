import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid,
} from "recharts";

export function Trends() {
  const { data: wraps } = useQuery({ queryKey: ["runs"], queryFn: () => api.listRuns(), refetchInterval: 3000 });
  const runs = wraps?.items ?? [];

  const sorted = [...runs].reverse();
  const statusData = sorted.map((r) => ({
    id: r.id.slice(0, 6),
    status: r.status === "succeeded" ? 1 : r.status === "running" ? 0.66 : r.status === "queued" ? 0.33 : 0,
  }));

  const tooltipStyle = {
    contentStyle: { background: "#0c1322", border: "1px solid #1c2840", borderRadius: 8, fontSize: 12 },
    labelStyle: { color: "#6b7a94", fontFamily: "JetBrains Mono", fontSize: 10 },
  };

  const axisStyle = { fill: "#3d4e66", fontSize: 10, fontFamily: "JetBrains Mono" };

  if (!runs || runs.length === 0) {
    return (
      <div className="space-y-6">
        <div className="animate-fade-in">
          <h1 className="text-2xl font-bold tracking-tight">Tendencias</h1>
          <p className="text-text-secondary mt-1">Análisis temporal de los runs</p>
        </div>
        <div className="lab-card lab-card--neutral p-16 text-center">
          <p className="text-text-muted font-mono text-lg">Sin datos suficientes</p>
          <p className="text-text-secondary text-sm mt-2">Generá runs para ver tendencias.</p>
        </div>
      </div>
    );
  }

  const recentWindow = sorted.slice(-20);
  const succeeded = recentWindow.filter((r) => r.status === "succeeded").length;
  const failed = recentWindow.filter((r) => r.status === "failed").length;

  return (
    <div className="space-y-8">
      <div className="animate-fade-in">
        <h1 className="text-2xl font-bold tracking-tight">Tendencias</h1>
        <p className="text-text-secondary mt-1">Análisis temporal de {runs.length} runs</p>
        <div className="divider-accent mt-5" />
      </div>

      <div className="animate-fade-in bg-surface-raised border border-border-subtle rounded-xl p-5 space-y-2">
        <h3 className="text-text-primary font-semibold text-sm">Últimos 20 runs</h3>
        <p className="text-text-secondary text-sm font-mono">
          succeeded: {succeeded} · failed: {failed} · otros: {recentWindow.length - succeeded - failed}
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-1 gap-6 animate-fade-in stagger-1">
        <div className="lab-card p-5">
          <h3 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-4">Estado por run (0-1)</h3>
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={statusData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#141e30" />
              <XAxis dataKey="id" tick={axisStyle} axisLine={false} tickLine={false} />
              <YAxis tick={axisStyle} axisLine={false} tickLine={false} width={40} domain={[0, 1]} />
              <Tooltip {...tooltipStyle} />
              <Line type="monotone" dataKey="status" stroke="#00e89d" strokeWidth={2} dot={{ r: 3 }} name="status" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
}
