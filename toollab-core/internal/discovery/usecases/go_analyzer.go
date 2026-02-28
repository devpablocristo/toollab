package usecases

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"toollab-core/internal/discovery/usecases/domain"
	"toollab-core/internal/shared"
)

// GoAnalyzer discovers HTTP endpoints in any Go project using the standard
// toolchain: go/packages loads the full module, go/types resolves every
// expression's type, and go/ast walks the syntax trees.
//
// The only domain knowledge encoded here is:
//   - routerTypes: which types from which packages are HTTP routers
//   - verbs: which method names on those types correspond to HTTP methods
//   - prefixMethods: which method names create sub-routers with a path prefix
//
// Everything else — prefix propagation, cross-package data flow — is derived
// from the type system, not from heuristics or name matching.
type GoAnalyzer struct{}

func NewGoAnalyzer() *GoAnalyzer { return &GoAnalyzer{} }

// ---------------------------------------------------------------------------
// Domain knowledge — the only configuration points
// ---------------------------------------------------------------------------

// routerTypes maps package import paths to the specific type names within that
// package that represent HTTP routers. Only these types trigger endpoint
// extraction — other types from the same package (e.g. gin.Context, echo.Context)
// are ignored.
var routerTypes = map[string][]string{
	"github.com/gin-gonic/gin":    {"Engine", "RouterGroup"},
	"github.com/go-chi/chi":       {"Mux", "Router"},
	"github.com/go-chi/chi/v5":    {"Mux", "Router"},
	"github.com/labstack/echo":    {"Echo", "Group"},
	"github.com/labstack/echo/v4": {"Echo", "Group"},
	"github.com/gorilla/mux":      {"Router"},
	"github.com/gofiber/fiber":    {"App", "Group", "Router"},
	"github.com/gofiber/fiber/v2": {"App", "Group", "Router"},
	"net/http":                    {"ServeMux"},
}

// verbs maps method names on router types to canonical HTTP methods.
var verbs = map[string]string{
	"GET": "GET", "POST": "POST", "PUT": "PUT", "DELETE": "DELETE",
	"PATCH": "PATCH", "HEAD": "HEAD", "OPTIONS": "OPTIONS",
	"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE",
	"Patch": "PATCH", "Head": "HEAD", "Options": "OPTIONS",
	"Any": "ANY", "HandleFunc": "ANY", "Handle": "ANY",
	"Static": "STATIC", "StaticFile": "STATIC", "StaticFS": "STATIC",
}

// prefixMethods lists method names that create a sub-router with a path prefix.
var prefixMethods = map[string]bool{
	"Group": true, "Route": true,
}

// ---------------------------------------------------------------------------
// Analyze — entry point
// ---------------------------------------------------------------------------

func (a *GoAnalyzer) Analyze(localPath string, _ domain.FrameworkHint) (domain.ServiceModel, domain.ModelReport, error) {
	if info, err := os.Stat(localPath); err != nil {
		return domain.ServiceModel{}, domain.ModelReport{}, fmt.Errorf("path not accessible: %w", err)
	} else if !info.IsDir() {
		return domain.ServiceModel{}, domain.ModelReport{}, fmt.Errorf("not a directory: %s", localPath)
	}

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps |
			packages.NeedImports,
		Dir: localPath,
	}, "./...")
	if err != nil {
		return domain.ServiceModel{}, domain.ModelReport{}, fmt.Errorf("loading packages: %w", err)
	}

	s := &scan{
		rootPath: localPath,
		prefixes: make(map[types.Object]string),
	}

	s.collectPrefixes(pkgs)
	s.propagatePrefixes(pkgs)
	s.extractEndpoints(pkgs)

	eps := deduplicateEndpoints(s.endpoints)
	framework := detectFramework(pkgs)
	confidence := computeConfidence(eps, s.gaps)

	return domain.ServiceModel{
			SchemaVersion: "v1",
			Framework:     framework,
			RootPath:      localPath,
			Endpoints:     eps,
			CreatedAt:     shared.Now(),
		}, domain.ModelReport{
			SchemaVersion:  "v1",
			EndpointsCount: len(eps),
			Confidence:     confidence,
			Gaps:           nonNil(s.gaps),
			CreatedAt:      shared.Now(),
		}, nil
}

