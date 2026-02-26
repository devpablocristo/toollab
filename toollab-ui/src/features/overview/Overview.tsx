import { useQuery } from "@tanstack/react-query";
import { api, type Run } from "../../lib/api";
import { StatCard } from "../../components/StatCard";
import { VerdictBadge } from "../../components/VerdictBadge";
import { Link } from "react-router-dom";
import {
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell,
} from "recharts";

const COLORS = { pass: "#00e89d", fail: "#ff3b5c" };

function VerdictPie({ passed, failed }: { passed: number; failed: number }) {
  const data = [
    { name: "Pass", value: passed },
    { name: "Fail", value: failed },
  ];
  if (passed + failed === 0) return null;
  return (
    <div className="lab-card p-5">
      <h3 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-4">Veredicto</h3>
      <ResponsiveContainer width="100%" height={180}>
        <PieChart>
          <Pie
            data={data}
            cx="50%"
            cy="50%"
            innerRadius={50}
            outerRadius={70}
            paddingAngle={4}
            dataKey="value"
            strokeWidth={0}
          >
            {data.map((entry) => (
              <Cell key={entry.name} fill={entry.name === "Pass" ? COLORS.pass : COLORS.fail} />
            ))}
          </Pie>
          <Tooltip
            contentStyle={{ background: "#0c1322", border: "1px solid #1c2840", borderRadius: 8, fontSize: 12 }}
            itemStyle={{ color: "#e0e6f0" }}
          />
        </PieChart>
      </ResponsiveContainer>
      <div className="flex justify-center gap-6 mt-2">
        <div className="flex items-center gap-2">
          <span className="w-2.5 h-2.5 rounded-full bg-pass" />
          <span className="text-xs text-text-secondary">{passed} pass</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="w-2.5 h-2.5 rounded-full bg-fail" />
          <span className="text-xs text-text-secondary">{failed} fail</span>
        </div>
      </div>
    </div>
  );
}

