package usecases

import "mem/application/ports"

// FirstAvailable devuelve el primer proveedor de la lista (orden de
// prioridad declarado por quien la arma) cuyo snapshot cacheado esté
// disponible, o nil si ninguno lo está o la lista es vacía/nil. Instantáneo:
// Snapshot() solo lee el cache, nunca invoca al proveedor — mismo contrato
// de no-bloqueo del resto de esta feature (feature 010, Historia 3).
//
// No reemplaza a build_context.go, que sigue mostrando una sección por CADA
// proveedor disponible en get_context (sin cambios, sin regresión) — esto es
// solo para los consumidores que necesitan una única fuente inequívoca
// (Historia 1: anotación de impacto; Historia 2: export/import de ADR).
func FirstAvailable(providers []ports.CodeGraphProvider) ports.CodeGraphProvider {
	for _, p := range providers {
		if p == nil {
			continue
		}
		if p.Snapshot().Available {
			return p
		}
	}
	return nil
}
