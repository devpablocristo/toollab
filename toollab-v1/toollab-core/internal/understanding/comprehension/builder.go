package comprehension

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"toollab-core/internal/contract"
	"toollab-core/internal/coverage"
	"toollab-core/internal/discovery"
	"toollab-core/internal/evidence"
	"toollab-core/internal/scenario"
	"toollab-core/internal/security"
	mapmodel "toollab-core/internal/understanding/map"
)

// Build creates a comprehension report. desc may be nil when the adapter
// does not expose the "description" capability — in that case, all
// identity/model/flow data is inferred heuristically from traffic.
func Build(bundle *evidence.Bundle, scn *scenario.Scenario, systemMap *mapmodel.SystemMap, desc *discovery.ServiceDescription) *Report {
	r := &Report{SchemaVersion: 2}
	if bundle != nil {
		r.RunID = bundle.Metadata.RunID
	}

	if desc != nil {
		r.DataSource = "declared+observed"
	} else {
		r.DataSource = "observed"
	}

	r.Identity = buildIdentity(bundle, scn, systemMap, desc)
	r.Architecture = buildArchitecture(bundle, scn, systemMap, desc)
	r.Dependencies = buildDependencies(desc)
	r.Models = inferModels(bundle, systemMap, desc)
	r.AllFlows = buildAllFlows(bundle, systemMap, desc)
	r.Behavior = analyzeBehavior(bundle)
	r.Performance = analyzePerformance(bundle)

	secReport := security.Audit(bundle)
	r.Security = buildSecuritySummary(secReport)

	contractReport := contract.Validate(bundle)
	r.ContractQuality = buildContractQuality(contractReport)

	var covReport *coverage.CoverageReport
	if scn != nil {
		covReport = coverage.Analyze(scn, bundle)
	}

	r.Verdict = buildVerdict(bundle, secReport, contractReport, covReport)
	r.MaturityScore = calculateMaturity(r)
	r.MaturityGrade = maturityGrade(r.MaturityScore)

	return r
}

func buildIdentity(bundle *evidence.Bundle, scn *scenario.Scenario, systemMap *mapmodel.SystemMap, desc *discovery.ServiceDescription) Identity {
	id := Identity{
		Name:    "Servicio desconocido",
		Version: "desconocida",
		APIType: "REST",
	}

	if systemMap != nil {
		if systemMap.ServiceIdentity.Name != "" && systemMap.ServiceIdentity.Name != "unknown-service" {
			id.Name = systemMap.ServiceIdentity.Name
		}
		if systemMap.ServiceIdentity.Version != "" && systemMap.ServiceIdentity.Version != "unknown" {
			id.Version = systemMap.ServiceIdentity.Version
		}
	}

	if desc != nil {
		if desc.Purpose != "" {
			id.Purpose = desc.Purpose
		}
		if desc.Domain != "" {
			id.Domain = desc.Domain
		}
		if desc.Consumers != "" {
			id.Consumers = desc.Consumers
		}
	}

	if id.Consumers == "" && scn != nil && scn.Target.BaseURL != "" {
		id.Consumers = fmt.Sprintf("Consumidores del API en %s", scn.Target.BaseURL)
	}
	if id.Domain == "" {
		id.Domain = inferDomain(bundle, systemMap)
	}
	if id.Purpose == "" {
		id.Purpose = inferPurpose(bundle, systemMap)
	}
	return id
}

