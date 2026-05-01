import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import Spinner from '../components/Spinner'
import { api } from '../lib/api'
import type { AuditConfig, AuditRun, Evidence, Finding, GeneratedDoc, Project, ScoreItem, TestResult } from '../lib/types'

const defaultConfig: AuditConfig = {
  generate_tests: true,
  run_existing_tests: true,
  allow_docs_read: false,
  allow_dependency_install: false,
}

export default function ProjectAuditor() {
  const queryClient = useQueryClient()
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [selectedAuditId, setSelectedAuditId] = useState<string | null>(null)
  const [name, setName] = useState('toollab')
  const [sourcePath, setSourcePath] = useState('/home/pablocristo/Proyectos/pablo/toollab')
  const [config, setConfig] = useState<AuditConfig>(defaultConfig)

  const { data: projects, isLoading: projectsLoading } = useQuery({ queryKey: ['projects'], queryFn: api.projects.list })
  const selectedProject = projects?.find(project => project.id === selectedProjectId) ?? projects?.[0] ?? null

  const { data: audits } = useQuery({
    queryKey: ['project-audits', selectedProject?.id],
    queryFn: () => api.projects.audits(selectedProject!.id),
    enabled: !!selectedProject,
  })
  const selectedAudit = selectedAuditId ?? audits?.[0]?.id ?? null

  const { data: audit } = useQuery({
    queryKey: ['audit', selectedAudit],
    queryFn: () => api.audits.get(selectedAudit!),
    enabled: !!selectedAudit,
  })
  const { data: findings } = useQuery({
    queryKey: ['audit-findings', selectedAudit],
    queryFn: () => api.audits.findings(selectedAudit!),
    enabled: !!selectedAudit,
  })
  const { data: evidence } = useQuery({
    queryKey: ['audit-evidence', selectedAudit],
    queryFn: () => api.audits.evidence(selectedAudit!),
    enabled: !!selectedAudit,
  })
  const { data: docs } = useQuery({
    queryKey: ['audit-docs', selectedAudit],
    queryFn: () => api.audits.docs(selectedAudit!),
    enabled: !!selectedAudit,
  })
  const { data: tests } = useQuery({
    queryKey: ['audit-tests', selectedAudit],
    queryFn: () => api.audits.tests(selectedAudit!),
    enabled: !!selectedAudit,
  })
  const { data: scoreItems } = useQuery({
    queryKey: ['audit-score', selectedAudit],
    queryFn: () => api.audits.score(selectedAudit!),
    enabled: !!selectedAudit,
  })

  const createProject = useMutation({
    mutationFn: () => api.projects.create({ name, source_path: sourcePath }),
    onSuccess: project => {
      setSelectedProjectId(project.id)
      setSelectedAuditId(null)
      queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })

  const createAudit = useMutation({
    mutationFn: (project: Project) => api.projects.createAudit(project.id, config),
    onSuccess: result => {
      setSelectedAuditId(result.run.id)
      queryClient.invalidateQueries({ queryKey: ['project-audits', result.run.project_id] })
      queryClient.invalidateQueries({ queryKey: ['audit', result.run.id] })
      queryClient.invalidateQueries({ queryKey: ['audit-findings', result.run.id] })
      queryClient.invalidateQueries({ queryKey: ['audit-evidence', result.run.id] })
      queryClient.invalidateQueries({ queryKey: ['audit-docs', result.run.id] })
      queryClient.invalidateQueries({ queryKey: ['audit-tests', result.run.id] })
      queryClient.invalidateQueries({ queryKey: ['audit-score', result.run.id] })
    },
  })

  const orderedAudits = useMemo(() => audits ?? [], [audits])

  return (
    <div className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">ToolLab MVP</p>
          <h1>AI Project Auditor</h1>
          <p className="subtitle">Load a local project, start an analysis, and review evidence-backed results.</p>
        </div>
      </header>

      <section className="steps">
        <Step number="1" title="Load project" description="Register a local path for ToolLab to audit." />
        <Step number="2" title="Start analysis" description="Run inventory, deterministic checks, tests, and scoring." />
        <Step number="3" title="Review results" description="Inspect score, findings, evidence, docs, and tests." />
      </section>

      <main className="layout">
        <aside className="sidebar">
          <section className="card stack">
            <h2>1. Load project</h2>
            <input value={name} onChange={event => setName(event.target.value)} placeholder="Project name" />
            <input value={sourcePath} onChange={event => setSourcePath(event.target.value)} placeholder="Local project path" />
            <div className="options">
              <p>Analysis options</p>
              <Toggle label="Run existing tests" checked={config.run_existing_tests} onChange={value => setConfig({ ...config, run_existing_tests: value })} />
              <Toggle label="Generate smoke-test signal" checked={config.generate_tests} onChange={value => setConfig({ ...config, generate_tests: value })} />
              <Toggle label="Read existing docs" checked={config.allow_docs_read} onChange={value => setConfig({ ...config, allow_docs_read: value })} />
              <Toggle label="Allow dependency install" checked={config.allow_dependency_install} onChange={value => setConfig({ ...config, allow_dependency_install: value })} />
            </div>
            <button disabled={!name || !sourcePath || createProject.isPending} onClick={() => createProject.mutate()}>
              {createProject.isPending ? 'Loading...' : 'Load project'}
            </button>
            {createProject.error && <ErrorText error={createProject.error} />}
          </section>

          <section className="card stack">
            <h2>Loaded projects</h2>
            {projectsLoading ? <Spinner /> : (
              <div className="list">
                {(projects ?? []).map(project => (
                  <button key={project.id} className={project.id === selectedProject?.id ? 'list-item active' : 'list-item'} onClick={() => { setSelectedProjectId(project.id); setSelectedAuditId(null) }}>
                    <strong>{project.name}</strong>
                    <span>{project.source_path}</span>
                  </button>
                ))}
                {(projects ?? []).length === 0 && <p className="muted">No projects loaded yet.</p>}
              </div>
            )}
          </section>
        </aside>

        <section className="content">
          {selectedProject ? (
            <>
              <section className="card project-header">
                <div>
                  <p className="eyebrow">Selected project</p>
                  <h2>{selectedProject.name}</h2>
                  <p className="muted">{selectedProject.source_path}</p>
                </div>
                <button disabled={createAudit.isPending} onClick={() => createAudit.mutate(selectedProject)}>
                  {createAudit.isPending ? 'Analyzing...' : 'Start analysis'}
                </button>
              </section>
              {createAudit.error && <ErrorText error={createAudit.error} />}
              <AuditPicker audits={orderedAudits} selectedAuditId={selectedAudit} onSelect={setSelectedAuditId} />
              {audit ? (
                <section className="results">
                  <h2>3. Results</h2>
                  <Overview audit={audit} />
                  <ScorePanel items={scoreItems ?? []} />
                  <FindingsPanel findings={findings ?? []} />
                  <EvidencePanel evidence={evidence ?? []} />
                  <DocsPanel docs={docs ?? []} />
                  <TestsPanel tests={tests ?? []} />
                </section>
              ) : (
                <section className="card empty-state">Start an analysis to see results.</section>
              )}
            </>
          ) : (
            <section className="card empty-state">Load or select a project to start.</section>
          )}
        </section>
      </main>
    </div>
  )
}

function Step({ number, title, description }: { number: string; title: string; description: string }) {
  return (
    <article className="card step">
      <span>{number}</span>
      <div>
        <h2>{title}</h2>
        <p>{description}</p>
      </div>
    </article>
  )
}

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="toggle">
      <span>{label}</span>
      <input type="checkbox" checked={checked} onChange={event => onChange(event.target.checked)} />
    </label>
  )
}

