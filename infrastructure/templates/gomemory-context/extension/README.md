# gomemory Project History Context Extension

Esta extensión conecta [gomemory](https://github.com/) — el servidor de
memoria persistente del proyecto — con el flujo de spec-kit, para que cada
especificación nueva se redacte con conocimiento del historial del
proyecto (features previas, decisiones arquitectónicas) sin tener que
barrer manualmente el directorio `specs/`.

Ver `specs/011-gomemory-spec-context/spec.md` para la especificación
completa y `specs/011-gomemory-spec-context/research.md` para las
decisiones técnicas detrás de este diseño.

## Qué hace

Antes de que se complete una especificación nueva (`/speckit-specify`), el
hook `before_specify` de esta extensión ejecuta
`speckit.gomemory-context.update`, que a su vez llama a `mem context` (la
CLI de gomemory) y entrega su salida — un resumen Markdown de memorias por
tipo (decisiones, patrones, bugfixes) y, si hay un proveedor externo de
grafo de código conectado, un resumen aparte y rotulado de esa estructura.

A diferencia de los hooks de la extensión `agent-context` (todos
opcionales), el hook `before_specify` de esta extensión es **mandatorio**:
se ejecuta automáticamente, sin pedir confirmación, porque el objetivo es
que el historial esté disponible sin que nadie tenga que solicitarlo.

También se ofrece el mismo resumen en `before_plan` y `before_clarify`
(`/speckit-plan`, `/speckit-clarify`), por si esas fases ocurren en una
sesión nueva que no cargó ya el contexto en `/speckit-specify`. A
diferencia de `before_specify`, estos dos hooks son **opcionales**: en la
mayoría de los casos plan/clarify ocurren en la misma sesión donde
`/speckit-specify` ya incorporó el resumen, así que forzarlos de nuevo
sería redundante — spec-kit los ofrece como sugerencia y quien esté
trabajando decide si los ejecuta.

## Por qué existe

Ver el "Contexto: qué existe hoy" de `spec.md`: sin esta extensión, la
única forma de saber qué ya existe en el proyecto es leer manualmente cada
`spec.md` bajo `specs/` — algo que crece con cada feature y que en la
práctica nadie hace de forma sistemática.

## Cómo desactivarla

Hay dos vías independientes:

1. **Desde gomemory** (recomendado si solo quieres apagar el resumen sin
   tocar spec-kit): abre la TUI de gomemory (`./mem`) → pantalla de
   configuración → alterna "Brazo extensor spec-kit", o usa
   `mem settings --speckit-context=false`. El script del hook lee este
   interruptor directo de `.memory/settings.json` antes de hacer nada — si
   está apagado, termina sin salida.
2. **Desde spec-kit** (si administras extensiones directamente):
   `specify extension disable gomemory-context`.

Ambas vías son válidas y no interfieren entre sí — la primera es la que
tiene efecto inmediato y visible desde gomemory sin depender de la CLI de
`specify`.

## Separación de fuentes en el resumen

La salida de `mem context` (y por lo tanto la de este hook) nunca mezcla
las dos fuentes de información en un solo bloque:

- **Historia y decisiones del proyecto** (gomemory): secciones como
  `## Decisiones de Arquitectura`, `## Decisiones Técnicas`,
  `## Patrones y Convenciones`, `## Bugfixes` — el "por qué" y el "qué se
  hizo".
- **Estructura de código** (grafo externo, cuando hay un proveedor
  conectado): sección aparte `## Grafo de código externo (<provider>)` —
  el "qué/cómo" (lenguajes, módulos, símbolos de alto impacto).

Esta separación **no la implementa esta extensión**: ya existe en
`application/usecases/build_context.go` (función
`writeCodeProviderSection`, feature 010) y está cubierta por
`application/usecases/build_context_test.go`. Esta extensión solo hereda
esa garantía al pasar la salida de `mem context` tal cual, sin
recombinarla ni reformatearla.

## Requisitos

El script del hook necesita el binario `mem` de gomemory: busca `./mem` en
la raíz del proyecto (el que deja `mem install`) y, si no lo encuentra,
`mem` en `PATH`. Si ninguno está disponible, el hook no produce salida y el
flujo de `/speckit-specify` continúa exactamente igual que sin esta
extensión — no hace falta desinstalar nada para que la ausencia de
gomemory sea inofensiva.

## Instalación

Como `agent-context`, esta extensión se instala con la CLI `specify` — no
edites `.specify/extensions.yml` ni `.specify/extensions/.registry` a mano
(esos archivos incluyen un hash del manifiesto y metadata que la CLI
calcula al instalar).

```bash
# Desde una copia FUERA de .specify/extensions/gomemory-context/ — specify
# se niega a instalar si el origen es el mismo directorio de destino.
cp -R .specify/extensions/gomemory-context /ruta/temporal/gomemory-context
specify extension add /ruta/temporal/gomemory-context --dev
```

`specify extension list` debería mostrar `gomemory-context` junto a
`agent-context`, y `.specify/extensions.yml` debería traer sus tres hooks
(`before_specify`, `before_plan`, `before_clarify`).

## Comandos

| Command | Description |
|---------|-------------|
| `speckit.gomemory-context.update` | Obtiene el resumen de `mem context` y lo entrega como salida del hook. |

## Scripts

- **Bash**: `scripts/bash/update-gomemory-context.sh`
- **PowerShell**: `scripts/powershell/update-gomemory-context.ps1`

Ninguno de los dos recibe argumentos ni modifica archivos — son de solo
lectura sobre `.memory/settings.json` y el binario `mem`.