func inferDomain(bundle *evidence.Bundle, systemMap *mapmodel.SystemMap) string {
	endpoints := collectEndpointPaths(bundle, systemMap)
	domainKeywords := map[string]string{
		"user":    "Gestión de usuarios",
		"auth":    "Autenticación y autorización",
		"payment": "Pagos y transacciones",
		"order":   "Gestión de pedidos",
		"product": "Catálogo de productos",
		"admin":   "Administración",
		"policy":  "Políticas y reglas",
		"tool":    "Herramientas y utilidades",
		"agent":   "Agentes y automatización",
		"world":   "Estado del sistema",
		"config":  "Configuración",
	}

	found := map[string]bool{}
	for _, path := range endpoints {
		pl := strings.ToLower(path)
		for keyword, domain := range domainKeywords {
			if strings.Contains(pl, keyword) {
				found[domain] = true
			}
		}
	}

	if len(found) == 0 {
		return "Dominio general"
	}
	domains := make([]string, 0, len(found))
	for d := range found {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	return strings.Join(domains, ", ")
}

func inferPurpose(bundle *evidence.Bundle, systemMap *mapmodel.SystemMap) string {
	endpoints := collectEndpointPaths(bundle, systemMap)
	methods := map[string]int{}
	for _, ep := range endpoints {
		parts := strings.SplitN(ep, " ", 2)
		if len(parts) == 2 {
			methods[parts[0]]++
		}
	}

	total := len(endpoints)
	if total == 0 {
		return "No se pudo determinar el propósito del servicio."
	}

	gets := methods["GET"]
	posts := methods["POST"]
	puts := methods["PUT"]
	deletes := methods["DELETE"]

	readHeavy := float64(gets) / float64(total) > 0.6
	writeHeavy := float64(posts+puts) / float64(total) > 0.5

	if readHeavy {
		return fmt.Sprintf("API orientada a consultas con %d endpoints. Proporciona datos a sus consumidores.", total)
	}
	if writeHeavy {
		return fmt.Sprintf("API orientada a operaciones con %d endpoints. Permite crear y modificar recursos.", total)
	}
	return fmt.Sprintf("API mixta (lectura/escritura) con %d endpoints, %d GET, %d POST, %d PUT, %d DELETE.",
		total, gets, posts, puts, deletes)
}

func buildDependencies(desc *discovery.ServiceDescription) []ExternalDependency {
	if desc == nil || len(desc.Dependencies) == 0 {
		return nil
	}
	deps := make([]ExternalDependency, 0, len(desc.Dependencies))
	for _, d := range desc.Dependencies {
		deps = append(deps, ExternalDependency{
			Name:        d.Name,
			Type:        d.Type,
			Description: d.Description,
			Required:    d.Required,
		})
	}
	return deps
}

func buildArchitecture(bundle *evidence.Bundle, scn *scenario.Scenario, systemMap *mapmodel.SystemMap, desc *discovery.ServiceDescription) Architecture {
	arch := Architecture{
		Type:       "REST API",
		DataFormat: "JSON",
		AuthType:   "Ninguno detectado",
	}

	if scn != nil {
		switch scn.Target.Auth.Type {
		case "bearer":
			arch.AuthType = "Bearer Token"
		case "basic":
			arch.AuthType = "Basic Auth"
		case "api_key":
			arch.AuthType = fmt.Sprintf("API Key (en %s, nombre: %s)", scn.Target.Auth.In, scn.Target.Auth.Name)
		}
	}

	endpoints := collectEndpointPaths(bundle, systemMap)
	arch.TotalEndpoints = len(endpoints)

	for _, ep := range endpoints {
		parts := strings.SplitN(ep, " ", 2)
		if len(parts) == 2 {
			path := parts[1]
			if strings.Contains(path, "/v1/") || strings.Contains(path, "/v2/") || strings.Contains(path, "/api/v") {
				arch.HasVersioning = true
			}
			if strings.Contains(path, "openapi") || strings.Contains(path, "swagger") {
				arch.HasOpenAPI = true
			}
		}
	}

	arch.ResourceCount = len(inferResources(endpoints))
	return arch
}

func inferResources(endpoints []string) []string {
	resources := map[string]bool{}
	for _, ep := range endpoints {
		parts := strings.SplitN(ep, " ", 2)
		if len(parts) != 2 {
			continue
		}
		segments := strings.Split(strings.Trim(parts[1], "/"), "/")
		for _, seg := range segments {
			if seg == "" || seg == "v1" || seg == "v2" || seg == "api" {
				continue
			}
			if strings.HasPrefix(seg, "{") || strings.HasPrefix(seg, ":") {
				continue
			}
			if len(seg) > 2 {
				resources[seg] = true
			}
		}
	}
	out := make([]string, 0, len(resources))
	for r := range resources {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func inferModels(bundle *evidence.Bundle, systemMap *mapmodel.SystemMap, desc *discovery.ServiceDescription) []Model {
	modelData := map[string]*Model{}
	endpointOps := map[string][]string{}

	// Seed from declared description (authoritative source).
	if desc != nil {
		for _, dm := range desc.Models {
			m := &Model{
				Name:        dm.Name,
				Kind:        "declared",
				Description: dm.Description,
				Fields:      make([]ModelField, 0, len(dm.Fields)),
				Operations:  []string{},
			}
			for _, df := range dm.Fields {
				m.Fields = append(m.Fields, ModelField{
					Name:        df.Name,
					Type:        df.Type,
					Required:    df.Required,
					Description: df.Description,
					Example:     df.Example,
				})
			}
			for _, dr := range dm.Relations {
				m.Relations = append(m.Relations, ModelRelation{
					Target:      dr.Target,
					Type:        dr.Type,
					Description: dr.Description,
				})
			}
			modelData[strings.ToLower(dm.Name)] = m
		}
	}

	// Enrich/extend from observed traffic.
	if bundle != nil {
		for _, sample := range bundle.Samples {
			resource := extractResource(sample.Request.URL)
			if resource == "" {
				continue
			}

			key := strings.ToLower(resource)
			if _, exists := modelData[key]; !exists {
				modelData[key] = &Model{
					Name:       resource,
					Kind:       "inferred",
					Fields:     []ModelField{},
					Operations: []string{},
				}
			}
			m := modelData[key]

			op := sample.Request.Method + " " + sample.Request.URL
			if _, seen := endpointOps[key]; !seen {
				endpointOps[key] = []string{}
			}
			endpointOps[key] = append(endpointOps[key], op)

			bodyToAnalyze := sample.Response.BodyPreview
			if bodyToAnalyze == "" {
				bodyToAnalyze = sample.Request.BodyPreview
			}
			if bodyToAnalyze != "" {
				fields := extractJSONFields(bodyToAnalyze)
				existingFields := map[string]bool{}
				for _, f := range m.Fields {
					existingFields[f.Name] = true
				}
				for _, f := range fields {
					if !existingFields[f.Name] {
						m.Fields = append(m.Fields, f)
						existingFields[f.Name] = true
					}
				}
			}
		}
	}

	for resource, ops := range endpointOps {
		m := modelData[resource]
		seen := map[string]bool{}
		for _, op := range ops {
			method := strings.SplitN(op, " ", 2)[0]
			opName := methodToOperation(method)
			if !seen[opName] {
				m.Operations = append(m.Operations, opName)
				seen[opName] = true
			}
		}
		sort.Strings(m.Operations)
	}

	models := make([]Model, 0, len(modelData))
	for _, m := range modelData {
		models = append(models, *m)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	return models
}

func extractResource(url string) string {
	parts := strings.Split(strings.Trim(url, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		seg := parts[i]
		if seg == "" || seg == "v1" || seg == "v2" || seg == "api" {
			continue
		}
		if strings.HasPrefix(seg, "{") || strings.HasPrefix(seg, ":") {
			continue
		}
		if _, err := fmt.Sscanf(seg, "%x"); err == nil && len(seg) > 8 {
			continue
		}
		if len(seg) > 2 && !strings.Contains(seg, ".") {
			return seg
		}
	}
	return ""
}

func extractJSONFields(body string) []ModelField {
	body = strings.TrimSpace(body)
	var fields []ModelField

	var obj map[string]any
	if err := json.Unmarshal([]byte(body), &obj); err == nil {
		for key, val := range obj {
			f := ModelField{Name: key, Type: inferJSONType(val)}
			if val != nil {
				ex := fmt.Sprintf("%v", val)
				if len(ex) > 60 {
					ex = ex[:60] + "…"
				}
				f.Example = ex
			}
			fields = append(fields, f)
		}
	} else {
		var arr []any
		if err := json.Unmarshal([]byte(body), &arr); err == nil && len(arr) > 0 {
			if item, ok := arr[0].(map[string]any); ok {
				for key, val := range item {
					f := ModelField{Name: key, Type: inferJSONType(val)}
					if val != nil {
						ex := fmt.Sprintf("%v", val)
						if len(ex) > 60 {
							ex = ex[:60] + "…"
						}
						f.Example = ex
					}
					fields = append(fields, f)
				}
			}
		}
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields
}

func inferJSONType(val any) string {
	switch val.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func methodToOperation(method string) string {
	switch strings.ToUpper(method) {
	case "GET":
		return "Lectura"
	case "POST":
		return "Creación"
	case "PUT":
		return "Actualización completa"
	case "PATCH":
		return "Actualización parcial"
	case "DELETE":
		return "Eliminación"
	default:
		return method
	}
}

func buildAllFlows(bundle *evidence.Bundle, systemMap *mapmodel.SystemMap, desc *discovery.ServiceDescription) []FlowDetail {
	if bundle == nil {
		return []FlowDetail{}
	}

	type flowAcc struct {
		method       string
		path         string
		total        int
		totalLatency int
		errors       int
		statusCodes  map[int]int
		reqSample    string
		resSample    string
	}

	flows := map[string]*flowAcc{}
	var order []string

	for _, o := range bundle.Outcomes {
		key := o.Method + " " + o.Path
		f, exists := flows[key]
		if !exists {
			f = &flowAcc{method: o.Method, path: o.Path, statusCodes: map[int]int{}}
			flows[key] = f
			order = append(order, key)
		}
		f.total++
		f.totalLatency += o.LatencyMS
		if o.StatusCode != nil {
			f.statusCodes[*o.StatusCode]++
			if *o.StatusCode >= 400 {
				f.errors++
			}
		}
		if o.ErrorKind != "none" && o.ErrorKind != "" && o.ErrorKind != "http_status_error" {
			f.errors++
		}
	}

	sampleMap := map[string]*evidence.Sample{}
	for i := range bundle.Samples {
		s := &bundle.Samples[i]
		key := s.Request.Method + " " + s.Request.URL
		if _, exists := sampleMap[key]; !exists {
			sampleMap[key] = s
		}
	}

	var details []FlowDetail
	for i, key := range order {
		f := flows[key]
		avgLat := 0
		if f.total > 0 {
			avgLat = f.totalLatency / f.total
		}
		errRate := 0.0
		if f.total > 0 {
			errRate = float64(f.errors) / float64(f.total)
		}

		codes := make([]int, 0, len(f.statusCodes))
		for code := range f.statusCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)

		flowDesc := describeEndpoint(f.method, f.path)
		flowCat := categorizeEndpoint(f.method, f.path)
		flowName := fmt.Sprintf("%s %s", f.method, f.path)

		if epd := findEndpointDesc(desc, f.method, f.path); epd != nil {
			if epd.Summary != "" {
				flowDesc = epd.Summary
				flowName = epd.Summary
			}
			if epd.Description != "" {
				flowDesc = epd.Description
			}
			if epd.Category != "" {
				flowCat = epd.Category
			}
		}

		detail := FlowDetail{
			ID:          fmt.Sprintf("flow_%03d", i+1),
			Name:        flowName,
			Description: flowDesc,
			Category:    flowCat,
			Steps: []FlowStep{{
				Order:       1,
				Method:      f.method,
				Path:        f.path,
				Description: flowDesc,
			}},
			StatusCodes: codes,
			AvgLatency:  avgLat,
			ErrorRate:   errRate,
		}

		if sample, ok := sampleMap[key]; ok {
			if sample.Request.BodyPreview != "" {
				detail.Payload = &PayloadDetail{
					ContentType: "application/json",
					Example:     sample.Request.BodyPreview,
					Fields:      extractFieldNames(sample.Request.BodyPreview),
				}
			}
			if sample.Response.BodyPreview != "" {
				detail.Response = &PayloadDetail{
					ContentType: "application/json",
					Example:     sample.Response.BodyPreview,
					Fields:      extractFieldNames(sample.Response.BodyPreview),
				}
			}
		}

		details = append(details, detail)
	}
	return details
}

func describeEndpoint(method, path string) string {
	resource := extractResource(method + " " + path)
	if resource == "" {
		resource = "recurso"
	}

	hasParam := strings.Contains(path, "{") || strings.Contains(path, ":")
	switch strings.ToUpper(method) {
	case "GET":
		if hasParam {
			return fmt.Sprintf("Obtener un %s específico por ID", resource)
		}
		return fmt.Sprintf("Listar todos los %s", resource)
	case "POST":
		if strings.Contains(strings.ToLower(path), "query") || strings.Contains(strings.ToLower(path), "search") {
			return fmt.Sprintf("Buscar %s con filtros", resource)
		}
		return fmt.Sprintf("Crear un nuevo %s", resource)
	case "PUT":
		return fmt.Sprintf("Actualizar completamente un %s", resource)
	case "PATCH":
		return fmt.Sprintf("Actualizar parcialmente un %s", resource)
	case "DELETE":
		return fmt.Sprintf("Eliminar un %s", resource)
	default:
		return fmt.Sprintf("%s sobre %s", method, resource)
	}
}

func categorizeEndpoint(method, path string) string {
	pl := strings.ToLower(path)
	if strings.Contains(pl, "admin") {
		return "Administración"
	}
	if strings.Contains(pl, "auth") || strings.Contains(pl, "login") || strings.Contains(pl, "token") {
		return "Autenticación"
	}
	if strings.Contains(pl, "health") || strings.Contains(pl, "status") || strings.Contains(pl, "ping") {
		return "Infraestructura"
	}
	if strings.Contains(pl, "openapi") || strings.Contains(pl, "swagger") || strings.Contains(pl, "docs") {
		return "Documentación"
	}
	return "Negocio"
}

func extractFieldNames(body string) map[string]string {
	body = strings.TrimSpace(body)
	fields := map[string]string{}

	var obj map[string]any
	if err := json.Unmarshal([]byte(body), &obj); err == nil {
		for key, val := range obj {
			fields[key] = inferJSONType(val)
		}
		return fields
	}
	var arr []any
	if err := json.Unmarshal([]byte(body), &arr); err == nil && len(arr) > 0 {
		if item, ok := arr[0].(map[string]any); ok {
			for key, val := range item {
				fields[key] = inferJSONType(val)
			}
		}
	}
	return fields
}

func analyzeBehavior(bundle *evidence.Bundle) Behavior {
	b := Behavior{
		InvalidInput:     "No evaluado",
		MissingAuth:      "No evaluado",
		NotFound:         "No evaluado",
		Duplicates:       "No evaluado",
		ErrorConsistency: "No evaluado",
		Idempotency:      "No evaluado",
	}
	if bundle == nil {
		return b
	}

	has4xx := false
	has401 := false
	has404 := false
	errorBodies := map[string]int{}

	for _, sample := range bundle.Samples {
		if sample.Response.StatusCode == nil {
			continue
		}
		sc := *sample.Response.StatusCode
		if sc >= 400 && sc < 500 {
			has4xx = true
			if sample.Response.BodyPreview != "" {
				var obj map[string]any
				if json.Unmarshal([]byte(sample.Response.BodyPreview), &obj) == nil {
					for k := range obj {
						errorBodies[k]++
					}
				}
			}
		}
		if sc == 401 || sc == 403 {
			has401 = true
		}
		if sc == 404 {
			has404 = true
		}
	}

	if has4xx {
		b.InvalidInput = "El servicio rechaza datos inválidos con respuestas 4xx."
	}
	if has401 {
		b.MissingAuth = "El servicio protege endpoints con autenticación (devuelve 401/403)."
	} else {
		b.MissingAuth = "No se observaron respuestas 401/403. Verificar si la autenticación funciona correctamente."
	}
	if has404 {
		b.NotFound = "El servicio devuelve 404 para recursos inexistentes."
	}

	if len(errorBodies) > 0 {
		hasError := errorBodies["error"] > 0 || errorBodies["message"] > 0 || errorBodies["detail"] > 0
		if hasError {
			b.ErrorConsistency = "Las respuestas de error incluyen campos descriptivos (error/message)."
		} else {
			b.ErrorConsistency = "Las respuestas de error no siguen un formato estándar con campo error/message."
		}
	}

	return b
}

func analyzePerformance(bundle *evidence.Bundle) Performance {
	p := Performance{}
	if bundle == nil {
		return p
	}

	p.OverallP50 = bundle.Stats.P50MS
	p.OverallP95 = bundle.Stats.P95MS
	p.OverallP99 = bundle.Stats.P99MS

	type epStats struct {
		endpoint string
		total    int
		totalMs  int
	}
	eps := map[string]*epStats{}
	var order []string
	for _, o := range bundle.Outcomes {
		key := o.Method + " " + o.Path
		ep, exists := eps[key]
		if !exists {
			ep = &epStats{endpoint: key}
			eps[key] = ep
			order = append(order, key)
		}
		ep.total++
		ep.totalMs += o.LatencyMS
	}

	type ranked struct {
		endpoint string
		avgMs    int
		reqs     int
	}
	var ranking []ranked
	for _, key := range order {
		ep := eps[key]
		avg := 0
		if ep.total > 0 {
			avg = ep.totalMs / ep.total
		}
		ranking = append(ranking, ranked{endpoint: key, avgMs: avg, reqs: ep.total})
	}

	sort.Slice(ranking, func(i, j int) bool { return ranking[i].avgMs < ranking[j].avgMs })

	limit := 5
	if len(ranking) < limit {
		limit = len(ranking)
	}
	for _, r := range ranking[:limit] {
		p.FastEndpoints = append(p.FastEndpoints, EndpointPerf{Endpoint: r.endpoint, AvgMs: r.avgMs, Requests: r.reqs})
	}

	sort.Slice(ranking, func(i, j int) bool { return ranking[i].avgMs > ranking[j].avgMs })
	limit = 5
	if len(ranking) < limit {
		limit = len(ranking)
	}
	for _, r := range ranking[:limit] {
		p.SlowEndpoints = append(p.SlowEndpoints, EndpointPerf{Endpoint: r.endpoint, AvgMs: r.avgMs, Requests: r.reqs})
	}

	for _, r := range ranking {
		if r.avgMs > bundle.Stats.P95MS {
			p.Bottlenecks = append(p.Bottlenecks, fmt.Sprintf("%s (avg %dms, supera P95 de %dms)", r.endpoint, r.avgMs, bundle.Stats.P95MS))
		}
	}

	return p
}

func buildSecuritySummary(report *security.AuditReport) SecuritySummary {
	s := SecuritySummary{
		Grade:     report.Grade,
		Score:     report.Score,
		Risks:     []string{},
		Strengths: []string{},
	}

	for _, f := range report.Findings {
		if f.Severity == "critical" || f.Severity == "high" {
			s.Risks = append(s.Risks, f.Title)
		}
	}

	if report.Summary.Critical == 0 {
		s.Strengths = append(s.Strengths, "Sin vulnerabilidades críticas")
	}
	if report.Summary.Total == 0 {
		s.Strengths = append(s.Strengths, "Sin hallazgos de seguridad")
	}
	return s
}

func buildContractQuality(report *contract.ContractReport) ContractQuality {
	cq := ContractQuality{
		ComplianceRate: report.ComplianceRate,
		Issues:         []string{},
		Strengths:      []string{},
	}

	for _, v := range report.Violations {
		cq.Issues = append(cq.Issues, v.Description)
	}

	if report.Compliant {
		cq.Strengths = append(cq.Strengths, "Todas las respuestas cumplen con el contrato")
	}
	return cq
}

func buildVerdict(bundle *evidence.Bundle, sec *security.AuditReport, con *contract.ContractReport, cov *coverage.CoverageReport) Verdict {
	v := Verdict{
		ProductionReady: true,
		Confidence:      "alta",
		Risks:           []string{},
		Improvements:    []string{},
		MissingFeatures: []string{},
	}

	if bundle != nil && bundle.Assertions.Overall == "FAIL" {
		v.ProductionReady = false
		v.Risks = append(v.Risks, "Las assertions del test no pasaron")
	}

	if bundle != nil && bundle.Stats.ErrorRate > 0.1 {
		v.ProductionReady = false
		v.Risks = append(v.Risks, fmt.Sprintf("Tasa de error alta: %.1f%%", bundle.Stats.ErrorRate*100))
	}

	if sec.Summary.Critical > 0 {
		v.ProductionReady = false
		v.Risks = append(v.Risks, fmt.Sprintf("%d vulnerabilidades críticas de seguridad", sec.Summary.Critical))
	}
	if sec.Summary.High > 0 {
		v.Risks = append(v.Risks, fmt.Sprintf("%d hallazgos de seguridad de severidad alta", sec.Summary.High))
	}

	if !con.Compliant {
		v.Improvements = append(v.Improvements, fmt.Sprintf("Resolver %d violaciones de contrato", con.TotalViolations))
	}

	if cov != nil && cov.CoverageRate < 1.0 {
		untested := cov.TotalEndpoints - cov.TestedEndpoints
		if untested > 0 {
			v.MissingFeatures = append(v.MissingFeatures, fmt.Sprintf("%d endpoints declarados no fueron testeados", untested))
		}
	}

	if bundle != nil && bundle.Stats.P95MS > 1000 {
		v.Improvements = append(v.Improvements, fmt.Sprintf("Optimizar latencia: P95 actual es %dms (recomendado <1000ms)", bundle.Stats.P95MS))
	}

	if len(v.Risks) > 2 {
		v.Confidence = "baja"
	} else if len(v.Risks) > 0 {
		v.Confidence = "media"
	}

	return v
}

func calculateMaturity(r *Report) int {
	score := 100

	if !r.Verdict.ProductionReady {
		score -= 30
	}
	score -= len(r.Verdict.Risks) * 10
	score -= len(r.Verdict.Improvements) * 5

	if r.Security.Score < 70 {
		score -= 15
	}
	if r.ContractQuality.ComplianceRate < 0.9 {
		score -= 10
	}
	if r.Performance.OverallP95 > 2000 {
		score -= 10
	}

	if score < 0 {
		score = 0
	}
	return score
}

func maturityGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func findEndpointDesc(desc *discovery.ServiceDescription, method, path string) *discovery.EndpointDescription {
	if desc == nil {
		return nil
	}
	for i := range desc.EndpointDescriptions {
		ed := &desc.EndpointDescriptions[i]
		if strings.EqualFold(ed.Method, method) && ed.Path == path {
			return ed
		}
	}
	return nil
}

func collectEndpointPaths(bundle *evidence.Bundle, systemMap *mapmodel.SystemMap) []string {
	seen := map[string]bool{}
	var out []string

	if systemMap != nil {
		for _, ep := range systemMap.Endpoints {
			key := ep.Method + " " + ep.Path
			if !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	if bundle != nil {
		for _, o := range bundle.Outcomes {
			key := o.Method + " " + o.Path
			if !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	sort.Strings(out)
	return out
}
