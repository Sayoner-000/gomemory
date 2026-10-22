# US2 — ciclo de vida real (T039)

2026-10-21. Binario `gomemory 2.35.0` recompilado con 040. HOME, PATH, almacén y repositorio temporales; ninguna configuración real modificada.

| Comprobación | Resultado |
|---|---|
| install global de los 3 agentes, seguido de install repetido | exit 0; Claude, Codex y OpenCode cableados |
| expand off + pre-tool rm -rf (solo payload, no se ejecuta shell) | `{}`; exit 0 |
| expand on + mismo payload | `permissionDecision: deny`; exit 0 |
| off rules + status --json | rules=false; resto=true |
| simulación de instrucciones v9 y hooks previos, update --version 2.35.0 --yes | exit 0; protocolo v10 y expand restaurados |
| uninstall de proyecto --yes --keep-memory | exit 0; se conserva configuración global por contrato |
| uninstall --all --yes --keep-memory --no-scan | exit 0; cero hook expand, plugin retirado, skills retiradas |

`TestExpandInstallUpgradeUninstall` valida además preservación de terceros, composición idempotente y limpieza; `TestUninstall_SistemaSinRastros` pasa con el inventario de las cinco skills.

Registro literal de la ejecución aislada: `/private/tmp/expand040-life-cfegxkp8/results.json` (transitorio). El binario de la prueba se retiró a sí mismo únicamente dentro del scratchpad.
