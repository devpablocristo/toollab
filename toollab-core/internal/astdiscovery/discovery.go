package discovery

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"

	d "toollab-core/internal/domain"
	"toollab-core/internal/pipeline"
)

type Step struct{}

func New() *Step { return &Step{} }

func (s *Step) Name() d.PipelineStep { return d.StepDiscovery }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	localPath := state.Target.Source.Value
	if localPath == "" {
		return d.StepResult{Step: d.StepDiscovery, Status: "failed", DurationMs: ms(start), Error: "no source path"}
	}

	if info, err := os.Stat(localPath); err != nil || !info.IsDir() {
		return d.StepResult{Step: d.StepDiscovery, Status: "failed", DurationMs: ms(start), Error: fmt.Sprintf("path not accessible: %s", localPath)}
	}

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps |
			packages.NeedImports,
		Dir: localPath,
	}, "./...")
	if err != nil {
		return d.StepResult{Step: d.StepDiscovery, Status: "failed", DurationMs: ms(start), Error: err.Error()}
	}

	scan := &scanState{
		rootPath:    localPath,
		prefixes:    make(map[types.Object]string),
		groupRefs:   make(map[types.Object]*d.RouteGroup),
		middlewares: make(map[types.Object][]d.ASTRef),
	}

	scan.collectPrefixes(pkgs)
	scan.propagatePrefixes(pkgs)
	scan.extractEndpoints(pkgs)
	scan.detectCodePatterns(pkgs)
	scan.detectEntities(pkgs)

	framework := detectFramework(pkgs)
	endpoints := deduplicateEndpoints(scan.endpoints)
	confidence := computeConfidence(endpoints, scan.gaps)

	catalog := &d.EndpointCatalog{
		SchemaVersion: "v2",
		Framework:     framework,
		Endpoints:     endpoints,
		TotalCount:    len(endpoints),
		Confidence:    confidence,
		Gaps:          scan.gaps,
	}

	graph := scan.buildRouterGraph()

	state.Catalog = catalog
	state.RouterGraph = graph
	state.ASTEntities = scan.entities
	state.ASTCodePatterns = scan.codePatterns

	// Enrich TargetProfile
	if state.TargetProfile != nil {
		state.TargetProfile.FrameworkGuess = framework
		for _, ep := range endpoints {
			for _, mw := range ep.Middlewares {
				if mw.Extra.MiddlewareName != "" {
					name := strings.ToLower(mw.Extra.MiddlewareName)
					if strings.Contains(name, "auth") || strings.Contains(name, "jwt") || strings.Contains(name, "bearer") {
						state.TargetProfile.AuthHints.MiddlewareNames = appendUniq(state.TargetProfile.AuthHints.MiddlewareNames, mw.Label)
					}
				}
			}
		}
	}

	state.Emit(pipeline.ProgressEvent{
		Step:    d.StepDiscovery,
		Phase:   "results",
		Message: fmt.Sprintf("Found %d endpoints (%s, %.0f%% confidence), %d code patterns, %d entities", len(endpoints), framework, confidence*100, len(scan.codePatterns), len(scan.entities)),
	})

	return d.StepResult{
		Step:       d.StepDiscovery,
		Status:     "ok",
		DurationMs: ms(start),
	}
}

// scanState holds mutable state across analysis passes.
type scanState struct {
	rootPath     string
	prefixes     map[types.Object]string
	groupRefs    map[types.Object]*d.RouteGroup
	middlewares  map[types.Object][]d.ASTRef
	endpoints    []d.EndpointEntry
	codePatterns []d.ASTCodePattern
	entities     []d.ASTEntity
	gaps         []string
	groups       []d.RouteGroup
}

// --- Pass 1: Collect group prefixes ---

func (s *scanState) collectPrefixes(pkgs []*packages.Package) {
	eachFile(pkgs, func(pkg *packages.Package, file *ast.File, filePath string) {
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
							full := parentPrefix + prefix
							s.prefixes[obj] = full
							pos := pkg.Fset.Position(assign.Pos())
							group := &d.RouteGroup{
								GroupID: d.ASTRefID(d.ASTRefRouteGroup, full, s.rel(filePath)),
								Prefix:  full,
								ASTRef: &d.ASTRef{
									Type:     d.ASTRefRouteGroup,
									ID:       d.ASTRefID(d.ASTRefRouteGroup, full, s.rel(filePath)),
									Label:    id.Name + " (" + full + ")",
									Location: d.ASTLocation{File: s.rel(filePath), Line: pos.Line, Package: pkg.PkgPath},
									Extra:    d.ASTRefExtra{GroupPrefix: full, Symbol: id.Name},
								},
							}
							s.groupRefs[obj] = group
							s.groups = append(s.groups, *group)
						}
					}
				}
			}
			return true
		})
	})
}