function AuditPicker({ audits, selectedAuditId, onSelect }: { audits: AuditRun[]; selectedAuditId: string | null; onSelect: (id: string) => void }) {
  return (
    <section className="card stack">
      <h2>Previous analyses</h2>
      <div className="chips">
        {audits.map(audit => (
          <button key={audit.id} className={audit.id === selectedAuditId ? 'chip active' : 'chip'} onClick={() => onSelect(audit.id)}>
            {audit.id.slice(0, 8)} · {audit.score}/100
          </button>
        ))}
        {audits.length === 0 && <p className="muted">No analyses for this project yet.</p>}
      </div>
    </section>
  )
}

function Overview({ audit }: { audit: AuditRun }) {
  return (
    <section className="card overview">
      <div className="score">
        <span>{audit.score}</span>
        <small>/100</small>
      </div>
      <div>
        <h2>Technical score</h2>
        <p>{audit.summary}</p>
        <div className="chips">
          {Object.keys(audit.stack).map(key => <span className="tag" key={key}>{key}</span>)}
        </div>
      </div>
    </section>
  )
}

function ScorePanel({ items }: { items: ScoreItem[] }) {
  return (
    <section className="card stack">
      <h2>Score reasons</h2>
      <div className="grid">
        {items.map(item => (
          <article key={item.id} className="mini-card">
            <div className="row">
              <strong>{item.category}</strong>
              <span>{item.awarded_points}/{item.max_points}</span>
            </div>
            <p>{item.reason}</p>
          </article>
        ))}
      </div>
    </section>
  )
}

