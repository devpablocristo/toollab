import { Routes, Route, Navigate } from 'react-router-dom'
import Targets from './pages/Targets'
import TargetDetail from './pages/TargetDetail'

export default function App() {
  return (
    <div className="min-h-screen flex flex-col">
      <header className="h-12 border-b border-gray-800 flex items-center px-4 gap-3 shrink-0">
        <a href="/targets" className="text-sm font-bold tracking-wide text-blue-400 hover:text-blue-300">
          ToolLab
        </a>
        <span className="text-xs text-gray-500">v4</span>
      </header>
      <main className="flex-1 overflow-auto">
        <Routes>
          <Route path="/" element={<Navigate to="/targets" replace />} />
          <Route path="/targets" element={<Targets />} />
          <Route path="/targets/:targetId" element={<TargetDetail />} />
        </Routes>
      </main>
    </div>
  )
}