// --- Pass 2: Propagate prefixes ---

func (s *scanState) propagatePrefixes(pkgs []*packages.Package) {
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

// --- Pass 3: Extract endpoints with AST refs ---

func (s *scanState) extractEndpoints(pkgs []*packages.Package) {
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
			handlerName := extractHandlerName(call.Args, sel.Sel.Name)
			pos := pkg.Fset.Position(call.Pos())

			eid := d.EndpointID(httpMethod, fullPath)

			handlerRef := &d.ASTRef{
				Type:     d.ASTRefHandler,
				ID:       d.ASTRefID(d.ASTRefHandler, pkg.PkgPath, s.rel(filePath), handlerName),
				Label:    handlerName,
				Location: d.ASTLocation{File: s.rel(filePath), Line: pos.Line, Package: pkg.PkgPath},
				Extra:    d.ASTRefExtra{Symbol: handlerName},
			}

			// Collect middlewares from group
			var middlewareRefs []d.ASTRef
			if id, ok := sel.X.(*ast.Ident); ok {
				if obj := pkg.TypesInfo.ObjectOf(id); obj != nil {
					middlewareRefs = s.middlewares[obj]
				}
			}

			var groupRef *d.ASTRef
			if id, ok := sel.X.(*ast.Ident); ok {
				if obj := pkg.TypesInfo.ObjectOf(id); obj != nil {
					if g, ok := s.groupRefs[obj]; ok {
						groupRef = g.ASTRef
					}
				}
			}

			entry := d.EndpointEntry{
				EndpointID:  eid,
				Method:      httpMethod,
				Path:        fullPath,
				HandlerRef:  handlerRef,
				Middlewares:  middlewareRefs,
				GroupRef:    groupRef,
			}

			s.endpoints = append(s.endpoints, entry)
			return true
		})
	})
}

// --- Pass 4: Detect code patterns (static, neutral) ---

func (s *scanState) detectCodePatterns(pkgs []*packages.Package) {
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

			pos := pkg.Fset.Position(call.Pos())
			relFile := s.rel(filePath)

			name := sel.Sel.Name
			switch {
			case name == "Exec" || name == "Query" || name == "QueryRow":
				if len(call.Args) > 0 {
					if !isStringLiteral(call.Args[0]) {
						s.addPattern("string_format_sql", "SQL query with non-literal string argument", relFile, pos.Line, pkg.PkgPath, []string{"db", "sql"})
					}
				}
			case name == "Command" || name == "CommandContext":
				s.addPattern("exec_command", "OS command execution observed", relFile, pos.Line, pkg.PkgPath, []string{"exec", "os"})
			case name == "Open" || name == "ReadFile" || name == "WriteFile":
				if obj := pkg.TypesInfo.ObjectOf(sel.Sel); obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "os" {
					s.addPattern("file_open", "File system operation observed", relFile, pos.Line, pkg.PkgPath, []string{"filesystem"})
				}
			}
			return true
		})
	})
}

func (s *scanState) addPattern(pattern, desc, file string, line int, pkgPath string, tags []string) {
	pid := d.ASTRefID(d.ASTRefPattern, pattern, file, fmt.Sprintf("%d", line))
	s.codePatterns = append(s.codePatterns, d.ASTCodePattern{
		PatternID:   pid,
		Pattern:     pattern,
		Description: desc,
		ASTRef: d.ASTRef{
			Type:     d.ASTRefPattern,
			ID:       pid,
			Label:    pattern,
			Location: d.ASTLocation{File: file, Line: line, Package: pkgPath},
			Extra:    d.ASTRefExtra{PatternName: pattern},
		},
		Tags: tags,
	})
}

// --- Pass 5: Detect entities (DTOs, handlers) ---

