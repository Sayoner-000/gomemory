package ports

import "mem/domain"

// CodeGraphProvider es un proveedor OPCIONAL y provider-agnóstico de
// inteligencia de código externa YA indexada (CodeGraph).
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
//     DESACOPLADO (proceso detached) que sondea al proveedor con timeout corto,
//     inicializa e indexa el proyecto cuando aún no hay índice o se recomienda
//     reindexar, sincroniza los cambios incrementales y reescribe el snapshot
//     en .memory para la PRÓXIMA llamada. La inicialización y la indexación
//     usan un plazo largo (10 min) dentro del proceso detached; el sondeo y la
//     sincronización conservan plazos cortos. Retorna de inmediato: nunca lo
//     espera el hot path. El enriquecimiento es eventual y
//     GOMEMORY_NO_CODE_REFRESH desactiva el refresco.
//
// Es agnóstico al agente que invoque la memoria (opencode, claude, etc.):
// todo vive en el binario `mem`, no en el plugin de ningún agente.
type CodeGraphProvider interface {
	Name() string
	Snapshot() domain.CodeProviderSnapshot
	MaybeRefresh()
	// ImpactFor resuelve, SOLO contra el snapshot cacheado (mismo contrato de
	// no-bloqueo que Snapshot()), si filepath casa con un símbolo del grafo
	// externo marcado como hotspot. false si no hay match o no hay snapshot
	// disponible — nunca invoca al proveedor (feature 010, Historia 1).
	ImpactFor(filepath string) (domain.CodeImpactAnnotation, bool)
}
