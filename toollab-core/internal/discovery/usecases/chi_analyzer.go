package usecases

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"toollab-core/internal/discovery/usecases/domain"
	"toollab-core/internal/shared"
)

var httpMethods = map[string]string{
	"Get":     "GET",
	"Post":    "POST",
	"Put":     "PUT",
	"Delete":  "DELETE",
	"Patch":   "PATCH",
	"Head":    "HEAD",
	"Options": "OPTIONS",
}

type ChiAnalyzer struct{}

func NewChiAnalyzer() *ChiAnalyzer { return &ChiAnalyzer{} }

func (a *ChiAnalyzer) Analyze(localPath string, hint domain.FrameworkHint) (domain.ServiceModel, domain.ModelReport, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return domain.ServiceModel{}, domain.ModelReport{}, fmt.Errorf("path not accessible: %w", err)
	}
	if !info.IsDir() {
		return domain.ServiceModel{}, domain.ModelReport{}, fmt.Errorf("path is not a directory: %s", localPath)
	}

	var endpoints []domain.Endpoint
	var gaps []string
	goFiles := 0

	err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == "node_modules" || base == ".git" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		goFiles++
		found, fileGaps := a.analyzeFile(path, localPath)
		endpoints = append(endpoints, found...)
		gaps = append(gaps, fileGaps...)
		return nil
	})
	if err != nil {
		return domain.ServiceModel{}, domain.ModelReport{}, fmt.Errorf("walking directory: %w", err)
	}

	if goFiles == 0 {
		gaps = append(gaps, "no Go files found in path")
	}

	endpoints = deduplicateEndpoints(endpoints)

	confidence := computeConfidence(endpoints, gaps)

	model := domain.ServiceModel{
		SchemaVersion: "v1",
		Framework:     "chi",
		RootPath:      localPath,
		Endpoints:     endpoints,
		CreatedAt:     shared.Now(),
	}
	report := domain.ModelReport{
		SchemaVersion:  "v1",
		EndpointsCount: len(endpoints),
		Confidence:     confidence,
		Gaps:           gaps,
		CreatedAt:      shared.Now(),
	}
	return model, report, nil
}

func (a *ChiAnalyzer) analyzeFile(filePath, rootPath string) ([]domain.Endpoint, []string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, []string{fmt.Sprintf("parse error in %s: %v", relPath(filePath, rootPath), err)}
	}

	var endpoints []domain.Endpoint
	var gaps []string

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		methodName := sel.Sel.Name

		if httpMethod, isHTTP := httpMethods[methodName]; isHTTP {
			ep, gap := a.extractRoute(fset, call, httpMethod, filePath, rootPath, "")
			if ep != nil {
				endpoints = append(endpoints, *ep)
			}
			if gap != "" {
				gaps = append(gaps, gap)
			}
			return true
		}

		if methodName == "Route" {
			nested, nestedGaps := a.extractRouteGroup(fset, call, filePath, rootPath)
			endpoints = append(endpoints, nested...)
			gaps = append(gaps, nestedGaps...)
			return false
		}

		if methodName == "Mount" {
			if len(call.Args) >= 1 {
				prefix := extractStringLit(call.Args[0])
				if prefix != "" {
					pos := fset.Position(call.Pos())
					gaps = append(gaps, fmt.Sprintf("mount at %q in %s:%d (sub-router not fully traced)",
						prefix, relPath(filePath, rootPath), pos.Line))
				}
			}
		}

		return true
	})

	return endpoints, gaps
}

func (a *ChiAnalyzer) extractRoute(fset *token.FileSet, call *ast.CallExpr, httpMethod, filePath, rootPath, prefix string) (*domain.Endpoint, string) {
	if len(call.Args) < 2 {
		return nil, ""
	}

	pathStr := extractStringLit(call.Args[0])
	if pathStr == "" {
		pos := fset.Position(call.Pos())
		return nil, fmt.Sprintf("dynamic path in %s at %s:%d", httpMethod, relPath(filePath, rootPath), pos.Line)
	}

	fullPath := normalizePath(prefix + pathStr)
	handlerName := extractHandlerName(call.Args[1])
	pos := fset.Position(call.Pos())

	ep := &domain.Endpoint{
		Method:      httpMethod,
		Path:        fullPath,
		HandlerName: handlerName,
		Ref: &shared.ModelRef{
			Kind: "endpoint",
			ID:   httpMethod + " " + fullPath,
			File: relPath(filePath, rootPath),
			Line: pos.Line,
		},
	}
	return ep, ""
}

func (a *ChiAnalyzer) extractRouteGroup(fset *token.FileSet, call *ast.CallExpr, filePath, rootPath string) ([]domain.Endpoint, []string) {
	if len(call.Args) < 2 {
		return nil, nil
	}

	prefix := extractStringLit(call.Args[0])
	if prefix == "" {
		pos := fset.Position(call.Pos())
		return nil, []string{fmt.Sprintf("dynamic route prefix in %s:%d", relPath(filePath, rootPath), pos.Line)}
	}

	funcLit, ok := call.Args[1].(*ast.FuncLit)
	if !ok {
		return nil, nil
	}

	var endpoints []domain.Endpoint
	var gaps []string

	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		innerCall, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := innerCall.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if httpMethod, isHTTP := httpMethods[sel.Sel.Name]; isHTTP {
			ep, gap := a.extractRoute(fset, innerCall, httpMethod, filePath, rootPath, prefix)
			if ep != nil {
				endpoints = append(endpoints, *ep)
			}
			if gap != "" {
				gaps = append(gaps, gap)
			}
			return true
		}

		if sel.Sel.Name == "Route" {
			innerPrefix := ""
			if len(innerCall.Args) >= 1 {
				innerPrefix = extractStringLit(innerCall.Args[0])
			}
			if innerPrefix != "" && len(innerCall.Args) >= 2 {
				if innerFuncLit, ok := innerCall.Args[1].(*ast.FuncLit); ok {
					nestedCall := &ast.CallExpr{
						Fun:  innerCall.Fun,
						Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: `"` + prefix + innerPrefix + `"`}, innerFuncLit},
					}
					nested, nestedGaps := a.extractRouteGroup(fset, nestedCall, filePath, rootPath)
					endpoints = append(endpoints, nested...)
					gaps = append(gaps, nestedGaps...)
				}
			}
			return false
		}

		return true
	})

	return endpoints, gaps
}

func extractStringLit(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s := lit.Value
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1]
	}
	return ""
}

func extractHandlerName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
		return v.Sel.Name
	case *ast.FuncLit:
		return "(anonymous)"
	}
	return "(unknown)"
}

func normalizePath(p string) string {
	p = strings.ReplaceAll(p, "//", "/")
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func relPath(filePath, rootPath string) string {
	rel, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		return filePath
	}
	return rel
}

func deduplicateEndpoints(eps []domain.Endpoint) []domain.Endpoint {
	seen := make(map[string]bool, len(eps))
	var out []domain.Endpoint
	for _, ep := range eps {
		key := ep.Method + " " + ep.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ep)
	}
	return out
}

func computeConfidence(eps []domain.Endpoint, gaps []string) float64 {
	if len(eps) == 0 {
		return 0.0
	}
	c := 0.9
	penalty := float64(len(gaps)) * 0.05
	c -= penalty
	anonCount := 0
	for _, ep := range eps {
		if ep.HandlerName == "(anonymous)" || ep.HandlerName == "(unknown)" {
			anonCount++
		}
	}
	if anonCount > 0 {
		c -= float64(anonCount) / float64(len(eps)) * 0.1
	}
	if c < 0.1 {
		c = 0.1
	}
	return c
}
