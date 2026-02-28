import { type ReactNode } from 'react'

/* ── Color constants ── */
const TRACK = '#1e2235'

/* ── Score Ring: big arc gauge with number in center ── */

export function ScoreRing({ score, grade, size = 120, stroke = 10 }: { score: number; grade: string; size?: number; stroke?: number }) {
  const r = (size - stroke) / 2
  const circ = 2 * Math.PI * r
  const pct = Math.max(0, Math.min(100, score)) / 100
  const offset = circ * (1 - pct)
  const c = size / 2
  const gradeColor = grade === 'A' ? '#3dd68c' : grade === 'B' ? '#52a8ff' : grade === 'C' ? '#ffb224' : grade === 'D' ? '#ff8c42' : '#ff4f5e'

  return (
    <div className="relative inline-flex items-center justify-center">
      <svg width={size} height={size} className="-rotate-90">
        <circle cx={c} cy={c} r={r} fill="none" stroke={TRACK} strokeWidth={stroke} />
        <circle cx={c} cy={c} r={r} fill="none" stroke={gradeColor} strokeWidth={stroke}
          strokeDasharray={circ} strokeDashoffset={offset} strokeLinecap="round"
          className="transition-all duration-700"
          style={{ filter: `drop-shadow(0 0 6px ${gradeColor}40)` }} />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-2xl font-display font-bold" style={{ color: gradeColor, textShadow: `0 0 12px ${gradeColor}40` }}>{score}</span>
        <span className="text-[10px] font-mono text-ghost">/ 100</span>
      </div>
    </div>
  )
}

/* ── Donut Chart: multi-segment ring ── */

interface DonutSegment { label: string; value: number; color: string }

export function DonutChart({ segments, size = 110, stroke = 14, children }: { segments: DonutSegment[]; size?: number; stroke?: number; children?: ReactNode }) {
  const r = (size - stroke) / 2
  const circ = 2 * Math.PI * r
  const c = size / 2
  const total = segments.reduce((s, seg) => s + seg.value, 0)
  if (total === 0) return null

  let accumulated = 0
  const arcs = segments.filter(s => s.value > 0).map(seg => {
    const pct = seg.value / total
    const dashLen = circ * pct
    const dashGap = circ - dashLen
    const rotDeg = (accumulated / total) * 360 - 90
    accumulated += seg.value
    return { ...seg, dashLen, dashGap, rotDeg }
  })

  return (
    <div className="relative inline-flex items-center justify-center">
      <svg width={size} height={size}>
        <circle cx={c} cy={c} r={r} fill="none" stroke={TRACK} strokeWidth={stroke} />
        {arcs.map((a, i) => (
          <circle key={i} cx={c} cy={c} r={r} fill="none" stroke={a.color} strokeWidth={stroke}
            strokeDasharray={`${a.dashLen} ${a.dashGap}`}
            transform={`rotate(${a.rotDeg} ${c} ${c})`} className="transition-all duration-500" />
        ))}
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        {children}
      </div>
    </div>
  )
}

export function DonutLegend({ segments }: { segments: DonutSegment[] }) {
  const total = segments.reduce((s, seg) => s + seg.value, 0)
  return (
    <div className="flex flex-wrap gap-x-4 gap-y-1">
      {segments.filter(s => s.value > 0).map((s, i) => (
        <div key={i} className="flex items-center gap-1.5 text-xs font-body">
          <span className="w-2 h-2 rounded-full shrink-0" style={{ background: s.color, boxShadow: `0 0 6px ${s.color}40` }} />
          <span className="text-ghost">{s.label}</span>
          <span className="text-zinc-300 font-semibold">{s.value}</span>
          <span className="text-ghost-faint">({total > 0 ? ((s.value / total) * 100).toFixed(0) : 0}%)</span>
        </div>
      ))}
    </div>
  )
}

/* ── Horizontal Bar Chart ── */

interface HBar { label: string; value: number; max?: number; color: string; suffix?: string }

