export function Terminal({ output, error }: { output?: string; error?: string }) {
  if (!output && !error) return null;
  return (
    <div className="bg-[#0c0c14] border border-border-subtle rounded-xl overflow-hidden font-mono text-xs">
      <div className="flex items-center gap-1.5 px-4 py-2 bg-surface-raised border-b border-border-subtle">
        <span className="w-2.5 h-2.5 rounded-full bg-fail/60" />
        <span className="w-2.5 h-2.5 rounded-full bg-warning/60" />
        <span className="w-2.5 h-2.5 rounded-full bg-pass/60" />
        <span className="text-text-muted ml-2">output</span>
      </div>
      <pre className="p-4 overflow-x-auto max-h-80 overflow-y-auto leading-relaxed">
        {output && <span className="text-text-secondary">{output}</span>}
        {error && <span className="text-fail">{error}</span>}
      </pre>
    </div>
  );
}
