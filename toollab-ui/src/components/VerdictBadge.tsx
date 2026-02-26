export function VerdictBadge({ verdict }: { verdict: string }) {
  const pass = verdict === "pass";
  return (
    <span
      className={`inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-mono font-semibold uppercase tracking-wider ${
        pass
          ? "bg-pass/8 text-pass border border-pass/15 glow-accent"
          : "bg-fail/8 text-fail border border-fail/15 glow-fail"
      }`}
    >
      <span
        className={`w-1.5 h-1.5 rounded-full ${pass ? "bg-pass" : "bg-fail"}`}
        style={{
          animation: "pulse-dot 2s ease-in-out infinite",
          boxShadow: pass
            ? "0 0 6px rgba(0,232,157,0.6)"
            : "0 0 6px rgba(255,59,92,0.6)",
        }}
      />
      {verdict}
    </span>
  );
}
