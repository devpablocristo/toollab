package exports

import (
	"encoding/json"
	"fmt"
	"strings"

	d "toollab-core/internal/pipeline/usecases/domain"
)

// GeneratePostmanCollection creates an enriched Postman collection from endpoint intelligence.
func GeneratePostmanCollection(dossier *d.DossierV2Full, intel *d.EndpointIntelligence) []byte {
	type postmanReq struct {
		Method      string       `json:"method"`
		Header      []postmanKV  `json:"header"`
		URL         postmanURL   `json:"url"`
		Body        *postmanBody `json:"body,omitempty"`
		Description string       `json:"description,omitempty"`
	}
	type postmanItem struct {
		Name    string     `json:"name"`
		Request postmanReq `json:"request"`
	}
	type postmanFolder struct {
		Name  string        `json:"name"`
		Items []postmanItem `json:"item"`
	}
	type postmanCollection struct {
		Info map[string]string `json:"info"`
		Item []postmanFolder   `json:"item"`
	}

	col := postmanCollection{
		Info: map[string]string{
			"name":   "ToolLab v2 — " + dossier.TargetProfile.BaseURL,
			"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
	}

	if intel != nil {
		for _, dom := range intel.Domains {
			folder := postmanFolder{Name: dom.DomainName}
			for _, ep := range dom.Endpoints {
				for _, cmd := range ep.HowToQuery.ReadyCommands {
					if cmd.Kind != "curl" {
						continue
					}
					item := postmanItem{
						Name: fmt.Sprintf("[%s] %s %s", cmd.Name, ep.Method, ep.PathTemplate),
						Request: postmanReq{
							Method:      ep.Method,
							Description: ep.WhatItDoes.Summary,
							URL: postmanURL{
								Raw:  strings.TrimRight(intel.BaseURL, "/") + ep.PathTemplate,
								Host: []string{intel.BaseURL},
								Path: strings.Split(strings.Trim(ep.PathTemplate, "/"), "/"),
							},
						},
					}

					for _, h := range ep.Inputs.Headers {
						val := ""
						if len(h.ObservedValues) > 0 {
							val = h.ObservedValues[0]
						}
						item.Request.Header = append(item.Request.Header, postmanKV{Key: h.Name, Value: val})
					}

					if ep.Auth.Required == "yes" || ep.Auth.Required == "unknown" {
						if ep.Auth.Mechanism == "api_key" {
							item.Request.Header = append(item.Request.Header, postmanKV{Key: "X-API-Key", Value: "{{api_key}}"})
						} else {
							item.Request.Header = append(item.Request.Header, postmanKV{Key: "Authorization", Value: "Bearer {{token}}"})
						}
					}

					if ep.Inputs.Body != nil && ep.Inputs.Body.ContentType != "" {
						item.Request.Header = append(item.Request.Header, postmanKV{Key: "Content-Type", Value: ep.Inputs.Body.ContentType})
						if len(ep.Inputs.Body.RequiredFields) > 0 {
							var fields []string
							for _, f := range ep.Inputs.Body.RequiredFields {
								fields = append(fields, fmt.Sprintf("\"%s\": \"\"", f.FieldPath))
							}
							item.Request.Body = &postmanBody{
								Mode: "raw",
								Raw:  "{" + strings.Join(fields, ", ") + "}",
								Options: &postmanBodyOpts{
									Raw: postmanRawLang{Language: "json"},
								},
							}
						}
					}

					folder.Items = append(folder.Items, item)
				}

				for _, v := range ep.HowToQuery.QueryVariants {
					folder.Items = append(folder.Items, postmanItem{
						Name: fmt.Sprintf("[variant:%s] %s %s", v.VariantName, ep.Method, ep.PathTemplate),
						Request: postmanReq{
							Method:      ep.Method,
							Description: v.Description,
							URL:         parsePostmanURL(strings.TrimRight(intel.BaseURL, "/") + ep.PathTemplate),
						},
					})
				}
			}
			col.Item = append(col.Item, folder)
		}
	} else {
		folder := postmanFolder{Name: "All Endpoints"}
		seen := make(map[string]bool)
		for _, sample := range dossier.Runtime.EvidenceSamples {
			key := sample.Request.Method + " " + sample.Request.URL
			if seen[key] {
				continue
			}
			seen[key] = true

			var headers []postmanKV
			for k, v := range sample.Request.Headers {
				headers = append(headers, postmanKV{Key: k, Value: v})
			}

			item := postmanItem{
				Name: sample.Request.Method + " " + sample.Request.Path,
				Request: postmanReq{
					Method: sample.Request.Method,
					Header: headers,
					URL:    parsePostmanURL(sample.Request.URL),
				},
			}

			if sample.Request.Body != "" {
				item.Request.Body = &postmanBody{
					Mode: "raw",
					Raw:  sample.Request.Body,
					Options: &postmanBodyOpts{
						Raw: postmanRawLang{Language: "json"},
					},
				}
			}
			folder.Items = append(folder.Items, item)
		}
		col.Item = append(col.Item, folder)
	}

	data, _ := json.MarshalIndent(col, "", "  ")
	return data
}

type postmanKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type postmanURL struct {
	Raw  string   `json:"raw"`
	Host []string `json:"host"`
	Path []string `json:"path"`
}

type postmanBody struct {
	Mode    string           `json:"mode"`
	Raw     string           `json:"raw"`
	Options *postmanBodyOpts `json:"options,omitempty"`
}

type postmanBodyOpts struct {
	Raw postmanRawLang `json:"raw"`
}

type postmanRawLang struct {
	Language string `json:"language"`
}

func parsePostmanURL(rawURL string) postmanURL {
	pu := postmanURL{Raw: rawURL}
	idx := strings.Index(rawURL, "://")
	rest := rawURL
	if idx >= 0 {
		rest = rawURL[idx+3:]
	}
	pathIdx := strings.Index(rest, "/")
	if pathIdx >= 0 {
		pu.Host = []string{rest[:pathIdx]}
		pu.Path = strings.Split(strings.Trim(rest[pathIdx:], "/"), "/")
	} else {
		pu.Host = []string{rest}
	}
	return pu
}

// GenerateCurlBook creates reproducible curl commands, enriched with intelligence when available.
func GenerateCurlBook(dossier *d.DossierV2Full, intel *d.EndpointIntelligence) []byte {
	type curlEntry struct {
		Name       string   `json:"name"`
		Command    string   `json:"command"`
		Tags       []string `json:"tags,omitempty"`
		BasedOn    string   `json:"based_on,omitempty"`
		Domain     string   `json:"domain,omitempty"`
		EndpointID string   `json:"endpoint_id,omitempty"`
	}

	var entries []curlEntry

	if intel != nil {
		for _, dom := range intel.Domains {
			for _, ep := range dom.Endpoints {
				for _, cmd := range ep.HowToQuery.ReadyCommands {
					entries = append(entries, curlEntry{
						Name:       fmt.Sprintf("[%s] %s %s", cmd.Name, ep.Method, ep.PathTemplate),
						Command:    cmd.Command,
						Tags:       ep.Tags,
						BasedOn:    cmd.BasedOn,
						Domain:     dom.DomainName,
						EndpointID: ep.EndpointID,
					})
				}
			}
		}
	} else {
		seen := make(map[string]bool)
		for _, sample := range dossier.Runtime.EvidenceSamples {
			key := sample.Request.Method + " " + sample.Request.URL
			if seen[key] {
				continue
			}
			seen[key] = true

			cmd := "curl"
			if sample.Request.Method != "GET" {
				cmd += " -X " + sample.Request.Method
			}
			for k, v := range sample.Request.Headers {
				cmd += fmt.Sprintf(` -H '%s: %s'`, k, v)
			}
			if sample.Request.Body != "" {
				body := sample.Request.Body
				if len(body) > 500 {
					body = body[:500] + "..."
				}
				cmd += fmt.Sprintf(` -d '%s'`, strings.ReplaceAll(body, "'", "'\\''"))
			}
			cmd += " '" + sample.Request.URL + "'"

			entries = append(entries, curlEntry{
				Name:    sample.Request.Method + " " + sample.Request.Path,
				Command: cmd,
				Tags:    sample.Tags,
			})
		}
	}

	data, _ := json.MarshalIndent(entries, "", "  ")
	return data
}

// GenerateOpenAPIInferred creates an enriched OpenAPI spec using intelligence data.
func GenerateOpenAPIInferred(dossier *d.DossierV2Full, intel *d.EndpointIntelligence) []byte {
	var sb strings.Builder
	sb.WriteString("openapi: 3.0.3\n")
	sb.WriteString("info:\n")
	sb.WriteString(fmt.Sprintf("  title: '%s (INFERRED by ToolLab v2)'\n", dossier.TargetProfile.BaseURL))
	sb.WriteString("  version: 'inferred'\n")
	sb.WriteString("  description: 'Auto-inferred from runtime evidence and AST analysis. Confidence varies per endpoint.'\n")
	sb.WriteString("servers:\n")
	sb.WriteString(fmt.Sprintf("  - url: '%s'\n", dossier.TargetProfile.BaseURL))

	hasAuth := false
	if intel != nil {
		for _, dom := range intel.Domains {
			for _, ep := range dom.Endpoints {
				if ep.Auth.Required == "yes" {
					hasAuth = true
					break
				}
			}
			if hasAuth {
				break
			}
		}
	}

	if hasAuth {
		sb.WriteString("components:\n")
		sb.WriteString("  securitySchemes:\n")
		sb.WriteString("    BearerAuth:\n")
		sb.WriteString("      type: http\n")
		sb.WriteString("      scheme: bearer\n")
		sb.WriteString("    ApiKeyAuth:\n")
		sb.WriteString("      type: apiKey\n")
		sb.WriteString("      in: header\n")
		sb.WriteString("      name: X-API-Key\n")
	}

	sb.WriteString("tags:\n")
	if intel != nil {
		for _, dom := range intel.Domains {
			sb.WriteString(fmt.Sprintf("  - name: '%s'\n", dom.DomainName))
			sb.WriteString(fmt.Sprintf("    description: '%s'\n", escapeSingleQuotes(dom.DomainDescription)))
		}
	}

	sb.WriteString("paths:\n")

	if intel != nil {
		type pathMethod struct {
			ep  d.IntelEndpoint
			dom string
		}
		byPath := make(map[string][]pathMethod)
		var pathOrder []string
		for _, dom := range intel.Domains {
			for _, ep := range dom.Endpoints {
				if _, exists := byPath[ep.PathTemplate]; !exists {
					pathOrder = append(pathOrder, ep.PathTemplate)
				}
				byPath[ep.PathTemplate] = append(byPath[ep.PathTemplate], pathMethod{ep: ep, dom: dom.DomainName})
			}
		}

		for _, path := range pathOrder {
			sb.WriteString(fmt.Sprintf("  '%s':\n", path))
			for _, pm := range byPath[path] {
				ep := pm.ep
				method := strings.ToLower(ep.Method)
				if method == "any" {
					continue
				}
				sb.WriteString(fmt.Sprintf("    %s:\n", method))
				sb.WriteString(fmt.Sprintf("      operationId: '%s'\n", ep.OperationID))
				sb.WriteString(fmt.Sprintf("      summary: '%s'\n", escapeSingleQuotes(ep.WhatItDoes.Summary)))
				sb.WriteString(fmt.Sprintf("      tags: ['%s']\n", pm.dom))

				if ep.Auth.Required == "yes" {
					if ep.Auth.Mechanism == "api_key" {
						sb.WriteString("      security:\n        - ApiKeyAuth: []\n")
					} else {
						sb.WriteString("      security:\n        - BearerAuth: []\n")
					}
				}

				if len(ep.Inputs.PathParams) > 0 || len(ep.Inputs.QueryParams) > 0 {
					sb.WriteString("      parameters:\n")
					for _, pp := range ep.Inputs.PathParams {
						sb.WriteString(fmt.Sprintf("        - name: '%s'\n          in: path\n          required: true\n          schema:\n            type: '%s'\n", pp.Name, pp.Type))
					}
					for _, qp := range ep.Inputs.QueryParams {
						sb.WriteString(fmt.Sprintf("        - name: '%s'\n          in: query\n          schema:\n            type: '%s'\n", qp.Name, qp.Type))
					}
				}

				sb.WriteString("      responses:\n")
				if len(ep.Outputs.Responses) > 0 {
					for _, r := range ep.Outputs.Responses {
						sb.WriteString(fmt.Sprintf("        '%d':\n", r.Status))
						sb.WriteString(fmt.Sprintf("          description: '%s'\n", escapeSingleQuotes(r.WhatYouGet)))
					}
				} else {
					sb.WriteString("        '200':\n")
					sb.WriteString("          description: 'No response schema inferred'\n")
				}
				for _, ce := range ep.Outputs.CommonErrors {
					sb.WriteString(fmt.Sprintf("        '%d':\n", ce.Status))
					sb.WriteString(fmt.Sprintf("          description: '%s'\n", escapeSingleQuotes(ce.Meaning)))
				}
			}
		}
	} else {
		byPath := make(map[string][]d.EndpointEntry)
		for _, ep := range dossier.AST.EndpointCatalog.Endpoints {
			byPath[ep.Path] = append(byPath[ep.Path], ep)
		}

		contractMap := make(map[string]d.InferredContract)
		for _, c := range dossier.Runtime.InferredContracts {
			contractMap[c.EndpointID] = c
		}

		for path, eps := range byPath {
			sb.WriteString(fmt.Sprintf("  '%s':\n", path))
			for _, ep := range eps {
				method := strings.ToLower(ep.Method)
				if method == "any" {
					continue
				}
				sb.WriteString(fmt.Sprintf("    %s:\n", method))
				label := ""
				if ep.HandlerRef != nil {
					label = ep.HandlerRef.Label
				}
				sb.WriteString(fmt.Sprintf("      summary: '%s'\n", label))
				sb.WriteString("      responses:\n")

				contract, hasContract := contractMap[ep.EndpointID]
				if hasContract && len(contract.ResponseSchemas) > 0 {
					for _, rs := range contract.ResponseSchemas {
						sb.WriteString(fmt.Sprintf("        '%d':\n", rs.Status))
						sb.WriteString(fmt.Sprintf("          description: 'Inferred (schema_ref: %s)'\n", rs.SchemaRef))
					}
				} else {
					sb.WriteString("        '200':\n")
					sb.WriteString("          description: 'No response schema inferred'\n")
				}
			}
		}
	}

	return []byte(sb.String())
}

// GenerateOpenAPIAST creates a minimal OpenAPI spec from AST only.
func GenerateOpenAPIAST(dossier *d.DossierV2Full) []byte {
	var sb strings.Builder
	sb.WriteString("openapi: 3.0.3\n")
	sb.WriteString("info:\n")
	sb.WriteString(fmt.Sprintf("  title: '%s (AST-only by ToolLab v2)'\n", dossier.TargetProfile.BaseURL))
	sb.WriteString("  version: 'ast'\n")
	sb.WriteString("  description: 'Generated from static analysis (AST) only. No runtime evidence. All descriptions are inferred from code.'\n")
	sb.WriteString("servers:\n")
	sb.WriteString(fmt.Sprintf("  - url: '%s'\n", dossier.TargetProfile.BaseURL))

	sb.WriteString("tags:\n")
	groupNames := make(map[string]bool)
	for _, g := range dossier.AST.RouterGraph.Groups {
		name := strings.Trim(g.Prefix, "/")
		if name != "" && !groupNames[name] {
			groupNames[name] = true
			sb.WriteString(fmt.Sprintf("  - name: '%s'\n", name))
		}
	}

	sb.WriteString("paths:\n")
	for _, ep := range dossier.AST.EndpointCatalog.Endpoints {
		method := strings.ToLower(ep.Method)
		if method == "any" {
			continue
		}
		handler := ""
		if ep.HandlerRef != nil {
			handler = ep.HandlerRef.Label
		}

		opID := handler
		if opID == "" {
			opID = method + "_" + strings.ReplaceAll(strings.Trim(ep.Path, "/"), "/", "_")
		}

		sb.WriteString(fmt.Sprintf("  '%s':\n", ep.Path))
		sb.WriteString(fmt.Sprintf("    %s:\n", method))
		sb.WriteString(fmt.Sprintf("      operationId: '%s'\n", opID))
		sb.WriteString(fmt.Sprintf("      summary: '%s (inferred from code)'\n", handler))

		if len(ep.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("      tags: ['%s']\n", strings.Join(ep.Tags, "', '")))
		}

		sb.WriteString("      responses:\n")
		sb.WriteString("        '200':\n")
		sb.WriteString("          description: 'unknown'\n")
	}

	return []byte(sb.String())
}

