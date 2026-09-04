package domain

import "regexp"

// ProtocolVersionMarker es el marcador de la versión VIGENTE del bloque de
// protocolo que gomemory administra en los archivos de instrucciones de cada
// agente. Fuente única: tanto el instalador (adapters/primary/cli/cmd_install.go)
// como el inspector de cobertura (adapters/primary/setup/activation_inspect.go,
// feature 019) lo consultan desde aquí, para que nunca puedan divergir sobre
// cuál es "la versión vigente".
const ProtocolVersionMarker = "<!-- gomemory-protocol-v7 -->"

// UniversalInstructionsVersionMarker identifica el baseline portable que se
// instala una sola vez en las instrucciones de usuario. Es independiente del
// protocolo GoMemory: este último puede cambiar sin reinyectar el baseline en
// cada hook o conexión MCP.
const UniversalInstructionsVersionMarker = "<!-- gomemory-universal-agent-instructions-v1 -->"

// ProtocolVersionPattern reconoce el marcador de versión del protocolo sin
// importar el número de versión (v1, v2, v3...), para ubicar el bloque
// instalado aunque sea de una versión anterior a ProtocolVersionMarker.
var ProtocolVersionPattern = regexp.MustCompile(`<!-- gomemory-protocol-v\d+ -->`)

// UniversalInstructionsVersionPattern permite detectar instalaciones previas
// del baseline y pedir una reconciliación si quedaron desactualizadas.
var UniversalInstructionsVersionPattern = regexp.MustCompile(`<!-- gomemory-universal-agent-instructions-v\d+ -->`)
