package llm

import (
	"context"
	"fmt"
	"os"
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
	provider := os.Getenv("LLM_PROVIDER")

	switch provider {
	case "gemini":
		return newGeminiProvider()
	case "vertex":
		return newVertexProvider()
	case "ollama", "":
		return newOllamaProvider()
	default:
		fmt.Fprintf(os.Stderr, "llm: unknown provider %q, falling back to ollama\n", provider)
		return newOllamaProvider()
	}
}
