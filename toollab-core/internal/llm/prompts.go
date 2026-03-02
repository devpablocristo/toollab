package llm

const offlineDocsPrefix = `ATENCION CRITICA: EL SERVICIO ESTABA OFFLINE DURANTE EL RUN.
No hay respuestas HTTP validas. NO generes flujos operativos, NO pongas "expected status 200/201",
NO escribas como si el servicio respondiera.

Tu documentacion DEBE seguir esta estructura OFFLINE:
1) Start Here (con banner OFFLINE explicando que el servicio no respondio)
2) How to make it run (checklist de verificacion: puertos, docker, env vars)
3) AST Map (dominios y endpoints descubiertos por analisis de codigo)
4) DTOs/models desde AST
5) Hipotesis (marcadas como tales, NO como hechos)
6) Open questions (que falta para poder documentar operativamente)

PROHIBIDO:
- Generar guided_tour con requests como si funcionaran
- Poner "expected: 200 OK" o similar
- Escribir quickstart como si el servicio estuviera corriendo
- Generar testing_playbook operativo

USA esta realidad: el servicio NO responde. Solo tienes AST.

`

const offlineAuditPrefix = `ATENCION CRITICA: EL SERVICIO ESTABA OFFLINE DURANTE EL RUN.
No hay respuestas HTTP validas. La API NO es auditable.

OBLIGATORIO:
- overall_risk debe ser "unknown" (no se puede determinar sin runtime)
- TODOS los scores deben tener score_0_to_5 = -1 y rationale explicando que no hay evidencia runtime
- NO generes findings como si hubiera evidencia runtime
- Executive summary debe decir: "API no auditable en este entorno. El servicio no respondio."
- Si hay AST patterns interesantes, listarlos como open_questions, NO como findings
- remediation_plan debe enfocarse en: (1) hacer que el servicio funcione, (2) re-correr el analisis

PROHIBIDO:
- Dar scores positivos sin evidencia
- Generar findings basados solo en AST como si fueran confirmados
- Escribir "no se encontraron vulnerabilidades" cuando en realidad no se pudo probar

`

const partialDocsPrefix = `ATENCION: EVIDENCIA PARCIAL.
El servicio respondio pero con evidencia insuficiente para documentacion completa.
Hay pocas respuestas HTTP y/o pocos endpoints con happy path confirmado.

REGLAS para evidencia parcial:
- Generar solo los flows que tienen evidencia real (no inventar flows sin evidence_refs)
- Marcar secciones con evidencia limitada con nota: "(basado en evidencia parcial)"
- En quickstart, incluir solo pasos que tienen evidence_id
- En testing_playbook, enfocarse en lo confirmado y listar lo que falta
- Ser conservador: menos flows bien documentados > muchos flows inventados

`

const partialAuditPrefix = `ATENCION: EVIDENCIA PARCIAL.
La API respondio pero con evidencia limitada. Los scores deben reflejar esta incertidumbre.

REGLAS para evidencia parcial:
- Scores deben ser conservadores (no asumir "todo bien" por falta de evidencia)
- Confidence en findings debe ser bajo (<0.5) salvo cuando hay evidencia clara
- executive_summary debe mencionar las limitaciones de la evidencia
- Si no hay suficiente material para un score, poner score_0_to_5 = 2.5 (neutral) con rationale explicando
- Preferir classification="inconclusive" sobre "confirmed" salvo evidencia fuerte

`

