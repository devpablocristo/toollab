package llm

import _ "embed"

//go:embed prompts/offline_docs_prefix.md
var offlineDocsPrefix string

//go:embed prompts/offline_audit_prefix.md
var offlineAuditPrefix string

//go:embed prompts/partial_docs_prefix.md
var partialDocsPrefix string

//go:embed prompts/partial_audit_prefix.md
var partialAuditPrefix string

//go:embed prompts/docs_prompt.md
var docsPrompt string

//go:embed prompts/audit_prompt.md
var auditPrompt string

const langSuffixES = `

LANGUAGE OVERRIDE: Write the entire document in Spanish.
Technical terms (endpoints, headers, middleware, framework, runtime, AST, auth, tokens, Bearer, JWT, API key, etc.) MUST remain in English.
Only translate the narrative prose to Spanish. Keep code snippets, JSON keys, HTTP methods, and status codes as-is.`

const auditLangSuffixES = `

LANGUAGE OVERRIDE: Write all JSON string values in Spanish.
Technical terms (endpoints, headers, middleware, framework, runtime, AST, auth, tokens, Bearer, JWT, API key, etc.) MUST remain in English.
JSON keys must remain in English. Only translate the narrative text values to Spanish.`
