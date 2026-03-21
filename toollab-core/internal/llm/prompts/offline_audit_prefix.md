CRITICAL: THE SERVICE WAS OFFLINE DURING THE RUN.
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