func (s *scanState) detectEntities(pkgs []*packages.Package) {
	eachFile(pkgs, func(pkg *packages.Package, file *ast.File, filePath string) {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				pos := pkg.Fset.Position(ts.Pos())
				relFile := s.rel(filePath)

				var fields []string
				if st.Fields != nil {
					for _, f := range st.Fields.List {
						for _, name := range f.Names {
							fields = append(fields, name.Name)
						}
					}
				}

				if len(fields) == 0 {
					continue
				}

				ref := d.ASTRef{
					Type:     d.ASTRefDTO,
					ID:       d.ASTRefID(d.ASTRefDTO, pkg.PkgPath, ts.Name.Name),
					Label:    ts.Name.Name,
					Location: d.ASTLocation{File: relFile, Line: pos.Line, Package: pkg.PkgPath},
					Extra:    d.ASTRefExtra{DTOName: ts.Name.Name},
				}

				name := strings.ToLower(ts.Name.Name)
				isDTO := strings.HasSuffix(name, "request") || strings.HasSuffix(name, "response") ||
					strings.HasSuffix(name, "dto") || strings.HasSuffix(name, "payload") ||
					strings.HasSuffix(name, "input") || strings.HasSuffix(name, "output") ||
					strings.HasSuffix(name, "body") || strings.HasSuffix(name, "params")

				if isDTO || hasJSONTags(st) {
					s.entities = append(s.entities, d.ASTEntity{
						ASTRef: ref,
						Kind:   "dto",
						Name:   ts.Name.Name,
						Fields: fields,
					})
				}
			}
		}
	})
}

func (s *scanState) buildRouterGraph() *d.RouterGraph {
	return &d.RouterGraph{Groups: s.groups}
}

// --- Helpers ---

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

var verbs = map[string]string{
	"GET": "GET", "POST": "POST", "PUT": "PUT", "DELETE": "DELETE",
	"PATCH": "PATCH", "HEAD": "HEAD", "OPTIONS": "OPTIONS",
	"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE",
	"Patch": "PATCH", "Head": "HEAD", "Options": "OPTIONS",
	"Any": "ANY", "HandleFunc": "ANY", "Handle": "ANY",
}

var prefixMethods = map[string]bool{"Group": true, "Route": true}

func (s *scanState) groupCall(pkg *packages.Package, expr ast.Expr) (string, ast.Expr) {
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

func (s *scanState) prefixOf(pkg *packages.Package, expr ast.Expr) string {
	id, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	obj := pkg.TypesInfo.ObjectOf(id)
	if obj == nil {
		return ""
	}
	return s.prefixes[obj]
}

func (s *scanState) rel(filePath string) string {
	if r, err := filepath.Rel(s.rootPath, filePath); err == nil {
		return r
	}
	return filePath
}

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

func isStringLiteral(expr ast.Expr) bool {
	_, ok := expr.(*ast.BasicLit)
	return ok
}

func extractHandlerName(args []ast.Expr, method string) string {
	idx := 1
	if method == "Handle" && len(args) >= 3 {
		idx = 2
	}
	if idx >= len(args) {
		if len(args) > 1 {
			return nameFromExpr(args[len(args)-1])
		}
		return "(unknown)"
	}
	return nameFromExpr(args[idx])
}

func nameFromExpr(expr ast.Expr) string {
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

func detectFramework(pkgs []*packages.Package) string {
	for _, pkg := range pkgs {
		for imp := range pkg.Imports {
			switch {
			case strings.Contains(imp, "gin"):
				return "gin"
			case strings.Contains(imp, "chi"):
				return "chi"
			case strings.Contains(imp, "echo"):
				return "echo"
			case strings.Contains(imp, "mux"):
				return "gorilla"
			case strings.Contains(imp, "fiber"):
				return "fiber"
			}
		}
	}
	return "go"
}

func deduplicateEndpoints(eps []d.EndpointEntry) []d.EndpointEntry {
	seen := make(map[string]bool, len(eps))
	var out []d.EndpointEntry
	for _, ep := range eps {
		if seen[ep.EndpointID] {
			continue
		}
		seen[ep.EndpointID] = true
		out = append(out, ep)
	}
	return out
}

func computeConfidence(eps []d.EndpointEntry, gaps []string) float64 {
	if len(eps) == 0 {
		return 0.0
	}
	c := 0.9
	c -= float64(len(gaps)) * 0.05
	anon := 0
	for _, ep := range eps {
		if ep.HandlerRef != nil && (ep.HandlerRef.Label == "(anonymous)" || ep.HandlerRef.Label == "(unknown)") {
			anon++
		}
	}
	if anon > 0 {
		c -= float64(anon) / float64(len(eps)) * 0.1
	}
	if c < 0.1 {
		c = 0.1
	}
	return c
}

func hasJSONTags(st *ast.StructType) bool {
	if st.Fields == nil {
		return false
	}
	for _, f := range st.Fields.List {
		if f.Tag != nil && strings.Contains(f.Tag.Value, "json:") {
			return true
		}
	}
	return false
}

func appendUniq(ss []string, s string) []string {
	for _, v := range ss {
		if v == s {
			return ss
		}
	}
	return append(ss, s)
}

func ms(start time.Time) int64 { return time.Since(start).Milliseconds() }