// GenerateAuthMatrixCSV creates a CSV of the auth matrix.
func GenerateAuthMatrixCSV(matrix *d.AuthMatrix) []byte {
	var sb strings.Builder
	sb.WriteString("endpoint_id,method,path,no_auth,invalid_auth,valid_auth\n")
	for _, e := range matrix.Entries {
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
			e.EndpointID, e.Method, e.Path, e.NoAuth, e.InvalidAuth, e.ValidAuth))
	}
	return []byte(sb.String())
}

// GenerateEndpointCatalogCSV creates a CSV of the endpoint catalog.
func GenerateEndpointCatalogCSV(catalog *d.EndpointCatalog) []byte {
	var sb strings.Builder
	sb.WriteString("endpoint_id,method,path,handler,middlewares,tags\n")
	for _, ep := range catalog.Endpoints {
		handler := ""
		if ep.HandlerRef != nil {
			handler = ep.HandlerRef.Label
		}
		var mws []string
		for _, mw := range ep.Middlewares {
			mws = append(mws, mw.Label)
		}
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
			ep.EndpointID, ep.Method, ep.Path, handler, strings.Join(mws, ";"), strings.Join(ep.Tags, ";")))
	}
	return []byte(sb.String())
}

// GenerateContractMatrixCSV creates a CSV of endpoint × status × schema_ref.
func GenerateContractMatrixCSV(contracts []d.InferredContract) []byte {
	var sb strings.Builder
	sb.WriteString("endpoint_id,method,path,status,schema_ref,content_type\n")
	for _, c := range contracts {
		for _, rs := range c.ResponseSchemas {
			sb.WriteString(fmt.Sprintf("%s,%s,%s,%d,%s,%s\n",
				c.EndpointID, c.Method, c.Path, rs.Status, rs.SchemaRef, rs.ContentType))
		}
	}
	return []byte(sb.String())
}