// ---------------------------------------------------------------------------
// scan holds mutable state across the three passes
// ---------------------------------------------------------------------------

type scan struct {
	rootPath  string
	prefixes  map[types.Object]string // typed object → accumulated path prefix
	endpoints []domain.Endpoint
	gaps      []string
}

// ---------------------------------------------------------------------------
// Pass 1 — collect prefix assignments (v1 := r.Group("/v1"))
// ---------------------------------------------------------------------------

func (s *scan) collectPrefixes(pkgs []*packages.Package) {
	eachFile(pkgs, func(pkg *packages.Package, file *ast.File, _ string) {
		ast.Inspect(file, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, rhs := range assign.Rhs {
				prefix, recv := s.groupCall(pkg, rhs)
				if prefix == "" && recv == nil {
					continue
				}
				parentPrefix := s.prefixOf(pkg, recv)
				if i < len(assign.Lhs) {
					if id, ok := assign.Lhs[i].(*ast.Ident); ok {
						if obj := pkg.TypesInfo.ObjectOf(id); obj != nil {
							s.prefixes[obj] = parentPrefix + prefix
						}
					}
				}
			}
			return true
		})
	})
}

// groupCall checks if expr is a call like x.Group("/prefix") on a router type.
func (s *scan) groupCall(pkg *packages.Package, expr ast.Expr) (string, ast.Expr) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return "", nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !prefixMethods[sel.Sel.Name] {
		return "", nil
	}
	if !isRouter(pkg.TypesInfo, sel.X) {
		return "", nil
	}
	if len(call.Args) < 1 {
		return "", nil
	}
	return stringLit(call.Args[0]), sel.X
}

// ---------------------------------------------------------------------------
// Pass 2 — propagate prefixes through function calls (type-driven data flow)
// ---------------------------------------------------------------------------
//
// For every call where a router-typed argument carries a known prefix,
// resolve the callee function via go/types and assign that prefix to the
// corresponding parameter object. This works for ANY function name — no
// string matching on "Register", "SetupRoutes", etc.
//
// Iterates until convergence to handle multi-level delegation:
//   main → handler.Register(v1Group) → subHandler.Wire(rg)

func (s *scan) propagatePrefixes(pkgs []*packages.Package) {
	for {
		changed := false
		eachFile(pkgs, func(pkg *packages.Package, file *ast.File, _ string) {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				for i, arg := range call.Args {
					if !isRouter(pkg.TypesInfo, arg) {
						continue
					}
					argPrefix := s.prefixOf(pkg, arg)
					if argPrefix == "" {
						continue
					}
					fn := resolveCallee(pkg, call)
					if fn == nil {
						continue
					}
					sig, ok := fn.Type().(*types.Signature)
					if !ok || i >= sig.Params().Len() {
						continue
					}
					param := sig.Params().At(i)
					if prev, exists := s.prefixes[param]; exists && prev == argPrefix {
						continue
					}
					s.prefixes[param] = argPrefix
					changed = true
				}
				return true
			})
		})
		if !changed {
			break
		}
	}
}

// resolveCallee extracts the *types.Func that a call expression invokes.
func resolveCallee(pkg *packages.Package, call *ast.CallExpr) *types.Func {
	var obj types.Object
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		obj = pkg.TypesInfo.ObjectOf(fn.Sel)
	case *ast.Ident:
		obj = pkg.TypesInfo.ObjectOf(fn)
	}
	if obj == nil {
		return nil
	}
	f, _ := obj.(*types.Func)
	return f
}

// ---------------------------------------------------------------------------
// Pass 3 — extract endpoints
// ---------------------------------------------------------------------------

