import { Routes, Route, Navigate } from 'react-router-dom'
import { useState, createContext, useContext } from 'react'
import ReposV2 from './pages/ReposV2'
import Targets from './pages/Targets'
import TargetDetail from './pages/TargetDetail'
import { appStorage } from './lib/storage'

type Lang = 'en' | 'es'
export const LangContext = createContext<{ lang: Lang; setLang: (l: Lang) => void }>({ lang: 'en', setLang: () => {} })
export function useLang() { return useContext(LangContext) }

function LangToggle() {
  const { lang, setLang } = useLang()
  return (
    <div className="flex items-center border border-edge rounded overflow-hidden text-[10px] font-mono">
      <button
        onClick={() => setLang('en')}
        className={`px-2 py-1 transition-colors ${lang === 'en' ? 'bg-accent/20 text-accent' : 'text-ghost hover:text-primary'}`}
      >EN</button>
      <button
        onClick={() => setLang('es')}
        className={`px-2 py-1 transition-colors ${lang === 'es' ? 'bg-accent/20 text-accent' : 'text-ghost hover:text-primary'}`}
      >ES</button>
    </div>
  )
}

export default function App() {
  const [lang, setLang] = useState<Lang>(() => (appStorage.getString('lang') as Lang) || 'en')
  const setAndPersist = (l: Lang) => { setLang(l); appStorage.setString('lang', l) }

  return (
    <LangContext.Provider value={{ lang, setLang: setAndPersist }}>
      <div className="min-h-screen flex flex-col">
        <header className="h-14 border-b border-edge flex items-center justify-between px-5 shrink-0 bg-surface/60 backdrop-blur-sm">
          <a href="/repos" className="flex items-center gap-2.5 group">
            <div className="w-2 h-2 rounded-full bg-accent animate-glow-pulse" />
            <span className="text-sm font-display font-bold tracking-[0.2em] uppercase text-accent text-glow-accent group-hover:tracking-[0.25em] transition-all">
              ToolLab
            </span>
          </a>
          <LangToggle />
        </header>
        <main className="flex-1 overflow-auto relative">
          <Routes>
            <Route path="/" element={<Navigate to="/repos" replace />} />
            <Route path="/repos" element={<ReposV2 />} />
            <Route path="/targets" element={<Targets />} />
            <Route path="/targets/:targetId" element={<TargetDetail />} />
          </Routes>
        </main>
      </div>
    </LangContext.Provider>
  )
}