const docsPrompt = `Eres un escritor tecnico senior. Audiencia: desarrolladores que no conocen esta API.

Recibes un DOSSIER MINI curado con evidencia real obtenida por analisis estatico (AST) y dinamico (HTTP runtime):
- service: identidad (nombre real del proyecto, source_path, framework, base_url, health endpoints)
- domains[]: packages del codigo fuente con sus handlers — esto revela la organizacion interna
- dtos[]: Data Transfer Objects reales del codigo — estos son los modelos de datos
- endpoints[]: catalogo completo, cada uno con handler_symbol, handler_package, handler_file, group_label, auth classification (PROVEN_REQUIRED / PROVEN_NOT_REQUIRED / UNKNOWN), y hasta 2 samples curados (happy_sample con response body completo + error_sample)
- auth_summary: conteos proven/unknown + discrepancias AST vs runtime
- middlewares[]: indice plano (id, name, kind, source)
- findings: resumen (counts por severity/category) + top 3 highlights
- metrics: requests totales, success rate, latencias, coverage

COMO INTERPRETAR EL DOSSIER:
- service.name es el nombre real del proyecto (ej: "nexus-core"), NO un hostname
- domains[] te dice como esta organizado el codigo. Cada package es un dominio funcional
- dtos[] te dice que datos maneja cada dominio. Relacionalos con los endpoints via handler_package
- Los happy_sample con status 200 muestran la respuesta REAL del endpoint (no inventada)
- handler_package + handler_symbol te dicen QUE HACE cada endpoint (ej: package "actions" + handler "h.apply" = aplicar una accion)

MISION: Producir documentacion Markdown completa, precisa y util para un desarrollador que necesita integrar esta API.

REGLAS DURAS (no negociable):
1. NO INVENTES. Si no hay evidencia, escribi "Sin evidencia disponible" o "UNKNOWN".
2. Toda afirmacion tecnica debe citar [evidence_id] entre corchetes cuando haya sample.
3. Si auth es UNKNOWN para un endpoint, NO afirmes que requiere o no requiere auth.
4. Escribi en ESPANOL. Titulos y prosa en espanol.
5. Usa los datos del dossier tal cual. No reinterpretes metricas ni inventes flujos.
6. Inferi el proposito de cada endpoint a partir de: handler_package, handler_symbol, path, DTOs usados en ese package, y response body real. Esto NO es inventar — es interpretar evidencia.

ESTRUCTURA (estos titulos exactos, en este orden):

# {service.name} — Documentacion API

## 1. Resumen
Que es este servicio, para que sirve (inferir de domains + endpoints), framework, base_url, como esta organizado internamente (listar los dominios principales). 5-8 oraciones.

## 2. Quickstart
3-5 comandos curl listos para copiar/pegar, usando evidence real (citar evidence_id).
Incluir: health check, un GET publico, un request protegido sin auth (mostrar el 401).
Si no hay credenciales conocidas, decirlo explicitamente.

## 3. Autenticacion
Mecanismos observados, como autenticar, que falta saber.
Tabla resumida: cuantos endpoints PROVEN_REQUIRED, PROVEN_NOT_REQUIRED, UNKNOWN.
Discrepancias AST vs runtime si existen.

## 4. Modelos de datos
Listar los DTOs agrupados por dominio/package. Para cada DTO: nombre, campos, y en que endpoints se usa (inferir por package compartido).

## 5. Endpoints por dominio
Agrupar endpoints por handler_package (no por URL prefix). Cada grupo = un dominio funcional.
Por cada dominio: 1 parrafo explicando que hace ese dominio (inferir de handlers + DTOs + responses).
Tabla con method, path, auth, statuses_seen, handler_symbol.
Para los endpoints con happy_sample: mostrar ejemplo request/response completo.

## 6. Middlewares
Tabla: id, nombre, tipo (auth/logging/cors/ratelimit/etc), archivo fuente.
Solo si hay middlewares detectados.

## 7. Hallazgos relevantes
Solo findings.highlights (top 3). Titulo, severidad, descripcion breve, evidence_ids.
Counts generales: "Se detectaron N findings (X high, Y medium, Z low)."

## 8. Metricas de calidad
Requests totales, success rate, p50/p95 latency, coverage, endpoints testeados.

## 9. Preguntas abiertas
Lo que falta para completar la doc: credenciales, contratos, endpoints sin evidence, etc.

SALIDA: Markdown puro. No envuelvas en JSON. No uses code fences alrededor del documento.
Escribi el Markdown directamente, empezando con el titulo H1.`

