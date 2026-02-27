import { useQuery } from "@tanstack/react-query";
import { api, type Run } from "../../lib/api";
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer,
  AreaChart, Area, CartesianGrid,
} from "recharts";

export function Trends() {
  const { data: runs } = useQuery({ queryKey: ["runs"], queryFn: () => api.listRuns() });

  const sorted = [...(runs ?? [])].reverse();
  const latencyData = sorted.map((r: Run) => ({
    id: r.id.slice(0, 6),
    date: new Date(r.created_at).toLocaleDateString(),
    p50: r.p50_ms,
    p95: r.p95_ms,
    p99: r.p99_ms,
  }));
  const successData = sorted.map((r: Run) => ({
    id: r.id.slice(0, 6),
    date: new Date(r.created_at).toLocaleDateString(),
    rate: +(r.success_rate * 100).toFixed(1),
  }));
  const requestsData = sorted.map((r: Run) => ({
    id: r.id.slice(0, 6),
    date: new Date(r.created_at).toLocaleDateString(),
    requests: r.total_requests,
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
          <p className="text-text-secondary text-sm mt-2">Ejecutá al menos 2 runs para ver tendencias.</p>
        </div>
      </div>
    );
  }

  const lastTwo = sorted.slice(-2);
  const regressions: string[] = [];
  if (lastTwo.length === 2) {
    const [prev, curr] = lastTwo;
    if (curr.p95_ms > prev.p95_ms * 1.2) regressions.push(`P95 latency increased: ${prev.p95_ms}ms → ${curr.p95_ms}ms`);
    if (curr.success_rate < prev.success_rate * 0.95) regressions.push(`Success rate dropped: ${(prev.success_rate*100).toFixed(1)}% → ${(curr.success_rate*100).toFixed(1)}%`);
    if (curr.error_rate > prev.error_rate * 1.5 && curr.error_rate > 0.01) regressions.push(`Error rate increased: ${(prev.error_rate*100).toFixed(1)}% → ${(curr.error_rate*100).toFixed(1)}%`);
  }

  return (
    <div className="space-y-8">
      <div className="animate-fade-in">
        <h1 className="text-2xl font-bold tracking-tight">Tendencias</h1>
        <p className="text-text-secondary mt-1">Análisis temporal de {runs.length} runs</p>
        <div className="divider-accent mt-5" />
      </div>

      {regressions.length > 0 && (
        <div className="animate-fade-in bg-fail/5 border border-fail/20 rounded-xl p-5 space-y-2">
          <h3 className="text-fail font-semibold text-sm">Regresiones detectadas</h3>
          {regressions.map((r, i) => (
            <p key={i} className="text-text-secondary text-sm font-mono">{r}</p>
          ))}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 animate-fade-in stagger-1">
        <div className="lab-card p-5">
          <h3 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-4">Latencia a lo largo del tiempo</h3>
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={latencyData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#141e30" />
              <XAxis dataKey="id" tick={axisStyle} axisLine={false} tickLine={false} />
              <YAxis tick={axisStyle} axisLine={false} tickLine={false} width={40} unit="ms" />
              <Tooltip {...tooltipStyle} />
              <Line type="monotone" dataKey="p50" stroke="#00e89d" strokeWidth={2} dot={{ r: 3 }} name="P50" />
              <Line type="monotone" dataKey="p95" stroke="#ffb020" strokeWidth={2} dot={{ r: 3 }} name="P95" />
              <Line type="monotone" dataKey="p99" stroke="#ff3b5c" strokeWidth={2} dot={{ r: 3 }} name="P99" />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <div className="lab-card p-5">
          <h3 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-4">Tasa de éxito</h3>
          <ResponsiveContainer width="100%" height={250}>
            <AreaChart data={successData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#141e30" />
              <XAxis dataKey="id" tick={axisStyle} axisLine={false} tickLine={false} />
              <YAxis tick={axisStyle} axisLine={false} tickLine={false} width={40} unit="%" domain={[0, 100]} />
              <Tooltip {...tooltipStyle} />
              <defs>
                <linearGradient id="successGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#00e89d" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#00e89d" stopOpacity={0} />
                </linearGradient>
              </defs>
              <Area type="monotone" dataKey="rate" stroke="#00e89d" fill="url(#successGrad)" strokeWidth={2} name="Success %" />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        <div className="lab-card p-5">
          <h3 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-4">Requests por run</h3>
          <ResponsiveContainer width="100%" height={250}>
            <AreaChart data={requestsData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#141e30" />
              <XAxis dataKey="id" tick={axisStyle} axisLine={false} tickLine={false} />
              <YAxis tick={axisStyle} axisLine={false} tickLine={false} width={40} />
              <Tooltip {...tooltipStyle} />
              <defs>
                <linearGradient id="reqGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#6b7a94" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#6b7a94" stopOpacity={0} />
                </linearGradient>
              </defs>
              <Area type="monotone" dataKey="requests" stroke="#6b7a94" fill="url(#reqGrad)" strokeWidth={2} name="Requests" />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
}
