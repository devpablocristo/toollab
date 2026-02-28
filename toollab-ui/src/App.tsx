import { Routes, Route, Navigate } from 'react-router-dom'
import Targets from './pages/Targets'
import TargetDetail from './pages/TargetDetail'

export default function App() {
  return (
    <div className="min-h-screen flex flex-col">
      <header className="h-14 border-b border-edge flex items-center px-5 gap-3 shrink-0 bg-surface/60 backdrop-blur-sm">
        <a href="/targets" className="flex items-center gap-2.5 group">
          <div className="w-2 h-2 rounded-full bg-accent animate-glow-pulse" />
          <span className="text-sm font-display font-bold tracking-[0.2em] uppercase text-accent text-glow-accent group-hover:tracking-[0.25em] transition-all">
            ToolLab
          </span>
        </a>
        <span className="text-[10px] font-mono text-ghost px-2 py-0.5 border border-edge rounded-sm">v4</span>
        <div className="flex-1" />
        <span className="text-[10px] font-mono text-ghost/50 tracking-wider">SECURITY AUDIT LABORATORY</span>
      </header>
      <main className="flex-1 overflow-auto relative">
        <Routes>
          <Route path="/" element={<Navigate to="/targets" replace />} />
          <Route path="/targets" element={<Targets />} />
          <Route path="/targets/:targetId" element={<TargetDetail />} />
        </Routes>
      </main>
    </div>
  )
}