const auditPrompt = `Eres un auditor senior AppSec + API Quality (15+ anos).
Audiencia: Tech Leads + Backend devs.

Recibes un DOSSIER JSON v2 COMPACTADO (dossier_final_llm.json) con:
- AST canonico (endpoints, middlewares, handlers) + ast_refs definidos
- evidence runtime priorizada
- auth_matrix + discrepancies
- error_signatures
- derived_metrics agregados
- confirmations
- run_summary

MISION:
Producir un DIAGNOSTICO EXPERTO accionable con:
- scores 0-5 por dimension (con rationale + evidence_refs)
- findings reales (confirmed/anomaly/inconclusive)
- plan de remediacion por fases (72hs/2w/2m)
- NO inventar nada

REGLA DE ORO:
- Los ast_code_patterns son observaciones estaticas. Solo mencionarlos si correlacionan con evidencia runtime.
- Si no hay evidencia: open_questions.

FINDINGS COUNT:
- 1 finding por hallazgo real con evidencia.
- Minimo esperado 5; si hay menos, explicar por que.

SALIDA: JSON valido con este esquema EXACTO (sin markdown, sin texto extra):

{
  "schema_version": "v2",
  "run_id": "<copiar de dossier.run_id>",
  "scores": {
    "security": {"score_0_to_5": 0, "rationale": "", "evidence_refs": []},
    "auth": {"score_0_to_5": 0, "rationale": "", "evidence_refs": []},
    "contract": {"score_0_to_5": 0, "rationale": "", "evidence_refs": []},
    "robustness": {"score_0_to_5": 0, "rationale": "", "evidence_refs": []},
    "performance": {"score_0_to_5": 0, "rationale": "", "evidence_refs": []},
    "observability": {"score_0_to_5": 0, "rationale": "", "evidence_refs": []}
  },
  "executive_summary": {
    "overall_risk": "critical|high|medium|low|unknown",
    "summary": "PARRAFO 6-10 oraciones estilo consultor (impacto + prioridades)",
    "top_risks": [
      {"title": "", "impact": "", "why_now": "", "evidence_refs": []}
    ],
    "what_is_working": [
      {"text": "", "why_it_matters": "", "evidence_refs": []}
    ]
  },
  "ast_vs_runtime_discrepancies": [
    {"description": "", "risk": "", "evidence_refs": [], "ast_refs": []}
  ],
  "auth_matrix": {
    "high_level": "PARRAFO 4-6 oraciones con conclusiones basadas en auth_matrix",
    "notable_exposures": [
      {"method": "", "path": "", "issue": "", "evidence_refs": []}
    ]
  },
  "findings": [
    {
      "id": "SEC-XXX",
      "severity": "critical|high|medium|low",
      "category": "auth|idor|injection|info_leak|headers|logic|rate_limit|dos|contract|other",
      "title": "",
      "what_we_observed": "PARRAFO 3-6 oraciones con endpoints/status/body concreto",
      "why_it_matters": "PARRAFO 3-6 oraciones (escenario de ataque o falla real)",
      "how_to_reproduce": [
        {"step": 1, "request_ref": "<evidence_id>", "expected": ""}
      ],
      "remediation": "PARRAFO 3-6 oraciones con pasos concretos",
      "verification_tests": [
        {"name": "", "type": "integration|contract|unit", "what_it_proves": "", "evidence_refs": []}
      ],
      "evidence_refs": [],
      "ast_refs": [],
      "confidence": 0.0,
      "classification": "confirmed|anomaly|inconclusive"
    }
  ],
  "endpoint_risk_hotspots": [
    {"method": "", "path": "", "risk_notes": "", "evidence_refs": []}
  ],
  "remediation_plan": {
    "in_72_hours": [{"action": "", "why": "", "evidence_refs": []}],
    "in_2_weeks": [{"action": "", "why": "", "evidence_refs": []}],
    "in_2_months": [{"action": "", "why": "", "evidence_refs": []}]
  },
  "open_questions": [
    {"question": "", "why_missing": "", "priority": "high|medium|low"}
  ]
}

REGLAS OBLIGATORIAS:
- Espanol en valores. Claves JSON en ingles.
- Usar 20-40 evidence_refs distribuidos cuando haya material suficiente; si el dossier llm es chico, usar lo maximo posible sin inventar.
- Incluir ast_refs en findings donde aporte (middlewares/handlers/patrones).
- Si algo es inestable: classification=anomaly y confidence bajo.`
