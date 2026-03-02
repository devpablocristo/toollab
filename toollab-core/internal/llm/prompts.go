package llm

const offlineDocsPrefix = `CRITICAL: THE SERVICE WAS OFFLINE DURING THE RUN.
No valid HTTP responses. Only AST analysis is available.

USE the standard structure (## 1 through ## 7) with these adaptations:
- ## 1. Overview: mention the service was offline during analysis.
- ## 2. Key Concepts: infer from route paths and handler names only, mark everything as "INFERRED — no runtime verification".
- ## 3. Main Flows: describe expected CRUD patterns from routes, mark ALL as "NOT VERIFIED — service was offline".
- ## 4. Authentication: report only AST-inferred data. No runtime evidence available.
- ## 5. Common Errors: omit section entirely (no runtime data).
- ## 6. Security Findings: only if AST findings exist.
- ## 7. Open Questions: emphasize that the service must be online for meaningful documentation.

FORBIDDEN:
- Asserting endpoint behavior without runtime evidence
- Inventing response shapes or auth mechanisms

`

const offlineAuditPrefix = `CRITICAL: THE SERVICE WAS OFFLINE DURING THE RUN.
No valid HTTP responses. The API is NOT auditable.

MANDATORY:
- overall_risk must be "unknown" (cannot determine without runtime)
- ALL scores must have score_0_to_5 = -1 with rationale explaining no runtime evidence
- DO NOT generate findings as if runtime evidence existed
- Executive summary must state: "API not auditable in this environment. Service did not respond."
- If there are interesting AST patterns, list them as open_questions, NOT as findings
- remediation_plan must focus on: (1) making the service work, (2) re-running the analysis

FORBIDDEN:
- Giving positive scores without evidence
- Generating findings based only on AST as if confirmed
- Writing "no vulnerabilities found" when testing was not possible

`

const partialDocsPrefix = `WARNING: PARTIAL EVIDENCE.
The service responded but with insufficient coverage — many endpoints lack runtime evidence.

USE the standard structure (## 1 through ## 7) with these adaptations:
- Mark Key Concepts and Main Flows with "(partial evidence)" where runtime data is missing.
- ## 3. Main Flows: only include flows where at least some steps have runtime evidence.
- ## 7. Open Questions: explicitly list which endpoints and flows lack coverage.

Be conservative: do not assert behavior without evidence.

`

const partialAuditPrefix = `WARNING: PARTIAL EVIDENCE.
The API responded but with limited evidence. Scores must reflect this uncertainty.

RULES for partial evidence:
- Scores must be conservative (do not assume "all good" due to lack of evidence)
- Confidence in findings must be low (<0.5) unless clear evidence exists
- executive_summary must mention evidence limitations
- If insufficient material for a score, set score_0_to_5 = 2.5 (neutral) with rationale explaining
- Prefer classification="inconclusive" over "confirmed" unless strong evidence

`

