# ToolLab UI

Web interface for the ToolLab v3 laboratory loop.

## Quickstart

```bash
# 1. Start the backend (from toollab-core/)
make run

# 2. Install UI dependencies
cd ui
npm install

# 3. Start dev server (proxies API to localhost:8090)
npm run dev
```

Open [http://localhost:5173](http://localhost:5173)

## Configuration

| Variable | Default | Description |
|---|---|---|
| `VITE_API_BASE_URL` | (empty, uses proxy) | Backend URL. In dev, Vite proxies `/api` to `localhost:8090`. |

## Lab Loop (6 steps)

1. **Create Target** — Set name, source path, and base URL
2. **Create Run** — Opens the Run Workspace
3. **Discover** — AST analysis generates `service_model` + `scenario_plan`
4. **Execute** — Runs scenarios against the target, captures `evidence_pack`
5. **Audit** — Deterministic rule engine produces `audit_report` with anchored findings
6. **Interpret** — LLM-bounded interpretation with facts, inferences, and guided tour

## Screens

- `/targets` — List and create targets
- `/targets/:id` — Target detail + runs list
- `/runs/:id` — **Run Workspace** with tabs:
  - **Overview** — Stepper showing loop progress + next step CTA
  - **Model** — Endpoint explorer with handler refs (file:line)
  - **Scenario** — Plan editor (table + JSON modes) with save
  - **Execute** — Run controls with tag/case filtering
  - **Evidence** — Evidence pack viewer with item detail + full bodies
  - **Audit** — Findings list with severity badges + cross-links to evidence/model
  - **Interpret** — Guided tour, facts, inferences, open questions, scenario suggestions

## Deep Links

Navigate across tabs with query params:

- `/runs/:id?tab=evidence&eid=...` — Open evidence item
- `/runs/:id?tab=audit&fid=...` — Open finding detail
- `/runs/:id?tab=model&ek=...` — Open endpoint

## Stack

React 18 + TypeScript + Vite 5 + TailwindCSS 3 + React Router 6 + TanStack Query 5
