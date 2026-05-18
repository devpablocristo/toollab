import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import Spinner from '../components/Spinner'
import { api } from '../lib/api'
import type {
  AuditConfig,
  AuditRun,
  CreatePRReviewRequest,
  CreateTaskSpecRequest,
  Evidence,
  Finding,
  GeneratedDoc,
  PRReview,
  PRReviewFile,
  PRReviewFinding,
  PRReviewResult,
  Project,
  ScoreItem,
  TaskSpec,
  TestResult,
} from '../lib/types'

const defaultConfig: AuditConfig = {
  generate_tests: true,
  run_existing_tests: true,
  allow_docs_read: false,
  allow_dependency_install: false,
}

const defaultSpecForm: CreateTaskSpecRequest = {
  module: '',
  title: '',
  task_description: '',
  spec_md: '',
  spec_status: 'provided',
}

const defaultReviewForm: CreatePRReviewRequest = {
  task_spec_id: '',
  title: '',
  description: '',
  diff_text: '',
  project_rules: '',
  test_output: '',
}

export default function ProjectAuditor() {
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState<'audit' | 'guard'>('audit')
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [selectedAuditId, setSelectedAuditId] = useState<string | null>(null)
  const [selectedReviewId, setSelectedReviewId] = useState<string | null>(null)
  const [name, setName] = useState('toollab')
  const [sourcePath, setSourcePath] = useState('/home/pablocristo/Proyectos/pablo/toollab')
  const [config, setConfig] = useState<AuditConfig>(defaultConfig)
  const [moduleFilter, setModuleFilter] = useState('')
  const [specForm, setSpecForm] = useState<CreateTaskSpecRequest>(defaultSpecForm)
  const [reviewForm, setReviewForm] = useState<CreatePRReviewRequest>(defaultReviewForm)

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

  const { data: specs } = useQuery({
    queryKey: ['project-specs', selectedProject?.id, moduleFilter],
    queryFn: () => api.projects.specs(selectedProject!.id, moduleFilter),
    enabled: !!selectedProject,
  })
  const { data: prReviews } = useQuery({
    queryKey: ['project-pr-reviews', selectedProject?.id],
    queryFn: () => api.projects.prReviews(selectedProject!.id),
    enabled: !!selectedProject,
  })
  const selectedReview = selectedReviewId ?? prReviews?.[0]?.id ?? null
  const { data: prReviewResult } = useQuery({
    queryKey: ['pr-review', selectedReview],
    queryFn: () => api.prReviews.get(selectedReview!),
    enabled: !!selectedReview,
  })

  const createProject = useMutation({
    mutationFn: () => api.projects.create({ name, source_path: sourcePath }),
    onSuccess: project => {
      setSelectedProjectId(project.id)
      setSelectedAuditId(null)
      setSelectedReviewId(null)
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

  const createSpec = useMutation({
    mutationFn: (project: Project) => api.projects.createSpec(project.id, specForm),
    onSuccess: spec => {
      setSpecForm(defaultSpecForm)
      setReviewForm(current => ({ ...current, task_spec_id: spec.id }))
      queryClient.invalidateQueries({ queryKey: ['project-specs', spec.project_id] })
    },
  })

  const createPRReview = useMutation({
    mutationFn: (project: Project) => api.projects.createPRReview(project.id, reviewForm),
    onSuccess: result => {
      setSelectedReviewId(result.review.id)
      queryClient.invalidateQueries({ queryKey: ['project-pr-reviews', result.review.project_id] })
      queryClient.invalidateQueries({ queryKey: ['pr-review', result.review.id] })
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

      <div className="tabs">
        <button className={activeTab === 'audit' ? 'tab active' : 'tab'} onClick={() => setActiveTab('audit')}>Project Audit</button>
        <button className={activeTab === 'guard' ? 'tab active' : 'tab'} onClick={() => setActiveTab('guard')}>PR Guard</button>
      </div>

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
                  <button key={project.id} className={project.id === selectedProject?.id ? 'list-item active' : 'list-item'} onClick={() => { setSelectedProjectId(project.id); setSelectedAuditId(null); setSelectedReviewId(null) }}>
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
          {activeTab === 'audit' ? (
            <ProjectAuditView
              selectedProject={selectedProject}
              createAudit={createAudit}
              orderedAudits={orderedAudits}
              selectedAudit={selectedAudit}
              onSelectAudit={setSelectedAuditId}
              audit={audit}
              findings={findings ?? []}
              evidence={evidence ?? []}
              docs={docs ?? []}
              tests={tests ?? []}
              scoreItems={scoreItems ?? []}
            />
          ) : (
            <PRGuardView
              selectedProject={selectedProject}
              specs={specs ?? []}
              moduleFilter={moduleFilter}
              setModuleFilter={setModuleFilter}
              specForm={specForm}
              setSpecForm={setSpecForm}
              createSpec={createSpec}
              reviewForm={reviewForm}
              setReviewForm={setReviewForm}
              createPRReview={createPRReview}
              prReviews={prReviews ?? []}
              selectedReviewId={selectedReview}
              onSelectReview={setSelectedReviewId}
              result={prReviewResult}
            />
          )}
        </section>
      </main>
    </div>
  )
}

function ProjectAuditView({
  selectedProject,
  createAudit,
  orderedAudits,
  selectedAudit,
  onSelectAudit,
  audit,
  findings,
  evidence,
  docs,
  tests,
  scoreItems,
}: {
  selectedProject: Project | null
  createAudit: ReturnType<typeof useMutation<unknown, Error, Project>>
  orderedAudits: AuditRun[]
  selectedAudit: string | null
  onSelectAudit: (id: string) => void
  audit?: AuditRun
  findings: Finding[]
  evidence: Evidence[]
  docs: GeneratedDoc[]
  tests: TestResult[]
  scoreItems: ScoreItem[]
}) {
  return selectedProject ? (
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
      <AuditPicker audits={orderedAudits} selectedAuditId={selectedAudit} onSelect={onSelectAudit} />
      {audit ? (
        <section className="results">
          <h2>3. Results</h2>
          <Overview audit={audit} />
          <ScorePanel items={scoreItems} />
          <FindingsPanel findings={findings} />
          <EvidencePanel evidence={evidence} />
          <DocsPanel docs={docs} />
          <TestsPanel tests={tests} />
        </section>
      ) : (
        <section className="card empty-state">Start an analysis to see results.</section>
      )}
    </>
  ) : (
    <section className="card empty-state">Load or select a project to start.</section>
  )
}

function PRGuardView({
  selectedProject,
  specs,
  moduleFilter,
  setModuleFilter,
  specForm,
  setSpecForm,
  createSpec,
  reviewForm,
  setReviewForm,
  createPRReview,
  prReviews,
  selectedReviewId,
  onSelectReview,
  result,
}: {
  selectedProject: Project | null
  specs: TaskSpec[]
  moduleFilter: string
  setModuleFilter: (value: string) => void
  specForm: CreateTaskSpecRequest
  setSpecForm: (value: CreateTaskSpecRequest) => void
  createSpec: ReturnType<typeof useMutation<unknown, Error, Project>>
  reviewForm: CreatePRReviewRequest
  setReviewForm: (value: CreatePRReviewRequest) => void
  createPRReview: ReturnType<typeof useMutation<unknown, Error, Project>>
  prReviews: PRReview[]
  selectedReviewId: string | null
  onSelectReview: (id: string) => void
  result?: PRReviewResult
}) {
  if (!selectedProject) {
    return <section className="card empty-state">Load or select a project to start.</section>
  }

  return (
    <>
      <section className="card project-header">
        <div>
          <p className="eyebrow">PR Guard project</p>
          <h2>{selectedProject.name}</h2>
          <p className="muted">{selectedProject.source_path}</p>
        </div>
      </section>

      <section className="guard-layout">
        <section className="card stack">
          <h2>SDD specs</h2>
          <div className="grid">
            <input value={specForm.module} onChange={event => setSpecForm({ ...specForm, module: event.target.value })} placeholder="Module" />
            <select value={specForm.spec_status} onChange={event => setSpecForm({ ...specForm, spec_status: event.target.value })}>
              <option value="provided">provided</option>
              <option value="draft">draft</option>
              <option value="inferred">inferred</option>
            </select>
          </div>
          <input value={specForm.title} onChange={event => setSpecForm({ ...specForm, title: event.target.value })} placeholder="Spec title" />
          <textarea value={specForm.task_description} onChange={event => setSpecForm({ ...specForm, task_description: event.target.value })} placeholder="Task description" rows={3} />
          <textarea value={specForm.spec_md} onChange={event => setSpecForm({ ...specForm, spec_md: event.target.value })} placeholder="Mini-spec SDD markdown" rows={8} />
          <button disabled={!specForm.title || !specForm.spec_md || createSpec.isPending} onClick={() => createSpec.mutate(selectedProject)}>
            {createSpec.isPending ? 'Saving...' : 'Save spec'}
          </button>
          {createSpec.error && <ErrorText error={createSpec.error} />}
          <div className="options">
            <p>Specs list</p>
            <input value={moduleFilter} onChange={event => setModuleFilter(event.target.value)} placeholder="Filter by module" />
            <div className="list">
              {specs.map(spec => (
                <button key={spec.id} className={reviewForm.task_spec_id === spec.id ? 'list-item active' : 'list-item'} onClick={() => setReviewForm({ ...reviewForm, task_spec_id: spec.id })}>
                  <strong>{spec.title}</strong>
                  <span>{[spec.module, spec.spec_status].filter(Boolean).join(' · ')}</span>
                </button>
              ))}
              {specs.length === 0 && <p className="muted">No specs stored for this project.</p>}
            </div>
          </div>
        </section>

        <section className="card stack">
          <h2>PR / diff review</h2>
          <select value={reviewForm.task_spec_id} onChange={event => setReviewForm({ ...reviewForm, task_spec_id: event.target.value })}>
            <option value="">No linked spec</option>
            {specs.map(spec => <option key={spec.id} value={spec.id}>{spec.title}</option>)}
          </select>
          <input value={reviewForm.title} onChange={event => setReviewForm({ ...reviewForm, title: event.target.value })} placeholder="PR title" />
          <textarea value={reviewForm.description} onChange={event => setReviewForm({ ...reviewForm, description: event.target.value })} placeholder="Description" rows={3} />
          <textarea className="diff-input" value={reviewForm.diff_text} onChange={event => setReviewForm({ ...reviewForm, diff_text: event.target.value })} placeholder="Paste git diff" rows={12} />
          <textarea value={reviewForm.project_rules} onChange={event => setReviewForm({ ...reviewForm, project_rules: event.target.value })} placeholder="Project rules" rows={4} />
          <textarea value={reviewForm.test_output} onChange={event => setReviewForm({ ...reviewForm, test_output: event.target.value })} placeholder="Test output" rows={4} />
          <button disabled={!reviewForm.title || !reviewForm.diff_text || createPRReview.isPending} onClick={() => createPRReview.mutate(selectedProject)}>
            {createPRReview.isPending ? 'Analyzing...' : 'Analyze PR / Diff'}
          </button>
          {createPRReview.error && <ErrorText error={createPRReview.error} />}
          <div className="options">
            <p>Previous PR reviews</p>
            <div className="chips">
              {prReviews.map(review => (
                <button key={review.id} className={review.id === selectedReviewId ? 'chip active' : 'chip'} onClick={() => onSelectReview(review.id)}>
                  {review.title} · {review.score}/100 · {review.decision}
                </button>
              ))}
              {prReviews.length === 0 && <p className="muted">No PR reviews yet.</p>}
            </div>
          </div>
        </section>
      </section>

      {result ? <PRReviewResultPanel result={result} /> : <section className="card empty-state">Analyze a diff to see PR Guard results.</section>}
    </>
  )
}

function PRReviewResultPanel({ result }: { result: PRReviewResult }) {
  return (
    <section className="results">
      <section className="card overview">
        <div className="score">
          <span>{result.review.score}</span>
          <small>/100</small>
        </div>
        <div>
          <h2>{result.review.decision}</h2>
          <p>{result.review.summary}</p>
          <div className="chips">
            <span className="tag">{result.review.confidence}</span>
            <span className="tag">{result.review.spec_status}</span>
          </div>
        </div>
      </section>
      <ChangedFilesPanel files={result.files} />
      <PRFindingsPanel findings={result.findings} />
      <PromptPanel title="Review prompt" value={result.review.review_prompt} />
    </section>
  )
}

function ChangedFilesPanel({ files }: { files: PRReviewFile[] }) {
  return (
    <section className="card stack">
      <h2>Changed files</h2>
      <div className="grid">
        {files.map(file => (
          <article key={file.id} className="mini-card">
            <div className="row">
              <strong>{file.path}</strong>
              <span className={`severity ${file.risk_level.toLowerCase()}`}>{file.risk_level}</span>
            </div>
            <p className="muted">{file.change_type} · {file.risk_area} · +{file.additions} / -{file.deletions}</p>
          </article>
        ))}
      </div>
    </section>
  )
}

function PRFindingsPanel({ findings }: { findings: PRReviewFinding[] }) {
  return (
    <section className="card stack">
      <h2>PR findings</h2>
      {findings.length === 0 && <p className="muted">No PR Guard findings.</p>}
      {findings.map(finding => (
        <article className="finding" key={finding.id}>
          <div className="chips">
            <span className={`severity ${finding.severity.toLowerCase()}`}>{finding.severity}</span>
            <span className="tag">{finding.status}</span>
            <span className="tag">{finding.code}</span>
          </div>
          <h3>{finding.title}</h3>
          <p>{finding.problem}</p>
          {finding.files.length > 0 && <p className="mono">{finding.files.join(' · ')}</p>}
          <p className="muted">{finding.evidence}</p>
          <p>{finding.suggested_fix}</p>
          <PromptPanel title="AI correction prompt" value={finding.ai_correction_prompt} compact />
        </article>
      ))}
    </section>
  )
}

function PromptPanel({ title, value, compact }: { title: string; value: string; compact?: boolean }) {
  return (
    <section className={compact ? 'prompt-box compact' : 'card stack'}>
      <div className="row">
        <h2>{title}</h2>
        <button className="secondary" onClick={() => navigator.clipboard?.writeText(value)}>Copy</button>
      </div>
      <textarea readOnly value={value} rows={compact ? 7 : 16} />
    </section>
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