const docsPrompt = `You are a senior technical writer producing an API integration guide. Your reader is a developer with ZERO prior context who needs to integrate with this service. Write as if onboarding a new team member — practical, direct, actionable.

WHAT YOU RECEIVE:
- service: project identity (name, description, framework, base_url). The DESCRIPTION is the owner's own words about what the service does — treat it as ground truth for understanding purpose and core flows.
- domains[]: code packages with handler names — reveals architecture
- route_summary[]: every METHOD /path grouped by domain — the full API surface
- response_shapes[]: top-level JSON keys from real 2xx responses — reveals resource structure
- auth_summary + auth_observed: authentication evidence
- common_errors[]: deduplicated error patterns
- findings: security findings with highlights
- metrics + stats: coverage data

CRITICAL RULES:
- If service.description exists, use it as the PRIMARY LENS to interpret ALL data. The description tells you what the service IS and what matters most. Align your Overview, Key Concepts, and Main Flows to the description.
- NEVER list package paths. Translate them into human-friendly domain names (e.g. "nexus-core/internal/identity" → "Identity & Access").
- NEVER enumerate all endpoints. Use representative examples to illustrate patterns.
- INTERPRET evidence. Don't parrot raw data — synthesize it into actionable knowledge.
- Use response_shapes to understand what each resource looks like (its fields). Infer entity relationships from shared field names.
- Mark anything without runtime evidence as "INFERRED" or "NOT VERIFIED".
- Keep ALL tables compact — max 60 characters per column.
- DO NOT INVENT. If you don't have evidence, say "No evidence available".

STRUCTURE (these exact H2 headings, in this order):

# {service.name} — API Guide

## 1. Overview
Start from service.description if available — expand it with what the dossier confirms. Architecture described as functional domains with purpose and key routes as examples. Coverage stats (N confirmed of M total).

## 2. Key Concepts
For each main resource/entity visible in the API (inferred from route paths, handler names, and response_shapes):
- **Entity name**: 1-2 sentence description of what it is and what it's used for.
- Key fields observed in response_shapes (if available).
- Relationships to other entities (inferred from shared fields or route nesting).
Group related entities. Order by importance to the service's core purpose (from description). Typically 5-10 entities. Mark all as "INFERRED from API surface" since you don't have specs.

## 3. Main Flows
3-5 common workflows a developer would need to perform.
THE MOST IMPORTANT FLOW COMES FIRST. Use service.description to identify it — it is almost always the service's primary operation (e.g. if the service "executes runs", the first flow must be about creating and managing runs).
For each flow:
- **Flow name** (e.g. "Execute a Run")
- Prerequisites (auth, prior resources needed)
- Steps: numbered list with METHOD /path and what it does
- Expected outcome
Mark all flows as "INFERRED from route patterns — verify with runtime testing".

## 4. Authentication
Evidence-based auth guide:
- **Observed mechanisms**: only what appears in auth_observed.headers_seen. State header name and how many times observed.
- **Auth rejection pattern**: show the error fingerprint (status + body) from auth_observed.error_fingerprints.
- **Coverage**: compact table with PROVEN_REQUIRED / PROVEN_NOT_REQUIRED / UNKNOWN counts.
- **How to get credentials**: if unknown, list where to look (env vars, admin endpoints, config files, bootstrap flow).
- **Discrepancies**: AST vs runtime mismatches if >5. One sentence each, max 5 examples.
DO NOT say "probably Bearer JWT" or similar — only state what was OBSERVED.

## 5. Common Errors
Compact table: | Status | Code | Message | Frequency | Likely Cause & Fix |
Max 60 chars per column. One row per pattern from common_errors[]. Max 10 rows.

## 6. Security Findings
From findings.highlights[]: severity, category, and 2-3 sentence description per finding.
If no findings: "No security findings reported."

## 7. Open Questions & Next Steps
Actionable bullet list:
- What credentials are needed and how to obtain them
- Which endpoints lack runtime evidence (count + examples)
- Which flows couldn't be verified and what to test next
- Specific gaps that block full integration

OUTPUT: Pure Markdown. No JSON wrapping. No code fences around the document. Start directly with the H1 heading.`

const langSuffixES = `

LANGUAGE OVERRIDE: Write the entire document in Spanish.
Technical terms (endpoints, headers, middleware, framework, runtime, AST, auth, tokens, Bearer, JWT, API key, etc.) MUST remain in English.
Only translate the narrative prose to Spanish. Keep code snippets, JSON keys, HTTP methods, and status codes as-is.`

const auditPrompt = `You are a senior AppSec + API Quality auditor (15+ years).
Audience: Tech Leads + Backend devs.

You receive a COMPACTED JSON v2 DOSSIER (dossier_final_llm.json) with:
- Canonical AST (endpoints, middlewares, handlers) + defined ast_refs
- Prioritized runtime evidence
- auth_matrix + discrepancies
- error_signatures
- Aggregated derived_metrics
- confirmations
- run_summary

MISSION:
Produce an EXPERT ACTIONABLE DIAGNOSTIC with:
- scores 0-5 per dimension (with rationale + evidence_refs)
- real findings (confirmed/anomaly/inconclusive)
- phased remediation plan (72hrs/2w/2m)
- DO NOT invent anything

GOLDEN RULE:
- ast_code_patterns are static observations. Only mention them if they correlate with runtime evidence.
- If no evidence: open_questions.

FINDINGS COUNT:
- 1 finding per real finding with evidence.
- Minimum expected 5; if fewer, explain why.

OUTPUT: Valid JSON with this EXACT schema (no markdown, no extra text):

{
  "schema_version": "v2",
  "run_id": "<copy from dossier.run_id>",
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
    "summary": "PARAGRAPH 6-10 sentences consultant-style (impact + priorities)",
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
    "high_level": "PARAGRAPH 4-6 sentences with conclusions based on auth_matrix",
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
      "what_we_observed": "PARAGRAPH 3-6 sentences with concrete endpoints/status/body",
      "why_it_matters": "PARAGRAPH 3-6 sentences (attack scenario or real failure)",
      "how_to_reproduce": [
        {"step": 1, "request_ref": "<evidence_id>", "expected": ""}
      ],
      "remediation": "PARAGRAPH 3-6 sentences with concrete steps",
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

MANDATORY RULES:
- JSON keys in English.
- Use 20-40 evidence_refs distributed when sufficient material exists; if the dossier is small, use as many as possible without inventing.
- Include ast_refs in findings where they add value (middlewares/handlers/patterns).
- If something is unstable: classification=anomaly and low confidence.`

const auditLangSuffixES = `

LANGUAGE OVERRIDE: Write all JSON string values in Spanish.
Technical terms (endpoints, headers, middleware, framework, runtime, AST, auth, tokens, Bearer, JWT, API key, etc.) MUST remain in English.
JSON keys must remain in English. Only translate the narrative text values to Spanish.`
