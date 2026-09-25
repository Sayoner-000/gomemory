// Package native implementa el motor nativo de compresión de contexto de
// gomemory (feature 033): aplica dentro del binario las prácticas de Headroom
// sin depender de ningún proceso externo.
//
// El flujo de un bloque es: umbral mínimo → router (tipo de contenido) →
// compresor del tipo (JSON, código, log/traza, diff, tabla o prosa) → guarda
// de literalidad → comparación con la compresión estructural → guardado de
// los originales omitidos → estadísticas. Toda omisión deja un marcador
// ⟦mem⟧ con una referencia que recupera el original byte a byte.
//
// Decisiones y alternativas descartadas:
// specs/033-native-context-compression/research.md.
package native
