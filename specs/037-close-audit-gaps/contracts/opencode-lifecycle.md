# Contrato del adaptador OpenCode v2

Requisitos FR-016–021, FR-024–027. Mantener export dual y rama v1. La [referencia oficial](https://dev.opencode.ai/v2/docs/build/plugins/) permite modificar event.result y abortar event.subscribe; la comprobación del consumidor real sigue siendo obligatoria.

## Resultado de herramienta

Solo actuar si status=completed, sesión resuelta al proyecto propio y compresión activada por política vigente. Usar traducción de nombres actual shell->bash y subagent->task. Antes de publicar, volver a comprobar que la instancia no esté closing.

| Forma | Extracción | Publicación cuando hay ganancia |
| --- | --- | --- |
| content string | Texto de content | Sustituir content, conservar otros campos |
| content Part[] con texto | Concatenar partes textuales con separador actual | Sustituir primera textual con copia de sus metadatos; quitar demás textuales agregadas; mantener no-textuales y orden relativo |
| output string | Texto de output | Sustituir output, conservar otros campos |
| Sin texto soportado | Ninguno | No-op |

Evitar serializar un objeto output y luego reemplazarlo con string. Si la reducción es igual al original, no mutar la estructura ni atribuir ganancia. No sustituir resultados de sesiones ajenas, incompletos o cuya propiedad no se pueda resolver. Las partes públicas recuperadas deben corresponder al original público exacto.

## Lifecycle

- Mantener `initializing/active/closing/closed`, lista de registrations, promise de subscription, operaciones propias y promise única de cleanup.
- Registrar operaciones antes de empezar; callbacks que llegan después de closing retornan sin nuevo IO. Cleanup establece closing sin esperar, aborta suscripción y operaciones propias, dispone todos los registros e impide publicación tardía.
- Exec v2 acepta AbortSignal y conserva cwd/input. Proceso ya abortado no arranca; proceso en curso recibe cancelación y, si no termina tras gracia acotada, terminación forzada. Esperar close para confirmar salida, además de manejar rechazo por abort. Limitar la gracia inicial a 1 s dentro del deadline global de 5 s.
- Requests de sesión v2 usan requestOptions.signal admitido por el cliente; comprobar el contrato de la versión ejercitada. Callbacks y tareas se esperan para no dejar promesas trabajando al declarar éxito.
- Deadline global de 5 s para cierre normal en entorno de validación, incluido cierre de sesión propia cuando proceda. Ese cierre final usa un canal de ejecución limitado al presupuesto restante, no la señal ya abortada del trabajo normal. No duplicarlo en cierres repetidos.
- Un fallo al disponer no impide intentar otros cierres. Registrar código OPENCODE_CLEANUP_FAILED depurado y no declarar éxito con recursos vivos.
- Fallo de setup ejecuta rollback propio y conserva la causa inicial. No cierra sesión de memoria ajena ni llama dispose v1 que tenga ese efecto.
- Cleanup repetido comparte resultado; nuevas operaciones quedan rechazadas desde primer closing. No recurrir solo a Promise.race para ocultar tareas pendientes.

Los procesos mem llamados por esta integración no deben usar shell arbitrario ni descendientes fuera de control; la evidencia de cierre incluye el árbol propio que efectivamente se lance. Un descendant observado requiere cierre real, no solo muerte del proceso inmediato.

## Fixtures y host

Fixture compara session/start por sus primeros argumentos y además exige --conversation=<id> y cwd del proyecto. Simulación de fallo admite argumentos adicionales y deja un contador de activaciones para probar que el fallo se produjo. Finally libera los recursos incluso si falla un assert.

Host v2: registrar versión exacta, usar HOME/XDG/config/DB aislados y servidor privado; verificar consumo posterior al hook mediante observador de prueba o mensaje de herramienta almacenado en la sesión. Una captura anterior a la aplicación de hooks no cuenta como evidencia. Repetir strings, partes mixtas y output, casos no-op, propiedad ajena, fallos de registros y cierre con evento en curso. Herramientas auxiliares y observadores de prueba no se instalan en el entorno real de usuario.

El runner `scripts/audit_037_opencode_smoke.mjs` previsto acepta --mem-bin, --opencode-bin y --out-dir; valida major=2 y crea todas las rutas aisladas. Usa opencode --standalone y comandos de la versión validada; si se necesita modelo para observar consumo, exige configuración de prueba proporcionada por el operador y no reutiliza credenciales reales automáticamente. Falta de host/modelo devuelve código 2 con status=pending_verification; no se omite silenciosamente. Nunca actualiza ni detiene el servicio habitual del usuario.

Los resultados de host se guardan depurados: versión, representación, bytes antes/después, hash público, referencias verificadas, operaciones posteriores al cierre, duración y recursos pendientes. Las comprobaciones previas a corrección deben fallar en H-05/H-06; las posteriores deben pasar sin modificar el mock para ocultar el defecto.
