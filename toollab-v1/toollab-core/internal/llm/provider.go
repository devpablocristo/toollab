package llm

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type Provider interface {
	Name() string
	Available(ctx context.Context) bool
	Interpret(ctx context.Context, fullContext string) (string, error)
}

const interpretPrompt = `Sos un experto en auditoría de APIs. Te doy TODOS los datos crudos de una auditoría automatizada.
Tu trabajo es interpretar estos datos y generar un reporte en español, en markdown, que un humano pueda usar para comprender completamente el servicio auditado.

SECCIONES REQUERIDAS:

## ¿Qué es este servicio?
Interpretá la descripción del servicio, su propósito, dominio y consumidores.

## ¿Qué hace?
Explicá cada endpoint: qué hace, qué recibe, qué devuelve. Agrupá por recurso/entidad.

## ¿Funciona bien?
Analizá la tasa de éxito, errores, latencias. ¿Qué endpoints fallan y por qué?

## ¿Es seguro?
Interpretá los hallazgos de seguridad. ¿Qué riesgos tiene? ¿Qué tan grave es?

## ¿Cumple con su contrato?
Analizá las violaciones de contrato. ¿Las respuestas coinciden con lo documentado?

## ¿Qué le falta?
Endpoints no testeados, funcionalidad ausente, problemas de cobertura.

## Conclusión y recomendaciones
Veredicto general: ¿está listo para producción? ¿Qué hay que mejorar primero?

REGLAS:
- Escribí en español
- Usá markdown con tablas cuando sea útil
- Sé concreto: citá endpoints, status codes, latencias, datos reales
- No repitas los datos crudos, interpretá y explicá
- Si falta información, decilo explícitamente

DATOS COMPLETOS DE LA AUDITORÍA:
%s`

func NewProvider() Provider {
	raw := strings.TrimSpace(os.Getenv("LLM_PROVIDER"))
	if raw == "" {
		raw = "ollama"
	}

	parts := strings.Split(raw, ",")
	providers := make([]Provider, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if p := providerByName(name); p != nil {
			providers = append(providers, p)
		}
	}

	if len(providers) == 0 {
		fmt.Fprintf(os.Stderr, "llm: no valid providers in LLM_PROVIDER=%q, falling back to ollama\n", raw)
		return newOllamaProvider()
	}
	if len(providers) == 1 {
		return providers[0]
	}
	return &multiProvider{providers: providers}
}

func providerByName(name string) Provider {
	switch name {
	case "gemini":
		return newGeminiProvider()
	case "vertex":
		return newVertexProvider()
	case "ollama":
		return newOllamaProvider()
	default:
		fmt.Fprintf(os.Stderr, "llm: unknown provider %q (skipping)\n", name)
		return nil
	}
}

type multiProvider struct {
	providers []Provider
}

func (m *multiProvider) Name() string {
	names := make([]string, 0, len(m.providers))
	for _, p := range m.providers {
		names = append(names, p.Name())
	}
	return strings.Join(names, " -> ")
}

func (m *multiProvider) Available(ctx context.Context) bool {
	for _, p := range m.providers {
		if p.Available(ctx) {
			return true
		}
	}
	return false
}

func (m *multiProvider) Interpret(ctx context.Context, fullContext string) (string, error) {
	var errs []string
	for _, p := range m.providers {
		if !p.Available(ctx) {
			errs = append(errs, fmt.Sprintf("%s: not available", p.Name()))
			continue
		}
		out, err := p.Interpret(ctx, fullContext)
		if err == nil {
			return out, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
	}
	return "", fmt.Errorf("all llm providers failed: %s", strings.Join(errs, " | "))
}
