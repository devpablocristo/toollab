CRITICAL: THE SERVICE WAS OFFLINE DURING THE RUN.
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
