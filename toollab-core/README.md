# toollab-core

Go backend for the ToolLab AI Project Auditor.

## Run

```bash
mkdir -p data
CGO_ENABLED=1 go run ./cmd/api
```

## API

- `GET /healthz`
- `GET /api/projects`
- `POST /api/projects`
- `GET /api/projects/{project_id}/audits`
- `POST /api/projects/{project_id}/audits`
- `GET /api/audits/{audit_id}`
- `GET /api/audits/{audit_id}/findings`
- `GET /api/audits/{audit_id}/evidence`
- `GET /api/audits/{audit_id}/docs`
- `GET /api/audits/{audit_id}/tests`
- `GET /api/audits/{audit_id}/score`

## Validate

```bash
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go build -o /tmp/api ./cmd/api
```
