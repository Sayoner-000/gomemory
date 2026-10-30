package usecases

import (
	"context"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// keepOmittedOriginal guarda el original de un item omitido y solo entonces
// añade su ref al recibo: una ref sin original sería una promesa rota (INV-C4).
func keepOmittedOriginal(req ContextRequest, receipt *domain.ContextReceipt, content string) {
	receipt.Refs = []string{}
	// Sin recibo no se promete ninguna ref: no hace falta guardar el original.
	if req.Originals == nil || !req.Receipt {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if ref, err := req.Originals.Put(ctx, req.Project, content, "context_item", "budget"); err == nil {
		receipt.Refs = []string{ref}
	}
}

// fitRenderedContextPack reserva también los recibos y contadores visibles.
// Las omisiones se explican; nunca se corta un item ni contenido obligatorio.
func fitRenderedContextPack(pack domain.ContextPack, counter ports.TokenCounter, req ContextRequest, originals map[string]string) (domain.ContextPack, error) {
	renderer := req.Renderer
	pack.Stats.Scope = "complete serialized pack; subsequent recoveries not measured here"
	for {
		// El contador de metadatos forma parte de su propia representación.
		stable := false
		for n := 0; n < 16; n++ {
			metadata := counter.Count(renderer.Render(pack)) - pack.TokenCount
			if metadata < 0 {
				metadata = 0
			}
			if metadata == pack.Stats.MetadataTokens {
				stable = true
				break
			}
			pack.Stats.MetadataTokens = metadata
		}
		if stable && counter.Count(renderer.Render(pack)) <= pack.Budget {
			return pack, nil
		}
		remove := -1
		for i := len(pack.Items) - 1; i >= 0; i-- {
			if pack.Items[i].Priority != domain.PriorityCritical {
				remove = i
				break
			}
		}
		if remove < 0 {
			return domain.ContextPack{}, domain.ErrCriticalContextOverflow
		}
		item := pack.Items[remove]
		pack.Items = append(pack.Items[:remove], pack.Items[remove+1:]...)
		pack.TokenCount -= item.Tokens
		pack.Stats.SelectedRawTokens -= item.RawTokens
		pack.Stats.ItemsDiscarded++
		if item.Priority == domain.PriorityRelevant {
			pack.Stats.ItemsRelevant--
		} else {
			pack.Stats.ItemsOptional--
		}
		for i := range pack.Receipts {
			if pack.Receipts[i].ID == item.ID {
				pack.Receipts[i].Status, pack.Receipts[i].Reason = "omitted", "serialized_budget"
				pack.Receipts[i].DeliveredSHA256 = ""
				keepOmittedOriginal(req, &pack.Receipts[i], originals[item.ID])
			}
		}
		pack.Stats.FinalTokens = pack.TokenCount
		pack.Stats.SavedTokens = pack.RawTokenCount - pack.TokenCount
		pack.Stats.SelectionSavedTokens = pack.RawTokenCount - pack.Stats.SelectedRawTokens
		pack.Stats.CompressionSavedTokens = pack.Stats.SelectedRawTokens - pack.TokenCount
		if pack.RawTokenCount > 0 {
			pack.CompressionRate = 1 - float64(pack.TokenCount)/float64(pack.RawTokenCount)
		}
		pack.Stats.CompressionRatio = pack.CompressionRate
	}
}
