# ToolLab

ToolLab es un laboratorio de análisis asistido por evidencia para APIs y servicios. Toma un target compuesto por un repo local y un `base_url`, inspecciona el código fuente, ejecuta probes HTTP controlados y produce artifacts reutilizables para documentación, auditoría técnica, comprensión de endpoints y exportes operativos.

No es solo un generador de prompts ni solo un scanner. El núcleo del producto es un loop reproducible:

1. `preflight`
2. `astdiscovery`
3. `schema`
4. `smoke`
5. `authmatrix`
6. `fuzz`
7. `logic`
8. `abuse`
9. `confirm`
10. `report`

Sobre ese loop, ToolLab genera:

- `run_summary`, `dossier_full`, `dossier_docs_mini` y `dossier_llm`
- `endpoint_intelligence`, `endpoint_queries`, `postman_collection`, `curl_book`
- documentación LLM bounded y, cuando se habilite, auditoría LLM
- un workspace UI para navegar evidencia, endpoints, documentación y QA crudo

## Componentes

- `toollab-core`: backend Go que orquesta runs, artifacts, pipeline, exports y runtime LLM
- `toollab-ui`: frontend React/TypeScript para operar el laboratorio
- `docker-compose.yml`: stack local completo
- `docs/prompts/`: suite documental para diseñar, extender y mantener ToolLab

## Uso rápido

```bash
make up
```

Endpoints locales:

- UI: [http://localhost:5173](http://localhost:5173)
- API: [http://localhost:8090](http://localhost:8090)

Modo desarrollo sin Docker:

```bash
make install
make dev
```

## Qué problema resuelve

ToolLab reduce el trabajo manual de entender una API desconocida o un servicio heredado. En vez de depender solo de OpenAPI, README o intuición, combina AST, runtime evidence y outputs derivados para responder preguntas como:

- qué endpoints existen realmente
- cuáles responden y con qué contratos
- dónde hay auth, drift, errores, leaks o inconsistencias
- cómo consultar un endpoint con `curl`, `http` file o Postman
- cómo documentar un servicio sin inventar comportamiento no observado

Como línea avanzada, ToolLab también puede crecer hacia simulación conductual: sandbox reproducible para actores autónomos, servicios y policies, orientado a detectar comportamiento emergente y riesgo sistémico.

La forma recomendada de implementarlo, si se avanza, es como un `run kind` adicional dentro del mismo ToolLab, no como otro producto separado.

## Modo de evidencia

Cada run queda clasificado como:

- `offline`: no hubo runtime útil; solo hay AST y preflight
- `online_partial`: hubo evidencia limitada; la confianza debe ser conservadora
- `online_good` / `online_strong`: evidencia suficiente para documentación y scoring más útiles

Ese modo impacta documentación, findings, scores y la interpretación LLM.

## Mapa documental

- `docs/DOC.md`: explicación corta del producto, arquitectura y flujo operativo
- `docs/prompts/README.md`: índice de la suite oficial de prompts
- `toollab-core/README.md`: referencia técnica del backend
- `toollab-ui/README.md`: referencia técnica del frontend
