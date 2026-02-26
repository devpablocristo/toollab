package explain

import (
	"fmt"
	"strings"
)

func RenderMD(in *Understanding) []byte {
	lines := []string{
		"# TOOLLAB Understanding",
		"",
		"## 1) Que es este servicio",
		in.Sections.WhatIs.Summary,
		"",
		"## 2) Como se usa",
		in.Sections.HowToUse.Summary,
		"",
		"## 3) Que se probo",
		in.Sections.WhatWasTested.Summary,
		"",
		"## 4) Que paso",
		in.Sections.WhatHappened.Summary,
		"",
		"## 5) Que fallo",
		in.Sections.WhatFailed.Summary,
		"",
		"## 6) Que esta probado",
		in.Sections.WhatIsProven.Summary,
		"",
		"## 7) Que es unknown",
		in.Sections.WhatIsUnknown.Summary,
		"",
		"## 8) Como reproducir",
		in.Sections.HowToReproduce.Summary,
		"",
		"## Claims",
	}
	for _, claim := range in.Claims {
		lines = append(lines, fmt.Sprintf("- [%s] %s", strings.ToUpper(claim.Status), claim.Statement))
		if len(claim.MissingEvidence) > 0 {
			lines = append(lines, fmt.Sprintf("  - missing: %s", strings.Join(claim.MissingEvidence, ", ")))
		}
	}
	lines = append(lines, "")
	lines = append(lines, "## Anchors")
	for _, anchor := range in.Anchors {
		lines = append(lines, fmt.Sprintf("- %s:%s", anchor.Type, anchor.Value))
	}
	lines = append(lines, "")
	return []byte(strings.Join(lines, "\n"))
}
