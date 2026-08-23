package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// legacyAgentFiles son los archivos de instrucciones que las versiones
// anteriores de `mem install` creaban o editaban en la raíz del proyecto.
//
// .cursorrules y .windsurfrules quedan DELIBERADAMENTE fuera: el instalador
// nunca los creaba desde cero, solo los editaba si ya existían. Son archivos de
// la persona y retirarlos sería pasarse de la raya.
var legacyAgentFiles = []string{"AGENTS.md", "CLAUDE.md", "CLAUDE.txt"}

// legacyMCPConfigs son las configuraciones MCP por proyecto que el instalador
// creaba para agentes que nadie pidió, dejando una carpeta en la raíz con un
// único JSON dentro. Se desregistra gomemory; el archivo y su carpeta solo
// desaparecen si no queda ningún otro servidor.
var legacyMCPConfigs = []string{
	filepath.Join(".windsurf", "mcp_config.json"),
	filepath.Join(".cline", "mcp_settings.json"),
}

// legacyGeneratedFiles son artefactos generados íntegramente por el instalador,
// recuperables desde el propio binario. No se respaldan.
var legacyGeneratedFiles = []string{"speckit-constitution-gen.md"}

// cleanupLegacyArtifacts retira lo que dejaron las instalaciones anteriores
// (feature 021, US3). La invoca CmdInstall, y por tanto también `mem update`,
// que delega en install.
//
// Idempotente y silenciosa: sobre un proyecto sin artefactos no imprime nada.
// Ningún fallo interrumpe la instalación.
func cleanupLegacyArtifacts(target, memDir string) {
	limpiarArchivosDeAgente(target, memDir)
	limpiarGenerados(target)
	limpiarConfigsDeAgente(target)
}

// limpiarArchivosDeAgente retira los archivos de instrucciones, respaldando
// cada uno ANTES de borrarlo.
//
// El borrado es una decisión explícita del usuario y es destructivo: estos
// archivos pueden contener texto propio. El respaldo es lo que lo hace
// responsable — y si el respaldo no se puede escribir, el original NO se borra.
// Es preferible dejar un archivo obsoleto que perder texto que alguien escribió.
func limpiarArchivosDeAgente(target, memDir string) {
	backupDir := filepath.Join(target, memDir, "backups", "agent-files")

	for _, nombre := range legacyAgentFiles {
		origen := filepath.Join(target, nombre)
		datos, err := os.ReadFile(origen)
		if err != nil {
			continue // no existe: nada que hacer, y nada que informar
		}

		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			fmt.Printf("  ⚠️  %s: no se pudo preparar el respaldo (%v); se conserva el archivo\n", nombre, err)
			continue
		}
		destino := filepath.Join(backupDir, nombre)
		if err := os.WriteFile(destino, datos, 0o644); err != nil {
			fmt.Printf("  ⚠️  %s: no se pudo respaldar (%v); se conserva el archivo\n", nombre, err)
			continue
		}
		if err := os.Remove(origen); err != nil {
			fmt.Printf("  ⚠️  %s: respaldado pero no se pudo eliminar (%v)\n", nombre, err)
			continue
		}
		fmt.Printf("  ✅ %s retirado (respaldado en %s)\n", nombre, destino)
	}
}

// limpiarGenerados elimina los artefactos que el instalador generaba por
// completo. Sin respaldo: son copia literal de una plantilla embebida y el
// contenido vigente se recupera con `mem docs export constitution`.
func limpiarGenerados(target string) {
	for _, nombre := range legacyGeneratedFiles {
		ruta := filepath.Join(target, nombre)
		if _, err := os.Stat(ruta); err != nil {
			continue
		}
		if err := os.Remove(ruta); err != nil {
			fmt.Printf("  ⚠️  %s: no se pudo eliminar (%v)\n", nombre, err)
			continue
		}
		fmt.Printf("  ✅ %s eliminado (la constitución vive en la memoria: ./mem docs show constitution)\n", nombre)
	}
}

// limpiarConfigsDeAgente desregistra gomemory de las configuraciones MCP que el
// instalador creaba sin que nadie las pidiera.
//
// Conservador con lo ajeno: si el archivo registra otros servidores, solo se
// quita la entrada de gomemory y el resto queda intacto. Un JSON que no se puede
// interpretar no se toca — es de alguien más y no sabemos qué contiene.
func limpiarConfigsDeAgente(target string) {
	for _, rel := range legacyMCPConfigs {
		ruta := filepath.Join(target, rel)
		datos, err := os.ReadFile(ruta)
		if err != nil {
			continue
		}

		var cfg map[string]any
		if err := json.Unmarshal(datos, &cfg); err != nil {
			fmt.Printf("  ℹ️  %s: no se pudo interpretar el JSON, se deja intacto\n", rel)
			continue
		}

		servidores, _ := cfg["mcpServers"].(map[string]any)
		if _, tiene := servidores["gomemory"]; !tiene {
			continue // no es nuestro: no se toca
		}
		delete(servidores, "gomemory")

		// Si no queda ningún otro servidor, el archivo entero era nuestro y su
		// carpeta también: se retiran los dos.
		if len(servidores) == 0 {
			dir := filepath.Dir(ruta)
			if err := os.RemoveAll(dir); err != nil {
				fmt.Printf("  ⚠️  %s: no se pudo eliminar (%v)\n", filepath.Base(dir), err)
				continue
			}
			fmt.Printf("  ✅ %s/ eliminada (solo contenía la configuración de gomemory)\n", filepath.Base(dir))
			continue
		}

		cfg["mcpServers"] = servidores
		salida, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			fmt.Printf("  ⚠️  %s: no se pudo serializar (%v), se deja intacto\n", rel, err)
			continue
		}
		if err := os.WriteFile(ruta, salida, 0o644); err != nil {
			fmt.Printf("  ⚠️  %s: no se pudo escribir (%v)\n", rel, err)
			continue
		}
		fmt.Printf("  ✅ %s: entrada de gomemory retirada (se conservan los demás servidores)\n", rel)
	}
}
