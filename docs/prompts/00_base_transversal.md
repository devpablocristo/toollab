# 00. Base Transversal

## Objetivo

Definir las invariantes obligatorias de `toollab` para que cualquier mejora futura preserve el carácter del producto: laboratorio de análisis basado en evidencia, no generador de texto libre.

## Alcance obligatorio

- todo output debe estar anclado a `AST`, `runtime evidence` o artifacts derivados
- el `run_mode` gobierna qué tan fuerte puede ser una afirmación
- los prompts, docs y UI deben reflejar el producto real, no roadmap implícito
- `toollab-core` y `toollab-ui` forman un solo producto; no se documentan como herramientas separadas

## Convención de idioma

- prosa explicativa en español
- términos técnicos, nombres de artifacts, endpoints, headers, JSON keys, tipos y símbolos en English

## Invariantes de ingeniería

### E1. Evidence-first

- ningún documento o auditoría puede inventar contratos, auth o response shapes
- si no hay evidencia suficiente, el output debe degradar a `open_questions`, `gaps` o `inconclusive`

### E2. Run mode explícito

- `offline`: no runtime útil; solo AST + preflight
- `online_partial`: evidencia limitada; se exige lenguaje conservador
- `online_good` y `online_strong`: mejor cobertura, sin perder trazabilidad

### E3. Artifacts como contrato interno

- cada etapa produce artifacts tipados y reutilizables
- los artifacts son la interfaz entre pipeline, exports, UI y runtime LLM

### E4. Determinismo antes que amplitud

- el pipeline prioriza reproducibilidad, budgets y límites antes que exploración agresiva
- cualquier paso nuevo debe definir costo, categoría de budget y criterio de corte

### E5. Seguridad operacional

- masking de credenciales y cookies
- SSRF controlado
- límites de body, redirects y timeout
- evidencias manuales del playground también deben quedar trazadas

### E6. Externalización de prompts críticos

- los prompts de runtime LLM deben vivir en archivos versionables
- el código solo compone prefijos, sufijos y dossier, pero no debe esconder el texto base en constantes enormes

### E7. Documentación breve, clara y profesional

- describir capacidades reales
- diferenciar claramente producto, runtime actual y roadmap
- evitar lenguaje marketinero vacío

## Criterios de éxito

- cualquier persona puede explicar qué hace ToolLab leyendo `README.md` y `docs/DOC.md`
- la suite de prompts cubre el producto real de punta a punta
- los prompts LLM son trazables y editables sin tocar lógica de negocio

## Orden de ejecución recomendado

1. fijar invariantes transversales
2. describir producto y arquitectura
3. describir pipeline y artifacts
4. documentar runtime LLM
5. documentar surfaces operativas: playground y UI
6. cerrar con testing y validación