export function HBarChart({ bars, height = 24 }: { bars: HBar[]; height?: number }) {
  const maxVal = Math.max(...bars.map(b => b.max ?? b.value), 1)
  return (
    <div className="space-y-2">
      {bars.map((b, i) => {
        const pct = Math.min((b.value / maxVal) * 100, 100)
        return (
          <div key={i} className="flex items-center gap-3">
            <span className="text-xs font-mono text-ghost w-20 text-right shrink-0">{b.label}</span>
            <div className="flex-1 bg-surface rounded-sm overflow-hidden" style={{ height }}>
              <div className="h-full rounded-sm transition-all duration-500 flex items-center justify-end pr-2"
                style={{ width: `${Math.max(pct, 2)}%`, background: `linear-gradient(90deg, ${b.color}80, ${b.color})` }}>
                {pct > 15 && <span className="text-[10px] font-mono font-semibold text-obsidian/90">{b.value}{b.suffix ?? ''}</span>}
              </div>
            </div>
            {pct <= 15 && <span className="text-xs font-mono text-ghost shrink-0">{b.value}{b.suffix ?? ''}</span>}
          </div>
        )
      })}
    </div>
  )
}

/* ── Stacked Horizontal Bar ── */

interface StackSegment { label: string; value: number; color: string }

export function StackedBar({ segments, height = 28 }: { segments: StackSegment[]; height?: number }) {
  const total = segments.reduce((s, seg) => s + seg.value, 0)
  if (total === 0) return <div className="bg-surface rounded-sm" style={{ height }} />
  return (
    <div className="flex rounded-sm overflow-hidden" style={{ height }}>
      {segments.filter(s => s.value > 0).map((s, i) => (
        <div key={i} className="flex items-center justify-center transition-all duration-500"
          style={{ width: `${(s.value / total) * 100}%`, background: s.color }}
          title={`${s.label}: ${s.value}`}>
          {(s.value / total) > 0.08 && (
            <span className="text-[10px] font-mono font-bold text-obsidian/90">{s.value}</span>
          )}
        </div>
      ))}
    </div>
  )
}

/* ── Mini Sparkline Bar (inline small bars) ── */

export function SparkBars({ values, colors, maxVal, height = 32 }: { values: number[]; colors: string[]; maxVal?: number; height?: number }) {
  const mx = maxVal ?? Math.max(...values, 1)
  const gap = 2
  const barW = Math.max(4, Math.min(12, 100 / values.length))
  const totalW = values.length * (barW + gap)
  return (
    <svg width={totalW} height={height} className="shrink-0">
      {values.map((v, i) => {
        const h = Math.max(2, (v / mx) * (height - 2))
        return <rect key={i} x={i * (barW + gap)} y={height - h} width={barW} height={h} rx={1} fill={colors[i % colors.length]} />
      })}
    </svg>
  )
}

/* ── Percentage Ring (small) ── */

export function PercentRing({ value, size = 56, stroke = 5, color = '#52a8ff' }: { value: number; size?: number; stroke?: number; color?: string }) {
  const r = (size - stroke) / 2
  const circ = 2 * Math.PI * r
  const c = size / 2
  const pct = Math.max(0, Math.min(100, value))
  const offset = circ * (1 - pct / 100)

  return (
    <div className="relative inline-flex items-center justify-center">
      <svg width={size} height={size} className="-rotate-90">
        <circle cx={c} cy={c} r={r} fill="none" stroke={TRACK} strokeWidth={stroke} />
        <circle cx={c} cy={c} r={r} fill="none" stroke={color} strokeWidth={stroke}
          strokeDasharray={circ} strokeDashoffset={offset} strokeLinecap="round"
          className="transition-all duration-500"
          style={{ filter: `drop-shadow(0 0 4px ${color}30)` }} />
      </svg>
      <span className="absolute text-xs font-display font-bold" style={{ color, textShadow: `0 0 8px ${color}30` }}>{pct.toFixed(0)}%</span>
    </div>
  )
}

/* ── Heat row: colored cells in a row ── */

export function HeatRow({ cells }: { cells: { label: string; value: number; color: string }[] }) {
  return (
    <div className="flex gap-1">
      {cells.map((cell, i) => (
        <div key={i} className="flex flex-col items-center gap-0.5" title={`${cell.label}: ${cell.value}`}>
          <div className="w-7 h-7 rounded-sm flex items-center justify-center text-[10px] font-mono font-bold"
            style={{ background: cell.color + '20', color: cell.color }}>
            {cell.value}
          </div>
          <span className="text-[9px] text-ghost-faint">{cell.label}</span>
        </div>
      ))}
    </div>
  )
}
