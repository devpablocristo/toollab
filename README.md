# ToolLab

ToolLab is an AI Project Auditor.

It audits projects created or modified with AI and helps decide whether the work is safe to continue, needs changes, or should not be accepted yet.

## MVP Scope

The first version focuses on one simple flow:

```text
Load project -> Start analysis -> Review results
```

ToolLab currently:

- loads a local project path;
- starts a deterministic project audit;
- inventories files, manifests, tests, CI, and migrations;
- creates findings with severity and priority;
- stores audit runs, findings, evidence, tests, docs, and score in SQLite;
- shows score, findings, evidence, docs, and tests in a simple UI.

## Out Of Scope For The MVP

- PR review.
- PR diff analysis.
- Advanced e2e generation.
- Markdown export.
- AI-assisted audit reasoning.

## Quick Start

```bash
make up
```

Local services:

- UI: [http://localhost:5173](http://localhost:5173)
- API: [http://localhost:8090](http://localhost:8090)

Development mode:

```bash
make install
make dev
```

## Validation

```bash
make test
make build
```
