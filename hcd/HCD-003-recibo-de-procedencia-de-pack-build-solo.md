# Recibo de procedencia de pack_build solo bajo demanda

ID: HCD-003
Tipo: evolution
Carril: fast
Motivo: Cambio acotado con decisión ya tomada por la persona (opción «Primero fix del recibo»), sin decisiones abiertas interdependientes.

## Intención
Que pack_build y mem pack build no entreguen por defecto el recibo de procedencia ni la contabilidad nueva añadidos en HCD-001 (encarecían cada pack ~15%); solo con una opción explícita (--receipt en CLI, receipt en MCP). Se conservan el presupuesto de la respuesta completa, el desbordamiento explícito, las refs recuperables y el aislamiento por proyecto.


## Alcance
Incluido: la salida por defecto de `pack_build` (MCP) y de `mem pack build` (texto y `--json`) deja fuera el recibo de procedencia y las cifras nuevas de contabilidad (selección, compresión, delta, metadatos, recuperaciones, ámbito y tipo de contador). Se entregan solo con `--receipt` en la CLI y `receipt: true` en MCP, y en ese caso también se cuentan dentro del presupuesto. Se conservan el aviso de no-match, el presupuesto de la respuesta completa, el desbordamiento explícito, las refs de omisión y el aislamiento por proyecto.
Excluido: `get_context` y las demás respuestas propias; cambiar el formato del recibo cuando se pide; `pack_show` y `pack_stats`, que reformatean un pack ya construido.

## ¿Qué pasa si no lo hacemos?
La release incluiría la regresión medida en HCD-001: cada `pack_build` costaría ~15% más (el no-match, más del doble) por metadatos que el agente no pidió.

## Alternativa más barata
Revertir todo HCD-001 eliminaría también el arreglo de aislamiento entre proyectos (una fuga real), las refs de omisión recuperables y el presupuesto de la respuesta completa. Hacer opcional el recibo es más barato y conserva esas garantías.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Hacer opcional el recibo y la contabilidad nueva en `mem pack build` (texto y JSON) y en MCP `pack_build`, y medir el presupuesto con la misma salida que se entrega → prueba `TestHCD003ReceiptOptIn` (rojo antes del cambio): sin la opción, no hay recibo ni contabilidad nueva y la respuesta cabe en el presupuesto; con la opción, el recibo aparece y también se cuenta.
- [x] 2. Comprobar en el binario servido que el coste por defecto vuelve a la referencia → reejecutar `scripts/hcd001_evaluate.py` con el corpus de HCD-001 sobre el binario nuevo y sobre la referencia `6c747abd`; observar que el aumento de bytes del −14,96% desaparece y que la calidad no empeora.

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-30 01:37 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run '^TestHCD003ReceiptOptIn$' -count=1 -v` (salida 1)
              Procedencia (contenido protegido; SHA-256): [{"ID":"memory:1","Source":"","OriginalSHA256":"166c43f1eb4c7d0562d57b425d5d5cd9297f4204b569be4786935fb2cb225858","DeliveredSHA256":"166c43f1eb4c7d0562d57b425d5d5cd9297f4204b569be4786935fb2cb225858","Status":"selected","Reason":"query","Refs":[],"Recovery":"memory:1"},{"ID":"memory:2","Source":"","OriginalSHA256":"b735cb19a0c954dad505e4c9c2444b6607e5955f6d54ba500c62c1df815790e1","DeliveredSHA256":"b735cb19a0c954dad505e4c9c2444b6607e5955f6d54ba500c62c1df815790e1","Status":"selected","Reason":"query","Refs":[],"Recovery":"memory:2"}]
              GoMemory Context Optimization
              Raw tokens:          440
              Final tokens:        440
              Reducción:            0.00%
              Ahorrados:            0
              Items críticos:       0
              Items relevantes:     2
              Items opcionales:     0
              Duplicados removidos: 0
              Descartados:          0
              Selección: 0; compresión: 0; delta: 0; metadatos: 261; recuperaciones: 0 (contador local, no facturación)
              Ámbito: complete serialized pack; subsequent recoveries not measured here
      --- FAIL: TestHCD003ReceiptOptIn (0.01s)
      FAIL
      FAIL	mem/adapters/primary/cli	0.296s
      FAIL
- 2026-10-30 01:42 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli -run '^TestHCD003ReceiptOptIn$' -count=1 -v` (salida 0)
      === RUN   TestHCD003ReceiptOptIn
      --- PASS: TestHCD003ReceiptOptIn (0.03s)
      PASS
      ok  	mem/adapters/primary/cli	0.328s
- 2026-10-30 01:44 · tarea 2 · manual · vigente
      Binario servido 91fb5daf2bb6 frente a la referencia 6c747abd (89b872d1278a) sobre el corpus de HCD-001 (f766c0d5), 100 observaciones cada uno, ejecutados uno tras otro: 343610 frente a 370960 bytes (-7,37%; antes +14,96%). Por categoría: search 0,0% (idéntico), recovery 0,0%, compaction -19,6% (extracto de HCD-002), no_match +16,7% (aviso explícito «Sin coincidencias» y campo MatchStatus, ~120 bytes). Calidad igual en ambos (25 fallos recovery heredados del corpus de HCD-001), 0 errores, p95 87,3 frente a 87,0 ms. Captura hcd003-go.json SHA256 d8a7ced43f5230af.

## Siguiente
Cerrada con aprobación sobre la revisión d2cababdde22.
