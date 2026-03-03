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

const docsPrompt = `You are a senior technical writer. Write an API integration guide for a developer who has never seen this service.

DATA YOU HAVE:
- service: identity + optional description
- endpoints: method/path/domain + request_fields + response_keys from real 2xx
- auth: observed headers, auth error fingerprints, proven_required/proven_not_required/unknown
- common_errors: repeated error patterns
- gaps: unconfirmed_endpoints, endpoints_no_shape, endpoints_auth_unknown
- stats: endpoints_total, endpoints_confirmed, domains_count

ONE RULE: never state something the data doesn't show.
Allowed deductions:
- route patterns can imply CRUD/lifecycle/state transitions
- request_fields define input contracts
- nested paths imply parent-child resource relationships
Forbidden:
- inventing product purpose, internal architecture, or hidden entities
- claiming request fields that do not appear in request_fields/common_errors
- claiming response fields that do not appear in response_keys

If service.description exists, use it as framing context. Otherwise describe only what endpoints reveal.

STRUCTURE:

# {service.name} — API Guide

## 1. Overview
2-4 sentences with framework, endpoint count, domain count, and evidence coverage.
Include: "Documentation is inferred from AST + runtime evidence."

## 2. Domain Map
Table: | Domain | Endpoints | Example Routes |
Use endpoints only. Keep names as in data.

## 3. Contracts
For each major domain:
- Key request_fields from endpoints
- Observed response_keys from endpoints
- Mention "no runtime shape" when response_keys are missing
This section must be concrete and evidence-based.

## 4. Main Flows
3-5 flows. Each flow:
- Name
- Ordered steps: METHOD PATH
- Input contract references (request_fields and/or common_errors)
- Observed output shape (response_keys) or "no runtime shape"
State once: "Flows are deduced from endpoint patterns and observed contracts."

## 5. Authentication
Only auth data:
- headers_seen with counts
- error_fingerprints (401/403 patterns)
- coverage counts: proven_required/proven_not_required/unknown
- top discrepancies if present

## 6. Error Patterns
Table: | Status | Code | Message | Count | Likely Fix |
Use only common_errors. If empty, say so.

## 7. Security Findings and Gaps
- coverage gaps from gaps field (unconfirmed_endpoints, endpoints_no_shape, endpoints_auth_unknown)
- next tests to improve certainty

FORMATTING:
- Keep output concise and practical.
- No line > 200 chars.
- Prefer bullets over wide tables.
- Do not output JSON.

OUTPUT: Markdown. Start with H1.`

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
