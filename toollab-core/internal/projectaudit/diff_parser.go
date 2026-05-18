package projectaudit

import (
	"path/filepath"
	"strings"
)

type parsedDiffFile struct {
	Path       string
	ChangeType string
	Additions  int
	Deletions  int
	Chunk      string
	AddedLines []string
}

func parseDiff(diffText string) []parsedDiffFile {
	lines := strings.Split(strings.ReplaceAll(diffText, "\r\n", "\n"), "\n")
	var files []parsedDiffFile
	current := -1

	startFile := func(path string) {
		files = append(files, parsedDiffFile{Path: cleanDiffPath(path), ChangeType: "modified"})
		current = len(files) - 1
	}
	ensureFile := func(path string) {
		if current < 0 {
			startFile(path)
			return
		}
		if files[current].Path == "" || files[current].Path == "unknown" {
			files[current].Path = cleanDiffPath(path)
		}
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			parts := strings.Fields(line)
			path := "unknown"
			if len(parts) >= 4 {
				path = parts[3]
			}
			startFile(path)
		case current >= 0 && strings.HasPrefix(line, "new file mode"):
			files[current].ChangeType = "added"
		case current >= 0 && strings.HasPrefix(line, "deleted file mode"):
			files[current].ChangeType = "deleted"
		case current >= 0 && strings.HasPrefix(line, "rename from "):
			files[current].ChangeType = "renamed"
		case current >= 0 && strings.HasPrefix(line, "rename to "):
			files[current].ChangeType = "renamed"
			files[current].Path = cleanDiffPath(strings.TrimPrefix(line, "rename to "))
		case strings.HasPrefix(line, "+++ "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			ensureFile(path)
			if current >= 0 {
				if path == "/dev/null" {
					files[current].ChangeType = "deleted"
				} else {
					files[current].Path = cleanDiffPath(path)
				}
			}
		case strings.HasPrefix(line, "--- "):
			path := strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			ensureFile(path)
			if current >= 0 && path == "/dev/null" {
				files[current].ChangeType = "added"
			}
		}

		if current < 0 {
			continue
		}
		files[current].Chunk += line + "\n"
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			files[current].Additions++
			files[current].AddedLines = append(files[current].AddedLines, strings.TrimPrefix(line, "+"))
		}
		if strings.HasPrefix(line, "-") {
			files[current].Deletions++
		}
	}

	for i := range files {
		if files[i].Path == "" {
			files[i].Path = "unknown"
		}
		if files[i].ChangeType == "" {
			files[i].ChangeType = "unknown"
		}
	}
	return files
}

func cleanDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, `"`)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == "" || path == "/dev/null" {
		return "unknown"
	}
	return filepath.ToSlash(path)
}

func classifyRisk(path string, diffChunk string) (riskArea string, riskLevel string) {
	lowerPath := strings.ToLower(path)
	lowerChunk := strings.ToLower(diffChunk)

	switch {
	case isTestPath(lowerPath):
		return "tests", prSeverityLow
	case isDocsPath(lowerPath):
		return "docs", prSeverityLow
	case containsAny(lowerPath, "auth", "permission", "role", "session", "token"):
		return "security/auth", prSeverityHigh
	case containsAny(lowerPath, "migration", "migrations", ".sql", "schema"):
		return "database", prSeverityHigh
	case containsAny(lowerPath, "api", "handler", "controller", "route", "dto", "contract", "openapi"):
		return "api_contract", prSeverityHigh
	case containsAny(lowerPath, "payment", "billing", "invoice"):
		return "payments", prSeverityHigh
	case containsAny(lowerPath, "dockerfile", "docker-compose", ".github/workflows", "nginx.conf"):
		return "deploy_config", prSeverityMedium
	case containsAny(lowerPath, "package.json", "package-lock.json", "go.mod", "go.sum"):
		return "dependencies", prSeverityMedium
	case containsAny(lowerChunk, "api_key=", "secret=", "password=", "token=", "private key"):
		return "security/auth", prSeverityHigh
	default:
		return "code", prSeverityLow
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func isTestPath(path string) bool {
	return strings.HasSuffix(path, "_test.go") ||
		strings.Contains(path, ".test.") ||
		strings.Contains(path, ".spec.") ||
		strings.HasPrefix(filepath.Base(path), "test_")
}

func isDocsPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.HasPrefix(base, "readme") ||
		strings.HasPrefix(base, "changelog") ||
		strings.HasSuffix(base, ".md") ||
		strings.HasSuffix(base, ".mdx") ||
		strings.Contains(path, "/docs/")
}

func isCriticalRiskArea(area string) bool {
	switch area {
	case "security/auth", "database", "api_contract", "payments":
		return true
	default:
		return false
	}
}
