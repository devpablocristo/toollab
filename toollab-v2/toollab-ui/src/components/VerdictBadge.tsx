export function VerdictBadge({ verdict }: { verdict: string }) {
  const ok = verdict === "pass" || verdict === "succeeded";
  const running = verdict === "running" || verdict === "queued";
  return (
    <span
      className={`inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-mono font-semibold uppercase tracking-wider ${
        ok
          ? "bg-pass/8 text-pass border border-pass/15 glow-accent"
          : running
            ? "bg-warning/10 text-warning border border-warning/20"
            : "bg-fail/8 text-fail border border-fail/15 glow-fail"
      }`}
    >
      <span
        className={`w-1.5 h-1.5 rounded-full ${ok ? "bg-pass" : running ? "bg-warning" : "bg-fail"}`}
        style={{
          animation: "pulse-dot 2s ease-in-out infinite",
          boxShadow: ok
            ? "0 0 6px rgba(0,232,157,0.6)"
            : running
              ? "0 0 6px rgba(255,176,32,0.6)"
              : "0 0 6px rgba(255,59,92,0.6)",
        }}
      />
      {verdict}
    </span>
  );
}
