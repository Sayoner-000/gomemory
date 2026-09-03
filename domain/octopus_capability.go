package domain

// RuntimeCapabilities es lo que el entorno de ejecución DECLARA saber hacer.
// Octopus nunca lo detecta ni lo infiere del entorno: llega en la petición. Esa
// frontera es lo que mantiene la política independiente del proveedor y del
// runtime (INV-AAR-017, INV-AAR-018).
type RuntimeCapabilities struct {
	Subagents         bool
	Parallel          bool
	IsolatedContext   bool
	ModelSelection    bool
	ContinuableAgents bool
	MaxParallel       int
}

// Normalize aplica el default CONSERVADOR: lo que no se declara, no existe.
//
// La dirección importa. Asumir capacidades ausentes produciría recomendaciones
// que nadie puede ejecutar — un plan con cuatro agentes para un runtime que no
// tiene subagentes es peor que no haber enrutado. Por eso una estructura vacía
// equivale a "sin subagentes", que fuerza INLINE (FR-035).
//
// Es idempotente: normalizar dos veces da lo mismo que una.
func (c RuntimeCapabilities) Normalize() RuntimeCapabilities {
	if !c.Subagents {
		// Sin subagentes no hay nada que paralelizar ni contexto ajeno que aislar.
		c.Parallel = false
		c.IsolatedContext = false
	}
	if !c.Parallel {
		c.MaxParallel = 1
	} else if c.MaxParallel < 1 {
		// Una capacidad paralela declarada sin tope explícito conserva la
		// capacidad: el límite efectivo lo aplica la política/configuración.
		c.MaxParallel = DefaultMaxParallel
	}
	return c
}

// ConcurrenciaEfectiva es el mínimo entre lo que el runtime admite y el tope
// configurado. El más restrictivo siempre gana (INV-AAR-008).
func (c RuntimeCapabilities) ConcurrenciaEfectiva(topeConfigurado int) int {
	n := c.Normalize().MaxParallel
	if topeConfigurado > 0 && topeConfigurado < n {
		return topeConfigurado
	}
	return n
}
