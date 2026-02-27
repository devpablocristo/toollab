export function StatCard({
  label,
  value,
  sub,
  accent,
}: {
  label: string;
  value: string | number;
  sub?: string;
  accent?: boolean;
}) {
  return (
    <div className={`lab-card ${accent ? "" : "lab-card--neutral"} p-5`}>
      <p className="text-text-muted text-[10px] font-mono uppercase tracking-[0.2em] mb-2">
        {label}
      </p>
      <p
        className={`text-3xl font-display font-bold tracking-tight ${
          accent ? "text-accent text-glow" : "text-text-primary"
        }`}
      >
        {value}
      </p>
      {sub && (
        <p className="text-text-secondary text-xs font-mono mt-1.5">{sub}</p>
      )}
    </div>
  );
}
