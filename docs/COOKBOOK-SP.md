## Receta rápida de Toollab

### 1. Generar un escenario
```bash
cd toollab-core

# Desde un adaptador Toollab (recomendado)
go run ./cmd/toollab generate --from toollab --target-base-url http://localhost:8080

# Desde un archivo OpenAPI
go run ./cmd/toollab generate --from openapi --openapi-file api.yaml --base-url http://localhost:8080
```
Resultado: `scenarios/scenario.yaml`

### 2. Enriquecer (opcional, mejora el escenario)
```bash
go run ./cmd/toollab enrich scenarios/scenario.yaml --from toollab --target-base-url http://localhost:8080
```
Resultado: `scenarios/enriched.yaml` (resuelve path params, agrega bodies, etc.)

### 3. Ejecutar
```bash
go run ./cmd/toollab run scenarios/enriched.yaml
```
Resultado: `golden_runs/<hash>/` con `evidence.json`, `report.json`, `decision_tape.json`

### 4. Interpretar con LLM
```bash
ollama serve  # si no está corriendo
go run ./cmd/toollab interpret golden_runs/<hash>
```
Te da una narrativa en lenguaje natural de qué pasó.

### 5. Comparar dos runs (opcional)
```bash
go run ./cmd/toollab diff golden_runs/<hashA> golden_runs/<hashB> --print
```

---

**Secretos:** ponelos en `toollab-core/.env` y se cargan solos.

**Reproducibilidad:** mismo escenario + mismo seed = mismo resultado. Siempre.