// GenerateHotspotsCSV creates a CSV of endpoint risk hotspots.
func GenerateHotspotsCSV(hotspots []d.EndpointHotspot) []byte {
	var sb strings.Builder
	sb.WriteString("endpoint_id,method,path,risk_notes\n")
	for _, h := range hotspots {
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s\n",
			h.EndpointID, h.Method, h.Path, strings.ReplaceAll(h.RiskNotes, ",", ";")))
	}
	return []byte(sb.String())
}

func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// GenerateEnvExample creates a .env.example file with variables for the target.
func GenerateEnvExample(dossier *d.DossierV2Full) []byte {
	var sb strings.Builder
	sb.WriteString("# ToolLab - Environment Variables\n")
	sb.WriteString(fmt.Sprintf("# Generated for run %s\n\n", dossier.RunID[:12]))

	sb.WriteString(fmt.Sprintf("BASE_URL=%s\n", dossier.TargetProfile.BaseURL))

	if len(dossier.TargetProfile.AuthHints.Mechanisms) > 0 {
		mech := dossier.TargetProfile.AuthHints.Mechanisms[0]
		if strings.Contains(strings.ToLower(mech), "jwt") || strings.Contains(strings.ToLower(mech), "bearer") {
			sb.WriteString("TOKEN=your_bearer_token_here\n")
		}
		if strings.Contains(strings.ToLower(mech), "key") {
			sb.WriteString("API_KEY=your_api_key_here\n")
		}
	} else {
		sb.WriteString("# TOKEN=your_bearer_token_here\n")
		sb.WriteString("# API_KEY=your_api_key_here\n")
	}

	sb.WriteString("\n# Timeouts\n")
	sb.WriteString("REQUEST_TIMEOUT_MS=10000\n")

	return []byte(sb.String())
}
