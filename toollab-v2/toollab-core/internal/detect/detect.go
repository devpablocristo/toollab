package detect

import (
	"strings"

	"toollab-v2/internal/model"
)

func LanguageAndFramework(snapshot model.RepoSnapshot) (string, string) {
	hasGo := false
	hasMod := false
	imports := strings.Builder{}

	for _, f := range snapshot.Files {
		switch {
		case strings.HasSuffix(f.Path, ".go"):
			hasGo = true
		case f.Path == "go.mod":
			hasMod = true
		}
		if strings.Contains(f.Path, "gin") || strings.Contains(f.Path, "chi") {
			imports.WriteString(f.Path)
		}
	}

	if hasGo || hasMod {
		framework := "net/http"
		raw := imports.String()
		switch {
		case strings.Contains(raw, "gin"):
			framework = "gin"
		case strings.Contains(raw, "chi"):
			framework = "chi"
		}
		return "go", framework
	}
	return "unknown", "unknown"
}