function LatencyChart({ runs }: { runs: Run[] }) {
  const data = [...runs].reverse().slice(-12).map((r) => ({
    id: r.id.slice(0, 6),
    P50: r.p50_ms,
    P95: r.p95_ms,
    P99: r.p99_ms,
  }));
  if (data.length === 0) return null;
  return (
    <div className="lab-card p-5">
      <h3 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-4">Latencia por run</h3>
      <ResponsiveContainer width="100%" height={180}>
        <BarChart data={data} barGap={1} barCategoryGap="20%">
          <XAxis dataKey="id" tick={{ fill: "#3d4e66", fontSize: 10, fontFamily: "JetBrains Mono" }} axisLine={false} tickLine={false} />
          <YAxis tick={{ fill: "#3d4e66", fontSize: 10 }} axisLine={false} tickLine={false} width={35} unit="ms" />
          <Tooltip
            contentStyle={{ background: "#0c1322", border: "1px solid #1c2840", borderRadius: 8, fontSize: 12 }}
            labelStyle={{ color: "#6b7a94", fontFamily: "JetBrains Mono", fontSize: 10 }}
            itemStyle={{ fontSize: 12 }}
          />
          <Bar dataKey="P50" fill="#00e89d" radius={[3, 3, 0, 0]} opacity={0.7} />
          <Bar dataKey="P95" fill="#ffb020" radius={[3, 3, 0, 0]} opacity={0.7} />
          <Bar dataKey="P99" fill="#ff3b5c" radius={[3, 3, 0, 0]} opacity={0.5} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}

function SuccessRateChart({ runs }: { runs: Run[] }) {
  const data = [...runs].reverse().slice(-12).map((r) => ({
    id: r.id.slice(0, 6),
    rate: +(r.success_rate * 100).toFixed(1),
    verdict: r.verdict,
  }));
  if (data.length === 0) return null;
  return (
    <div className="lab-card p-5">
      <h3 className="text-xs font-mono text-text-muted uppercase tracking-widest mb-4">Tasa de éxito por run</h3>
      <ResponsiveContainer width="100%" height={180}>
        <BarChart data={data}>
          <XAxis dataKey="id" tick={{ fill: "#3d4e66", fontSize: 10, fontFamily: "JetBrains Mono" }} axisLine={false} tickLine={false} />
          <YAxis tick={{ fill: "#3d4e66", fontSize: 10 }} axisLine={false} tickLine={false} width={35} unit="%" domain={[0, 100]} />
          <Tooltip
            contentStyle={{ background: "#0c1322", border: "1px solid #1c2840", borderRadius: 8, fontSize: 12 }}
            labelStyle={{ color: "#6b7a94", fontFamily: "JetBrains Mono", fontSize: 10 }}
            formatter={(v) => [`${v}%`, "Success"]}
          />
          <Bar dataKey="rate" radius={[3, 3, 0, 0]}>
            {data.map((d, i) => (
              <Cell key={i} fill={d.verdict === "pass" ? COLORS.pass : COLORS.fail} opacity={0.75} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}

export function Overview() {
  const { data: stats } = useQuery({ queryKey: ["stats"], queryFn: api.getStats });
  const { data: runs } = useQuery({ queryKey: ["runs"], queryFn: () => api.listRuns() });

  const recent = runs?.slice(0, 8) ?? [];
  const passRate = stats && stats.total_runs > 0
    ? ((stats.passed / stats.total_runs) * 100).toFixed(1)
    : "\u2014";

  return (
    <div className="space-y-10">
      <div className="animate-fade-in">
        <h1 className="text-3xl font-bold tracking-tight">Overview</h1>
        <p className="text-text-secondary mt-1">Estado general de las auditorías API</p>
        <div className="divider-accent mt-5" />
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 animate-fade-in stagger-1">
        <StatCard label="Targets" value={stats?.total_targets ?? 0} />
        <StatCard label="Total runs" value={stats?.total_runs ?? 0} />
        <StatCard label="Passed" value={stats?.passed ?? 0} accent />
        <StatCard label="Pass rate" value={`${passRate}%`} sub={`${stats?.failed ?? 0} failed`} />
      </div>

      {runs && runs.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 animate-fade-in stagger-2">
          <VerdictPie passed={stats?.passed ?? 0} failed={stats?.failed ?? 0} />
          <LatencyChart runs={runs} />
          <SuccessRateChart runs={runs} />
        </div>
      )}

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
              Ejecutá <code className="text-accent font-mono bg-accent/5 px-1.5 py-0.5 rounded">toollab run</code> y después ingesta los resultados
            </p>
          </div>
        ) : (
          <div className="lab-card overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-text-muted text-[10px] font-mono uppercase tracking-[0.2em]">
                  <th className="text-left px-5 py-3.5">Run ID</th>
                  <th className="text-left px-5 py-3.5">Verdict</th>
                  <th className="text-right px-5 py-3.5">Requests</th>
                  <th className="text-right px-5 py-3.5">Success</th>
                  <th className="text-right px-5 py-3.5">P95</th>
                  <th className="text-right px-5 py-3.5">Fecha</th>
                </tr>
              </thead>
              <tbody>
                {recent.map((r) => (
                  <tr key={r.id} className="border-b border-border-subtle/50 hover:bg-surface-overlay/30 transition-colors">
                    <td className="px-5 py-3.5">
                      <Link to={`/runs/${r.id}`} className="font-mono text-accent hover:text-glow transition-all">
                        {r.id.slice(0, 12)}&hellip;
                      </Link>
                    </td>
                    <td className="px-5 py-3.5"><VerdictBadge verdict={r.verdict} /></td>
                    <td className="px-5 py-3.5 text-right font-mono text-text-secondary">{r.total_requests}</td>
                    <td className="px-5 py-3.5 text-right font-mono text-text-secondary">{(r.success_rate * 100).toFixed(1)}%</td>
                    <td className="px-5 py-3.5 text-right font-mono text-text-secondary">{r.p95_ms}ms</td>
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
