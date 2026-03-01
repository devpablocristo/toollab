package domain

// ASTRefType classifies the kind of AST reference.
type ASTRefType string

const (
	ASTRefHandler    ASTRefType = "handler"
	ASTRefMiddleware ASTRefType = "middleware"
	ASTRefRouteGroup ASTRefType = "route_group"
	ASTRefDTO        ASTRefType = "dto"
	ASTRefPattern    ASTRefType = "code_pattern"
)

// ASTRef is the universal reference to an AST element. Same format everywhere.
type ASTRef struct {
	Type     ASTRefType   `json:"type"`
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	Location ASTLocation  `json:"location"`
	Extra    ASTRefExtra  `json:"extra,omitempty"`
}

type ASTLocation struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Package string `json:"package,omitempty"`
}

type ASTRefExtra struct {
	Symbol         string `json:"symbol,omitempty"`
	GroupPrefix    string `json:"group_prefix,omitempty"`
	MiddlewareName string `json:"middleware_name,omitempty"`
	DTOName        string `json:"dto_name,omitempty"`
	PatternName    string `json:"pattern_name,omitempty"`
}

// EndpointEntry is a single endpoint in the canonical catalog.
type EndpointEntry struct {
	EndpointID  string   `json:"endpoint_id"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	HandlerRef  *ASTRef  `json:"handler_ref,omitempty"`
	Middlewares []ASTRef `json:"middlewares,omitempty"`
	GroupRef    *ASTRef  `json:"group_ref,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// RouterGraph represents the hierarchical route structure.
type RouterGraph struct {
	Groups []RouteGroup `json:"groups"`
}

type RouteGroup struct {
	GroupID     string       `json:"group_id"`
	Prefix      string       `json:"prefix"`
	Middlewares []ASTRef     `json:"middlewares,omitempty"`
	Endpoints   []string     `json:"endpoint_ids,omitempty"`
	Children    []RouteGroup `json:"children,omitempty"`
	ASTRef      *ASTRef      `json:"ast_ref,omitempty"`
}

// ASTEntity is a discovered code entity (handler function, DTO struct, etc.)
type ASTEntity struct {
	ASTRef     ASTRef   `json:"ast_ref"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Fields     []string `json:"fields,omitempty"`
	UsedBy     []string `json:"used_by,omitempty"`
}

// ASTCodePattern is a neutral static observation (NOT a vulnerability judgment).
type ASTCodePattern struct {
	PatternID      string   `json:"pattern_id"`
	Pattern        string   `json:"pattern"`
	Description    string   `json:"description"`
	ASTRef         ASTRef   `json:"ast_ref"`
	ContextSnippet string   `json:"context_snippet"`
	Tags           []string `json:"tags,omitempty"`
}

// EndpointCatalog is the canonical list of endpoints from AST.
type EndpointCatalog struct {
	SchemaVersion string          `json:"schema_version"`
	Framework     string          `json:"framework"`
	Endpoints     []EndpointEntry `json:"endpoints"`
	TotalCount    int             `json:"total_count"`
	Confidence    float64         `json:"confidence"`
	Gaps          []string        `json:"gaps,omitempty"`
}
