<!-- gomemory-workrules-v2 -->
## Reglas operativas del proyecto

Estas reglas complementan el baseline universal. Prevalecen solo cuando aportan
evidencia operativa específica de este proyecto.

1. **Primero reproduce la realidad.** Ante un bug o una paridad con un sistema
   en ejecución, comprueba logs, `curl`, navegador o el servicio real antes de
   cambiar código o iniciar un flujo de especificación.
2. **Tests verdes no bastan.** Valida el artefacto realmente servido cuando sea
   pertinente; un fixture que no representa al upstream no demuestra el caso
   real.
3. **Si el cambio no se ve, comprueba el despliegue.** Revisa binario/bundle,
   URL y caché antes de atribuir el problema a la implementación.
4. **Usa constitución y especificaciones como guía, no como ritual.** No fuerces
   un proceso que contradiga la evidencia o el flujo real de la persona.
5. **Cierra los hallazgos.** Todo riesgo o defecto descubierto debe incluir una
   propuesta de cierre y una validación proporcional antes de declararlo listo.
6. **Delega de forma intencional.** Si Octopus está disponible, consulta su ruta
   antes de crear subagentes; en cualquier caso, el beneficio debe justificar
   el coste de coordinación.
