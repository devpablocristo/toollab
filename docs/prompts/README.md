# Suite Oficial de Prompts de ToolLab

Esta carpeta contiene la suite documental oficial para diseñar, operar y evolucionar `toollab` sin drift entre producto, código y narrativa.

## Orden recomendado

1. `00_base_transversal.md`
2. `01_producto_y_arquitectura.md`
3. `02_pipeline_de_analisis.md`
4. `03_artifacts_dossier_exports.md`
5. `04_llm_runtime_y_prompting.md`
6. `05_playground_y_manual_probing.md`
7. `06_ui_workspace_y_experiencia.md`
8. `07_testing_validacion_y_release.md`
9. `08_behavioral_simulation_foundations.md`
10. `09_multiagent_scenarios_and_policies.md`
11. `10_emergent_behavior_audit_and_scoring.md`
12. `11_behavioral_simulation_runtime_architecture.md`

## Reglas de lectura

- `00` define invariantes obligatorios para todos los demás prompts
- los prompts describen el producto real, no aspiraciones abstractas
- cuando exista tensión entre documentación y código, debe corregirse la documentación o implementarse el cambio faltante con evidencia
- los términos técnicos, nombres de artifacts, endpoints y tipos permanecen en English
- la prosa explicativa se mantiene en español

## Qué cubre la suite

- identidad y alcance real de ToolLab
- arquitectura backend/frontend
- pipeline de análisis y run modes
- sistema de artifacts, dossiers y exports
- runtime LLM bounded y externalización de prompts
- playground/manual probing
- workspace UI
- testing, evidencia y criterios de release
- simulación multiagente y auditoría de comportamiento emergente