func (s *scan) extractEndpoints(pkgs []*packages.Package) {
	eachFile(pkgs, func(pkg *packages.Package, file *ast.File, filePath string) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			httpMethod, ok := verbs[sel.Sel.Name]
			if !ok {
				return true
			}
			if !isRouter(pkg.TypesInfo, sel.X) {
				return true
			}
			if len(call.Args) < 1 {
				return true
			}
			pathStr := stringLit(call.Args[0])
			if pathStr == "" {
				pos := pkg.Fset.Position(call.Pos())
				s.gaps = append(s.gaps, fmt.Sprintf("dynamic path in %s:%d", s.rel(filePath), pos.Line))
				return true
			}

			prefix := s.prefixOf(pkg, sel.X)
			fullPath := normPath(prefix + pathStr)
			handler := handlerName(call.Args, sel.Sel.Name)
			pos := pkg.Fset.Position(call.Pos())

			s.endpoints = append(s.endpoints, domain.Endpoint{
				Method:      httpMethod,
				Path:        fullPath,
				HandlerName: handler,
				Ref: &shared.ModelRef{
					Kind: "endpoint",
					ID:   httpMethod + " " + fullPath,
					File: s.rel(filePath),
					Line: pos.Line,
				},
			})
			return true
		})
	})
}

// ---------------------------------------------------------------------------
// Type resolution — the only code that touches go/types
// ---------------------------------------------------------------------------

// isRouter returns true if expr's resolved type is a known HTTP router.
func isRouter(info *types.Info, expr ast.Expr) bool {
	if info == nil {
		return false
	}
	t := info.TypeOf(expr)
	if t == nil {
		return false
	}
	return isRouterType(t)
}

// isRouterType classifies a type as a router by checking if its concrete type
// name appears in routerTypes for its package. This is precise: gin.Engine is
// a router, gin.Context is not, even though both come from the same package.
func isRouterType(t types.Type) bool {
	switch v := t.(type) {
	case *types.Named:
		if pkg := v.Obj().Pkg(); pkg != nil {
			typeName := v.Obj().Name()
			if names, ok := routerTypes[pkg.Path()]; ok {
				for _, n := range names {
					if typeName == n {
						return true
					}
				}
			}
		}
	case *types.Pointer:
		return isRouterType(v.Elem())
	case *types.Interface:
		repr := v.String()
		for pkgPath := range routerTypes {
			if strings.Contains(repr, pkgPath) {
				return true
			}
		}
	}
	return false
}

// prefixOf resolves the accumulated group prefix for a named expression.
func (s *scan) prefixOf(pkg *packages.Package, expr ast.Expr) string {
	id, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	obj := pkg.TypesInfo.ObjectOf(id)
	if obj == nil {
		return ""
	}
	if p, ok := s.prefixes[obj]; ok {
		return p
	}
	return ""
}

// ---------------------------------------------------------------------------
// Pure helpers — no type info, no mutable state
// ---------------------------------------------------------------------------

func eachFile(pkgs []*packages.Package, fn func(*packages.Package, *ast.File, string)) {
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil || pkg.Fset == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			path := pkg.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			fn(pkg, file, path)
		}
	}
}

func stringLit(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s := lit.Value
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '`' && s[len(s)-1] == '`') {
			return s[1 : len(s)-1]
		}
	}
	return ""
}

func handlerName(args []ast.Expr, method string) string {
	idx := 1
	if method == "Handle" && len(args) >= 3 {
		idx = 2
	}
	if idx >= len(args) {
		if len(args) > 1 {
			return extractHandlerName(args[len(args)-1])
		}
		return ""
	}
	return extractHandlerName(args[idx])
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

func normPath(p string) string {
	p = strings.ReplaceAll(p, "//", "/")
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func (s *scan) rel(filePath string) string {
	if r, err := filepath.Rel(s.rootPath, filePath); err == nil {
		return r
	}
	return filePath
}

func detectFramework(pkgs []*packages.Package) string {
	for _, pkg := range pkgs {
		for imp := range pkg.Imports {
			for pkgPath := range routerTypes {
				if strings.Contains(imp, pkgPath) {
					switch {
					case strings.Contains(pkgPath, "gin"):
						return "gin"
					case strings.Contains(pkgPath, "chi"):
						return "chi"
					case strings.Contains(pkgPath, "echo"):
						return "echo"
					case strings.Contains(pkgPath, "mux"):
						return "gorilla"
					case strings.Contains(pkgPath, "fiber"):
						return "fiber"
					}
				}
			}
		}
	}
	return "go"
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

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