function FindingsPanel({ findings }: { findings: Finding[] }) {
  return (
    <section className="card stack">
      <h2>Findings</h2>
      {findings.length === 0 && <p className="muted">No findings for this audit.</p>}
      {findings.map(finding => (
        <article className="finding" key={finding.id}>
          <div className="chips">
            <span className={`severity ${finding.severity.toLowerCase()}`}>{finding.severity}</span>
            <span className="tag">{finding.priority}</span>
            <span className="tag">{finding.state}</span>
            <span className="tag">{finding.rule_id}</span>
          </div>
          <h3>{finding.title}</h3>
          <p>{finding.description}</p>
          {finding.file_path && <p className="mono">{finding.file_path}{finding.line ? `:${finding.line}` : ''}</p>}
          <p className="muted">{finding.details.minimum_recommendation}</p>
        </article>
      ))}
    </section>
  )
}

function EvidencePanel({ evidence }: { evidence: Evidence[] }) {
  return (
    <section className="card stack">
      <h2>Evidence</h2>
      {evidence.length === 0 && <p className="muted">No evidence recorded.</p>}
      {evidence.map(item => (
        <article className="mini-card" key={item.id}>
          <strong>{item.kind}</strong>
          <p>{item.summary}</p>
          {(item.file_path || item.command) && <p className="mono">{[item.file_path, item.line, item.command].filter(Boolean).join(' · ')}</p>}
        </article>
      ))}
    </section>
  )
}

function DocsPanel({ docs }: { docs: GeneratedDoc[] }) {
  const doc = docs[0]
  return (
    <section className="card stack">
      <h2>Generated docs</h2>
      {doc ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{doc.content}</ReactMarkdown> : <p className="muted">No generated docs yet.</p>}
    </section>
  )
}

function TestsPanel({ tests }: { tests: TestResult[] }) {
  return (
    <section className="card stack">
      <h2>Tests</h2>
      {tests.length === 0 && <p className="muted">No test results recorded.</p>}
      {tests.map(test => (
        <article className="mini-card" key={test.id}>
          <div className="row">
            <strong>{test.name}</strong>
            <span className={`status ${test.status}`}>{test.status}</span>
          </div>
          <p className="mono">{test.command || 'no command'}</p>
          {test.output && <pre>{test.output}</pre>}
        </article>
      ))}
    </section>
  )
}

function ErrorText({ error }: { error: unknown }) {
  return <p className="error">{error instanceof Error ? error.message : String(error)}</p>
}
