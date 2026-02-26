import { BrowserRouter, Routes, Route, NavLink } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Overview } from "./features/overview/Overview";
import { RunsList } from "./features/runs/RunsList";
import { RunDetail } from "./features/runs/RunDetail";
import { TargetsList } from "./features/targets/TargetsList";
import { Execute } from "./features/execute/Execute";
import { Trends } from "./features/trends/Trends";

const qc = new QueryClient({
  defaultOptions: { queries: { staleTime: 10_000, retry: 1 } },
});

function Nav() {
  const link = (to: string, label: string) => (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
          isActive
            ? "bg-surface-overlay text-accent"
            : "text-text-secondary hover:text-text-primary"
        }`
      }
    >
      {label}
    </NavLink>
  );

  return (
    <nav className="border-b border-border-subtle bg-surface-raised/80 backdrop-blur-sm sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-6 h-14 flex items-center gap-1">
        <span className="font-mono font-bold text-accent tracking-tight mr-6 text-lg">
          toollab
        </span>
        {link("/", "Overview")}
        {link("/execute", "Ejecutar")}
        {link("/runs", "Runs")}
        {link("/trends", "Tendencias")}
        {link("/targets", "Targets")}
      </div>
    </nav>
  );
}

export default function App() {
  return (
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <Nav />
        <main className="max-w-7xl mx-auto px-6 py-8">
          <Routes>
            <Route path="/" element={<Overview />} />
            <Route path="/execute" element={<Execute />} />
            <Route path="/runs" element={<RunsList />} />
            <Route path="/runs/:id" element={<RunDetail />} />
            <Route path="/trends" element={<Trends />} />
            <Route path="/targets" element={<TargetsList />} />
          </Routes>
        </main>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
