Sí. Te dejo una checklist detallada, en orden. La idea es probar primero lo básico, después calidad del diagnóstico, y al final casos borde.

**1. Levantar Proyecto**
```bash
make dev
```

Verificar:
- Backend responde en `http://127.0.0.1:8090/healthz`.
- Frontend abre en `http://127.0.0.1:5173/repos`.
- No aparecen errores visibles en consola del navegador.
- No aparecen errores graves en logs del backend/frontend.

**2. Crear Repo V2**
En `/repos`, crear repo con:

```text
/home/pablocristo/Proyectos/pablo/toollab
```

Verificar:
- Se crea sin error.
- Aparece en la lista lateral.
- El path se muestra correctamente.
- No se crea target V1.
- La pantalla sigue en V2.

**3. Audit Estático Sin Tests**
Config:
- Generate tests: OFF
- Run existing tests: OFF
- Allow docs read: OFF
- Install dependencies: OFF

Verificar:
- El audit termina.
- Score aparece.
- `tests` queda vacío o indica que no se corrieron.
- Se generan findings estáticos.
- Se genera documentación.
- Evidence ledger tiene entradas.
- No se creó sandbox innecesario.
- README/docs del repo no aparecen como fuente.

**4. Audit Con Tests**
Config:
- Generate tests: ON
- Run existing tests: ON
- Allow docs read: OFF
- Install dependencies: OFF

Verificar:
- El audit termina.
- Tests existentes se registran.
- Tests generados se registran.
- Si algo falla, queda como finding con evidencia.
- Los tests generados aparecen con path en sandbox, no en el repo original.
- No aparecen archivos `zz_tollab_generated_test.go` ni similares en el repo original.

**5. Política De Docs**
Primero con:

- Allow docs read: OFF

Verificar:
- Docs generadas dicen que ignoraron docs existentes.
- README, `docs/**`, `*.md`, `*.mdx`, `CHANGELOG*` no influyen en documentación/evidencia.

Luego correr otro audit con:

- Allow docs read: ON

Verificar:
- Cambia la política indicada.
- Docs del repo pueden aparecer inventariadas.
- Queda claro cuándo se permitió leer docs.

**6. Findings**
Para cada finding revisar:
- Tiene `severity`.
- Tiene `priority`.
- Tiene `state`.
- Tiene `confidence`.
- Tiene `rule_id`.
- Tiene descripción concreta.
- Tiene archivo/línea cuando aplica.
- Tiene evidencia.
- Tiene recomendación mínima.
- Tiene validación sugerida.
- No suena a “opinión estética”.

Preguntas clave:
- ¿El finding es accionable?
- ¿Está exagerada la severidad?
- ¿Está bien marcado como Confirmado/Probable/Hipótesis?
- ¿La evidencia alcanza para confiar?
- ¿Hay falsos positivos obvios?

**7. Evidence Ledger**
Verificar:
- Hay evidencia de inventario.
- Hay evidencia por finding relevante.
- Evidence muestra archivo/línea/comando cuando aplica.
- Evidence no referencia README/docs si `Allow docs read` está OFF.
- Evidence no es puro texto genérico.
- Se puede entender por qué ToolLab llegó a cada conclusión.

**8. Score**
Verificar:
- Score está entre 0 y 100.
- Se muestran las 5 categorías:
  - build_tests
  - bugs_findings
  - test_quality
  - docs_traceability
  - ci_config
- Cada categoría tiene puntos obtenidos y máximos.
- Las deducciones tienen razón.
- El score baja si hay findings.
- El score sube si pasan tests.
- El score no parece arbitrario.

**9. Tests Generados**
Verificar:
- Se generan solo en sandbox/data.
- No modifican el repo auditado.
- Si el repo es Git, no ensucian el working tree.
- Si falla generación, queda registrado como blocked/failed.
- Si pasa, queda registrado como passed.
- El output truncado sigue siendo útil.

**10. No Instalación De Dependencias**
Con `Install dependencies: OFF`:
- En repos Node sin `node_modules`, debe bloquear o saltear checks Node, no ejecutar `npm install`.
- No debe aparecer `node_modules` nuevo en el repo auditado.
- No debe modificar lockfiles.
- No debe modificar package.json.

**11. UI**
Verificar:
- Se puede seleccionar repo.
- Se puede seleccionar audit run anterior.
- Score/findings/docs/tests cambian según audit seleccionado.
- No hay textos cortados de forma grave.
- Findings largos siguen legibles.
- Evidence ledger no rompe layout.
- Output de tests largo no rompe la pantalla.
- Link “V1 legacy” sigue funcionando.

**12. V1 Sigue Vivo**
Ir a:

```text
http://127.0.0.1:5173/targets
```

Verificar:
- V1 carga.
- No se rompieron targets/runs legacy.
- V2 no mezcló datos con V1.
- Rutas `/api/v1` siguen respondiendo si ya tenías datos.

**13. Casos Borde**
Probar crear repo con:
- Path inexistente.
- Archivo en vez de carpeta.
- Path vacío.
- Repo sin `go.mod`, `package.json`, `pyproject`, SQL.
- Repo solo con README.
- Repo con docs engañosos y `Allow docs read` OFF.
- Repo Node sin lockfile.
- Repo SQL con tabla sin PK.
- Repo Python con `requests.get` sin timeout.
- Repo Go con `http.ListenAndServe`.

Verificar:
- No crashea.
- Error es entendible.
- Findings no prometen más de lo que saben.
- Score sigue calculándose.

**14. Revisión De Archivos**
Después de auditar ToolLab sobre sí mismo:

```bash
git status --short
```

Verificar:
- No hay archivos nuevos en el repo auditado por ToolLab.
- No aparecieron tests generados en el working tree.
- Solo deberían estar tus cambios reales de desarrollo.

**15. Señales Para Anotar**
Anotá especialmente:
- Findings útiles.
- Findings falsos positivos.
- Findings repetidos o ruidosos.
- Severidades mal calibradas.
- Cosas que faltan detectar.
- Score que no coincide con tu intuición.
- UI confusa.
- Documentación generada útil/inútil.
- Cualquier error de backend/frontend.

Cuando termines esa pasada, lo siguiente sería ajustar calibración: bajar ruido, mejorar reglas y recién después pensar en borrar V1.