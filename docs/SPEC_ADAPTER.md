# Toolab Adapter Spec v1

A **Toolab Adapter** is a set of HTTP endpoints that an application exposes to enable precise, controlled, and reproducible testing with Toolab (or any compatible tool).

Applications that implement this contract are called **toolab-ready**.

## Design Principles

1. **Capability-based**: Apps implement only what they can. Toolab adapts.
2. **Zero coupling**: The adapter is a sidecar concern. No toolab dependency in production code.
3. **Convention over configuration**: Fixed paths, fixed formats, predictable behavior.
4. **Safe by default**: All adapter endpoints require explicit opt-in. Nothing is exposed unless mounted.

## Endpoint Prefix

All adapter endpoints live under:

```
/_toolab/
```

This prefix is reserved. Apps MUST NOT use it for other purposes.

## 1. Manifest (required)

```
GET /_toolab/manifest
```

The only **required** endpoint. Returns what capabilities the app supports.

### Response (200)

```json
{
  "adapter_version": "1",
  "app_name": "nexus-core",
  "app_version": "1.1.0",
  "capabilities": [
    "state.fingerprint",
    "state.snapshot",
    "state.restore",
    "state.reset",
    "seed",
    "metrics",
    "traces",
    "logs"
  ]
}
```

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `adapter_version` | string | yes | Always `"1"` for this spec |
| `app_name` | string | yes | Application identifier |
| `app_version` | string | yes | Application version |
| `capabilities` | string[] | yes | List of implemented capabilities |

### Valid Capabilities

| Capability | Endpoints | Purpose |
|---|---|---|
| `state.fingerprint` | `GET /_toolab/state/fingerprint` | Hash of current data state |
| `state.snapshot` | `POST /_toolab/state/snapshot` | Save current state |
| `state.restore` | `POST /_toolab/state/restore` | Restore a saved state |
| `state.reset` | `POST /_toolab/state/reset` | Reset to initial/seed state |
| `seed` | `POST /_toolab/seed` | Propagate deterministic seed |
| `metrics` | `GET /_toolab/metrics` | Structured metrics snapshot |
| `traces` | `GET /_toolab/traces` | Traces from last window |
| `logs` | `GET /_toolab/logs` | Structured log lines |

Toolab MUST check the manifest before calling any capability endpoint. If a capability is not listed, toolab MUST NOT call it.

## 2. State Management

### 2.1 State Fingerprint

```
GET /_toolab/state/fingerprint
```

Returns a deterministic hash of the current application data state. Two calls against the same data MUST return the same fingerprint.

**Response (200)**:

```json
{
  "fingerprint": "sha256:a1b2c3d4e5f6...",
  "scope": "full",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

| Field | Type | Description |
|---|---|---|
| `fingerprint` | string | `algo:hex` format. Recommended: `sha256:...` |
| `scope` | string | `"full"` = entire DB, `"tables"` = selected tables only |
| `timestamp` | string | ISO 8601 UTC |

**How it's computed**: Implementation-defined. Common approach: hash of row counts + checksums of key tables. MUST be deterministic (same data = same hash).

### 2.2 State Snapshot

```
POST /_toolab/state/snapshot
```

Captures the current data state for later restoration.

**Request body (optional)**:

```json
{
  "label": "before_chaos_test"
}
```

**Response (201)**:

```json
{
  "snapshot_id": "snap_20250115_103000",
  "fingerprint": "sha256:a1b2c3d4e5f6...",
  "label": "before_chaos_test",
  "created_at": "2025-01-15T10:30:00Z"
}
```

**Implementation**: Can be a DB dump, a named savepoint, a logical backup, or even a Docker volume snapshot. The adapter abstracts the mechanism.

### 2.3 State Restore

```
POST /_toolab/state/restore
```

Restores application state to a previous snapshot.

**Request body**:

```json
{
  "snapshot_id": "snap_20250115_103000"
}
```

**Response (200)**:

```json
{
  "restored": true,
  "snapshot_id": "snap_20250115_103000",
  "fingerprint": "sha256:a1b2c3d4e5f6..."
}
```

The `fingerprint` in the response MUST match the fingerprint from when the snapshot was taken.

### 2.4 State Reset

```
POST /_toolab/state/reset
```

Resets the application to its initial seed state (e.g., after `make seed`). Simpler than snapshot/restore — just "go back to clean."

**Response (200)**:

```json
{
  "reset": true,
  "fingerprint": "sha256:..."
}
```

## 3. Seed Propagation

```
POST /_toolab/seed
```

Tells the application to enter deterministic mode with a given seed. After this call, any internal randomness (UUIDs, timestamps, jitter) SHOULD use the provided seed for deterministic output.

**Request body**:

```json
{
  "run_seed": "42",
  "scope": ["uuid", "timestamp", "jitter"]
}
```

| Field | Type | Description |
|---|---|---|
| `run_seed` | string | Decimal seed string (same format as toolab seeds) |
| `scope` | string[] | What to make deterministic. App returns what it actually applied. |

**Response (200)**:

```json
{
  "applied": true,
  "run_seed": "42",
  "scope_applied": ["uuid"],
  "scope_ignored": ["timestamp", "jitter"]
}
```

The app reports honestly what it could make deterministic and what it couldn't. Toolab records this in the evidence bundle.

**To exit deterministic mode**:

```
DELETE /_toolab/seed
```

**Response (200)**:

```json
{
  "cleared": true
}
```

## 4. Observability

### 4.1 Metrics

```
GET /_toolab/metrics
```

Returns a structured metrics snapshot. Unlike Prometheus text format, this returns JSON with labeled metrics that toolab can diff between start/end of a run.

**Response (200)**:

```json
{
  "collected_at": "2025-01-15T10:30:00Z",
  "metrics": [
    {
      "name": "http_requests_total",
      "type": "counter",
      "value": 15234,
      "labels": { "method": "POST", "path": "/v1/run", "status": "200" }
    },
    {
      "name": "http_request_duration_ms",
      "type": "histogram",
      "value": { "p50": 12, "p95": 45, "p99": 120, "count": 15234, "sum": 234567 },
      "labels": { "method": "POST", "path": "/v1/run" }
    },
    {
      "name": "db_pool_active",
      "type": "gauge",
      "value": 3,
      "labels": {}
    }
  ]
}
```

| Metric Field | Type | Description |
|---|---|---|
| `name` | string | Metric name |
| `type` | string | `"counter"`, `"gauge"`, `"histogram"` |
| `value` | number or object | Scalar for counter/gauge, object for histogram |
| `labels` | object | Key-value string pairs |

### 4.2 Traces

```
GET /_toolab/traces?since=<iso8601>&limit=100
```

Returns traces collected since a given timestamp.

**Query parameters**:

| Param | Type | Default | Description |
|---|---|---|---|
| `since` | string | 5 minutes ago | ISO 8601 timestamp |
| `limit` | int | 100 | Max traces to return |

**Response (200)**:

```json
{
  "collected_at": "2025-01-15T10:30:05Z",
  "traces": [
    {
      "trace_id": "abc123...",
      "span_id": "def456...",
      "operation": "POST /v1/run",
      "duration_ms": 45,
      "status": "ok",
      "started_at": "2025-01-15T10:30:01Z",
      "attributes": {
        "tool_name": "send_email",
        "decision": "allow"
      }
    }
  ]
}
```

### 4.3 Logs

```
GET /_toolab/logs?since=<iso8601>&limit=500&level=WARN
```

Returns structured log lines.

**Query parameters**:

| Param | Type | Default | Description |
|---|---|---|---|
| `since` | string | 5 minutes ago | ISO 8601 timestamp |
| `limit` | int | 500 | Max lines |
| `level` | string | all | Minimum level: `DEBUG`, `INFO`, `WARN`, `ERROR` |

**Response (200)**:

```json
{
  "collected_at": "2025-01-15T10:30:05Z",
  "lines": [
    {
      "timestamp": "2025-01-15T10:30:01Z",
      "level": "INFO",
      "message": "policy evaluated",
      "attrs": {
        "tool_name": "send_email",
        "decision": "allow",
        "latency_ms": 12
      }
    }
  ]
}
```

This matches toolab's existing `LogLine` format (`timestamp`, `level`, `message`, `attrs`).

## 5. Authentication

Adapter endpoints MAY be protected. When protected, they MUST accept the same authentication mechanism as the main application.

Toolab uses the `target.headers` and `target.auth` from the scenario to authenticate against adapter endpoints.

## 6. Error Format

All adapter error responses MUST use:

```json
{
  "error": "snapshot_not_found",
  "message": "Snapshot snap_xyz does not exist"
}
```

**Standard error codes**:

| Code | HTTP Status | Meaning |
|---|---|---|
| `not_implemented` | 501 | Capability listed but not yet implemented |
| `snapshot_not_found` | 404 | Unknown snapshot ID |
| `seed_invalid` | 400 | Seed value is not a valid decimal string |
| `state_locked` | 409 | Another operation is in progress |
| `internal` | 500 | Unexpected error |

## 7. Toolab Integration

When toolab detects an adapter (via manifest), the run flow becomes:

```
1. GET  /_toolab/manifest              → discover capabilities
2. POST /_toolab/state/snapshot        → save pre-test state (if capable)
3. POST /_toolab/seed                  → propagate seed (if capable)
4. GET  /_toolab/state/fingerprint     → record pre-test fingerprint
5. GET  /_toolab/metrics               → baseline metrics (start)
6. --- toolab executes workload ---
7. GET  /_toolab/metrics               → post-test metrics (end)
8. GET  /_toolab/traces?since=<start>  → collect traces
9. GET  /_toolab/logs?since=<start>    → collect logs
10. GET /_toolab/state/fingerprint     → record post-test fingerprint
11. DELETE /_toolab/seed               → exit deterministic mode
12. --- toolab builds evidence ---
```

For reproduction:

```
1. POST /_toolab/state/restore         → restore to snapshot
2. POST /_toolab/seed                  → same seed
3. --- toolab re-executes ---
4. Compare fingerprints
```

## 8. SDK Contract

A Toolab Adapter SDK (e.g., `toolab-go-adapter`) provides:

```go
adapter := toolab.NewAdapter(toolab.Config{
    AppName:    "nexus-core",
    AppVersion: "1.1.0",
    DB:         db,                    // *sql.DB for state ops
    Logger:     logger,                // structured logger
    Metrics:    metricsCollector,      // interface for metrics snapshot
})

// Mount on your router (Gin example)
adapter.Register(router.Group("/_toolab"))
```

The SDK implements all endpoint handlers. The app only provides:
- A `*sql.DB` (for state fingerprint/snapshot/restore)
- A metrics collector (interface with `Snapshot() []Metric`)
- A structured logger (interface with `Lines(since, limit, level) []LogLine`)
- Optionally: a seed handler (interface with `ApplySeed(seed string, scope []string) ApplyResult`)

Everything else (manifest, routing, error handling, JSON serialization) is handled by the SDK.

## 9. Capability Levels

Apps can be toolab-ready at different levels:

| Level | Capabilities | What it enables |
|---|---|---|
| **L0** | manifest only | Toolab knows the app exists and its version |
| **L1** | + metrics, logs | Observability collection in evidence bundle |
| **L2** | + state.fingerprint | Can verify if state changed during test |
| **L3** | + state.snapshot, state.restore | Full reproducibility of data state |
| **L4** | + seed | End-to-end deterministic testing |

Most apps can reach L1 in minutes (just mount the SDK with a logger). L3-L4 require more integration effort but unlock full reproducibility.
