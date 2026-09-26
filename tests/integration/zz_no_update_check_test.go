package main

import "os"

// Desde la feature 034, session-start lanza `mem update-check` en segundo
// plano cuando la caché de versiones venció. En la suite eso consultaría la
// API real de GitHub (constitución §10: sin red pública en pruebas) y
// escribiría en el almacén temporal durante su limpieza. Las pruebas que
// ejercitan el aviso lo reactivan explícitamente en su entorno aislado.
func init() {
	_ = os.Setenv("GOMEMORY_NO_UPDATE_CHECK", "1")
}
