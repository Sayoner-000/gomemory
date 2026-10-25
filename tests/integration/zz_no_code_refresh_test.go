package main

import "os"

// Cuando el snapshot del proveedor de grafo externo venció, los comandos que
// arman contexto (plan-context, hooks) lanzan `mem code-refresh` en segundo
// plano. Ese hijo escribe .memory/code_provider_snapshot*.json después de que
// el comando termina y, en Windows, deja sin poder borrar el directorio de
// t.TempDir(): «The directory is not empty». Igual que update-check, se apaga
// para toda la suite.
func init() {
	_ = os.Setenv("GOMEMORY_NO_CODE_REFRESH", "1")
}
