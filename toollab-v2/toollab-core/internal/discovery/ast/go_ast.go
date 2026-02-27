package astx

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"toollab-v2/internal/shared/common"
	"toollab-v2/internal/shared/model"
)

var httpMethods = map[string]bool{
	"GET":    true,
	"POST":   true,
	"PUT":    true,
	"PATCH":  true,
	"DELETE": true,
}

func ExtractGoModel(snapshot model.RepoSnapshot) (endpoints []model.Endpoint, types []model.ModelType, deps []model.Dependency, err error) {
	fset := token.NewFileSet()
	dirs := map[string]struct{}{}
	for _, f := range snapshot.Files {
		if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		dir := path.Dir(f.Path)
		if dir == "." {
			dir = ""
		}
		dirs[dir] = struct{}{}
	}
	if len(dirs) == 0 {
		return nil, nil, nil, nil
	}

	dirList := make([]string, 0, len(dirs))
	for d := range dirs {
		dirList = append(dirList, d)
	}
	sort.Strings(dirList)

	endpointMap := map[string]model.Endpoint{}
	typeMap := map[string]model.ModelType{}
	depMap := map[string]model.Dependency{}

	for _, relDir := range dirList {
		absDir := snapshot.ResolvedLocalPath
		if relDir != "" {
			absDir = filepath.Join(snapshot.ResolvedLocalPath, filepath.FromSlash(relDir))
		}

		pkgs, perr := parser.ParseDir(fset, absDir, nil, parser.ParseComments)
		if perr != nil {
			continue
		}

		for pkgName, pkg := range pkgs {
			for filePath, file := range pkg.Files {
				relPath, _ := filepath.Rel(snapshot.ResolvedLocalPath, filePath)
				relPath = filepath.ToSlash(relPath)

				for _, imp := range file.Imports {
					impPath := strings.Trim(imp.Path.Value, "\"")
					name := impPath
					if idx := strings.LastIndex(impPath, "/"); idx >= 0 {
						name = impPath[idx+1:]
					}
					depMap[impPath] = model.Dependency{
						Name:  name,
						Type:  dependencyType(impPath),
						Scope: dependencyScope(impPath),
						Evidence: model.EvidenceRef{
							File:      relPath,
							LineStart: fset.Position(imp.Pos()).Line,
							LineEnd:   fset.Position(imp.End()).Line,
						},
					}
				}

				ast.Inspect(file, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.CallExpr:
						ep, ok := endpointFromCall(fset, relPath, pkgName, node)
						if ok {
							endpointMap[ep.ID] = ep
						}
					case *ast.TypeSpec:
						if s, ok := node.Type.(*ast.StructType); ok && ast.IsExported(node.Name.Name) {
							mt := model.ModelType{
								Name: node.Name.Name,
								Kind: "struct",
								Evidence: model.EvidenceRef{
									File:      relPath,
									LineStart: fset.Position(node.Pos()).Line,
									LineEnd:   fset.Position(node.End()).Line,
									Symbol:    node.Name.Name,
								},
							}
							for _, field := range s.Fields.List {
								typeName := exprString(field.Type)
								tag := ""
								if field.Tag != nil {
									tag = strings.Trim(field.Tag.Value, "`")
								}
								jsonTag := structTagValue(tag, "json")
								validateTag := structTagValue(tag, "validate")
								bindingTag := structTagValue(tag, "binding")

								if len(field.Names) == 0 {
									mt.Fields = append(mt.Fields, model.TypeField{
										Name:     typeName,
										Type:     typeName,
										JSONTag:  jsonTag,
										Validate: validateTag,
										Binding:  bindingTag,
									})
									continue
								}
								for _, name := range field.Names {
									mt.Fields = append(mt.Fields, model.TypeField{
										Name:       name.Name,
										Type:       typeName,
										JSONTag:    jsonTag,
										Validate:   validateTag,
										Binding:    bindingTag,
										IsRequired: strings.Contains(validateTag, "required") || strings.Contains(bindingTag, "required"),
									})
								}
							}
							typeMap[mt.Name] = mt
						}
					}
					return true
				})
			}
		}
	}

	for _, ep := range endpointMap {
		endpoints = append(endpoints, ep)
	}
	for _, mt := range typeMap {
		types = append(types, mt)
	}
	for _, dep := range depMap {
		deps = append(deps, dep)
	}

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path == endpoints[j].Path {
			return endpoints[i].Method < endpoints[j].Method
		}
		return endpoints[i].Path < endpoints[j].Path
	})
	sort.Slice(types, func(i, j int) bool { return types[i].Name < types[j].Name })
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })
	return endpoints, types, deps, nil
}

func endpointFromCall(fset *token.FileSet, relPath, pkgName string, c *ast.CallExpr) (model.Endpoint, bool) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return model.Endpoint{}, false
	}

	method := strings.ToUpper(sel.Sel.Name)
	if httpMethods[method] && len(c.Args) >= 2 {
		path, ok := literalString(c.Args[0])
		if !ok {
			return model.Endpoint{}, false
		}
		handler := exprString(c.Args[1])
		ep := model.Endpoint{
			Method:      method,
			Path:        path,
			HandlerPkg:  pkgName,
			HandlerName: handler,
			Evidence: model.EvidenceRef{
				File:      relPath,
				LineStart: fset.Position(c.Pos()).Line,
				LineEnd:   fset.Position(c.End()).Line,
			},
		}
		ep.ID = common.SHA256String(ep.Method + ":" + ep.Path)
		return ep, true
	}

	if (sel.Sel.Name == "HandleFunc" || sel.Sel.Name == "Handle") && len(c.Args) >= 2 {
		path, ok := literalString(c.Args[0])
		if !ok {
			return model.Endpoint{}, false
		}
		handler := exprString(c.Args[1])
		ep := model.Endpoint{
			Method:      "ANY",
			Path:        path,
			HandlerPkg:  pkgName,
			HandlerName: handler,
			Evidence: model.EvidenceRef{
				File:      relPath,
				LineStart: fset.Position(c.Pos()).Line,
				LineEnd:   fset.Position(c.End()).Line,
			},
		}
		ep.ID = common.SHA256String(ep.Method + ":" + ep.Path)
		return ep, true
	}

	return model.Endpoint{}, false
}

func literalString(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	case *ast.ArrayType:
		return "[]" + exprString(v.Elt)
	case *ast.MapType:
		return "map[" + exprString(v.Key) + "]" + exprString(v.Value)
	case *ast.FuncLit:
		return "func_literal"
	default:
		return ""
	}
}

func structTagValue(tag, key string) string {
	parts := strings.Split(tag, " ")
	prefix := key + ":\""
	for _, part := range parts {
		if strings.HasPrefix(part, prefix) && strings.HasSuffix(part, "\"") {
			return strings.TrimSuffix(strings.TrimPrefix(part, prefix), "\"")
		}
	}
	return ""
}

func dependencyType(path string) string {
	switch {
	case strings.Contains(path, "database/sql"), strings.Contains(path, "gorm"), strings.Contains(path, "pgx"), strings.Contains(path, "mongo"):
		return "database"
	case strings.Contains(path, "http"), strings.Contains(path, "grpc"), strings.Contains(path, "resty"):
		return "external_api"
	case strings.Contains(path, "kafka"), strings.Contains(path, "nats"), strings.Contains(path, "amqp"):
		return "queue"
	default:
		return "library"
	}
}

func dependencyScope(path string) string {
	if strings.HasPrefix(path, "github.com/") || strings.HasPrefix(path, "golang.org/") || strings.HasPrefix(path, "gopkg.in/") {
		return "external"
	}
	return "internal"
}
