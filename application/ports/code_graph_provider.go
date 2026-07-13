package ports

import "mem/domain"

// CodeGraphProvider es un proveedor OPCIONAL y provider-agnóstico de
// inteligencia de código externa YA indexada (p.ej. codebase-memory-mcp).
//
// Principio: gomemory NUNCA depende de él. Si no hay proveedor disponible, el
// contexto se arma igual con el grafo propio Fase-1. Es un brazo extensor
// enchufable, no un requisito, y todo fallo degrada en silencio.
//
// Contrato de NO-BLOQUEO (patrón engram: hot path barato + refresco en
// background):
//   - Snapshot() SOLO lee el estado cacheado en disco: instantáneo, nunca
//     invoca al proveedor externo ni bloquea el hot path (armar get_context /
//     decidir guardar).
//   - MaybeRefresh() dispara, si el snapshot está viejo, un refresco
//     DESACOPLADO (proceso detached) que sondea al proveedor con timeout corto
//     y reescribe el snapshot para la PRÓXIMA llamada. Retorna de inmediato:
//     nunca lo espera el hot path. El enriquecimiento es eventual.
//
// Es agnóstico al agente que invoque la memoria (opencode, claude, etc.):
// todo vive en el binario `mem`, no en el plugin de ningún agente.
type CodeGraphProvider interface {
	Name() string
	Snapshot() domain.CodeProviderSnapshot
	MaybeRefresh()
}
