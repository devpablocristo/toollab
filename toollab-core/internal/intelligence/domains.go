package intelligence

import (
	"strings"

	d "toollab-core/internal/pipeline/usecases/domain"
)

// classifyDomain assigns each endpoint to a domain using deterministic rules.
func classifyDomain(ep d.EndpointEntry, graph *d.RouterGraph, entities []d.ASTEntity) (name, basis string) {
	path := ep.Path

	if isOpsPath(path) {
		return "ops", "special_path"
	}
	if strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/api/admin") {
		return "admin", "special_path"
	}
	if strings.HasPrefix(path, "/debug") || strings.HasPrefix(path, "/pprof") {
		return "debug", "special_path"
	}

	if graph != nil && ep.GroupRef != nil {
		if g := findGroup(graph.Groups, ep.GroupRef.ID); g != nil {
			name := domainFromPrefix(g.Prefix)
			if name != "" {
				return name, "router_group"
			}
		}
	}

	seg := firstContentSegment(path)
	if seg != "" {
		return seg, "path_prefix"
	}

	if ep.HandlerRef != nil {
		hint := packageHint(ep.HandlerRef, entities)
		if hint != "" {
			return hint, "package_hint"
		}
	}

	return "unknown", "fallback"
}

func isOpsPath(p string) bool {
	ops := []string{"/health", "/healthz", "/readyz", "/metrics", "/status", "/readiness", "/liveness", "/openapi", "/swagger", "/docs"}
	for _, o := range ops {
		if p == o || strings.HasPrefix(p, o+"/") || strings.HasPrefix(p, o+".") {
			return true
		}
	}
	return false
}

func findGroup(groups []d.RouteGroup, id string) *d.RouteGroup {
	for i := range groups {
		if groups[i].GroupID == id {
			return &groups[i]
		}
		if found := findGroup(groups[i].Children, id); found != nil {
			return found
		}
	}
	return nil
}

func domainFromPrefix(prefix string) string {
	prefix = strings.Trim(prefix, "/")
	parts := strings.Split(prefix, "/")
	for _, p := range parts {
		if isVersionSegment(p) || p == "api" || p == "" {
			continue
		}
		return strings.ToLower(p)
	}
	return ""
}

func firstContentSegment(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if isVersionSegment(p) || p == "api" || p == "" {
			continue
		}
		if strings.HasPrefix(p, "{") || strings.HasPrefix(p, ":") {
			continue
		}
		return strings.ToLower(p)
	}
	return ""
}

func isVersionSegment(s string) bool {
	s = strings.ToLower(s)
	return s == "v1" || s == "v2" || s == "v3" || s == "v4" ||
		strings.HasPrefix(s, "v1.") || strings.HasPrefix(s, "v2.")
}

func packageHint(ref *d.ASTRef, entities []d.ASTEntity) string {
	if ref == nil {
		return ""
	}
	pkg := strings.ToLower(ref.Location.Package)
	keywords := []string{"billing", "auth", "users", "user", "orders", "order",
		"admin", "payment", "product", "catalog", "inventory", "notification"}
	for _, kw := range keywords {
		if strings.Contains(pkg, kw) {
			return kw
		}
	}
	return ""
}
