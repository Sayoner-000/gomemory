package cli

import (
	"context"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// compressDeliveredContext pasa por el motor nativo un documento de contexto
// que gomemory va a entregar al agente (get_context, mem context, el contexto
// de arranque y el de después de compactar). Solo actúa con el nivel max
// (feature 033, FR-020).
//
// Reglas que no deben romperse:
//
//   - El hash que se registra en DeliveryLog es SIEMPRE el del documento sin
//     procesar, y el llamador lo registra ANTES de llamar aquí: get_plan_context
//     compara contra el hash del documento crudo, así que registrar el de la
//     salida comprimida desactivaría su supresión (hallazgo I1 del análisis).
//   - Si el motor no omitió nada con ref recuperable, se entrega el documento
//     intacto: el contexto lleva el protocolo y es el prefijo que el proveedor
//     puede cachear, y no se altera por una ganancia nula.
func compressDeliveredContext(deps *Deps, raw string) string {
	if deps == nil || deps.CompressionLevel != ports.CompressionMax || deps.Compressor == nil || raw == "" {
		return raw
	}
	res := usecases.CompressContent(deps.Compressor, ports.CompressionMax, raw, nil, "")
	if len(res.Refs) == 0 {
		return raw
	}
	return res.Content + RetrieveHint
}

// RetrieveHint explica al agente cómo recuperar lo omitido. Va al FINAL y solo
// en las salidas que llevan marcas: no altera el prefijo estable ni cuesta
// nada cuando no hay omisiones (feature 033, contrato cli-and-mcp.md).
const RetrieveHint = "\n\n> Marcas ⟦mem⟧ … ref=X: contenido omitido para ahorrar tokens; recupéralo íntegro con pack_retrieve(ref=X) (CLI: mem pack retrieve X).\n"

// deltaActivo dice si aplica el delta de sesión: solo en max (FR-031) y con
// registro disponible.
func deltaActivo(deps *Deps) bool {
	return deps != nil && deps.CompressionLevel == ports.CompressionMax && deps.DeliveredBlocks != nil
}

func deltaCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), domain.StoreTimeout)
}

// deliverContextDoc prepara un documento de contexto para entregarlo: primero
// el delta de sesión (lo ya entregado e igual se sustituye por una línea
// compacta) y después el motor de compresión. El llamador ya registró el hash
// del documento crudo en DeliveryLog.
func deliverContextDoc(deps *Deps, raw string) string {
	return compressDeliveredContext(deps, applyContextDelta(deps, raw))
}

// deliverContextDocFull es deliverContextDoc con la salida de recuperación
// (full=true): entrega el documento entero, sin delta, y NO lo anota. La usa
// quien no tiene el contexto —un subagente, sobre todo—, y el registro es de
// toda la sesión: anotarlo haría que el agente principal recibiera marcadores
// de lo que nunca leyó (A-R2-01 de acr_df09a036). Mismo criterio que
// get_plan_context(full=true).
func deliverContextDocFull(deps *Deps, raw string, full bool) string {
	if !full {
		return deliverContextDoc(deps, raw)
	}
	return compressDeliveredContext(deps, raw)
}

// applyContextDelta aplica solo el delta de sesión (sin compresión).
func applyContextDelta(deps *Deps, raw string) string {
	if !deltaActivo(deps) {
		return raw
	}
	ctx, cancel := deltaCtx()
	defer cancel()
	return usecases.SessionDelta{Blocks: deps.DeliveredBlocks}.ApplyContext(ctx, raw)
}

// deliverSearchDoc es la variante para los resultados de search_memories.
func deliverSearchDoc(deps *Deps, raw string) string {
	doc := raw
	if deltaActivo(deps) {
		ctx, cancel := deltaCtx()
		doc = usecases.SessionDelta{Blocks: deps.DeliveredBlocks}.ApplySearch(ctx, raw)
		cancel()
	}
	return compressDeliveredContext(deps, doc)
}

// resetDeliveries olvida lo entregado en la sesión activa (FR-019): tras
// compactar o al arrancar, el agente ya no tiene ese material. Borra tanto el
// registro por bloques como el registro por canal de get_plan_context.
func resetDeliveries(deps *Deps) {
	if deps == nil {
		return
	}
	if deps.DeliveryLog != nil {
		_ = deps.DeliveryLog.Reset()
	}
	if deps.DeliveredBlocks != nil {
		ctx, cancel := deltaCtx()
		_ = deps.DeliveredBlocks.Reset(ctx)
		cancel()
	}
}

// markContextDelivered anota como entregado un documento que va completo.
func markContextDelivered(deps *Deps, doc string) {
	if !deltaActivo(deps) {
		return
	}
	ctx, cancel := deltaCtx()
	usecases.SessionDelta{Blocks: deps.DeliveredBlocks}.MarkDelivered(ctx, doc)
	cancel()
}
