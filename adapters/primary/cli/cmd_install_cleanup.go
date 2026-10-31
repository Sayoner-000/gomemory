package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mem/adapters/primary/setup"
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

// limpiarArchivosDeAgente retira solo los bloques completos de gomemory: un
// archivo sin ellos es de la persona y no se toca, y un bloque sin cierre se
// informa. Copia el original antes de cambiarlo; si no puede, lo conserva.
// Ningún fallo interrumpe la instalación: cada uno ya se informó.
func limpiarArchivosDeAgente(target, memDir string) {
	backupDir := filepath.Join(target, memDir, "backups", "agent-files")
	for _, nombre := range legacyAgentFiles {
		origen, err := setup.ProjectConfigPath(target, nombre)
		if err != nil {
			continue
		}
		_, _ = retiraBloqueDeProyecto(origen, nombre, backupDir)
	}
}

// retiraBloqueDeProyecto aplica la retirada a un archivo y reporta si cambió.
// backupDir vacío omite la copia. Un archivo inexistente o sin bloques no es un
// fallo; el error distingue lo que quedó sin retirar, ya informado por consola.
func retiraBloqueDeProyecto(path, nombre, backupDir string) (bool, error) {
	datos, err := os.ReadFile(path)
	if err != nil {
		return false, nil // no existe: nada que hacer, y nada que informar
	}
	content := string(datos)
	rest, err := stripMarkedBlocks(content)
	if err != nil {
		fmt.Printf("  ⚠️  %s no se modificó: %v\n", nombre, err)
		return false, fmt.Errorf("%s no se modificó: %w", nombre, err)
	}
	if rest == content {
		return false, nil
	}
	aviso := ""
	if backupDir != "" {
		destino, err := copiaUnica(backupDir, nombre, datos)
		if err != nil {
			fmt.Printf("  ⚠️  %s: no se pudo respaldar (%v); se conserva el archivo\n", nombre, err)
			return false, fmt.Errorf("%s: no se pudo respaldar: %w", nombre, err)
		}
		aviso = " (respaldado en " + destino + ")"
	}
	if err := writeRetired(path, rest); err != nil {
		fmt.Printf("  ⚠️  %s: no se pudo retirar el bloque (%v)\n", nombre, err)
		return false, fmt.Errorf("%s: no se pudo retirar el bloque: %w", nombre, err)
	}
	fmt.Printf("  ✅ %s: bloque de gomemory retirado%s\n", nombre, aviso)
	return true, nil
}

// copiaUnica crea la copia sin reemplazar ninguna anterior.
func copiaUnica(dir, nombre string, datos []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	destino := filepath.Join(dir, nombre)
	if _, err := os.Lstat(destino); err == nil {
		destino += "." + time.Now().Format("20060102-150405.000000000")
	}
	if err := writeExclusive(destino, datos, 0o644); err != nil {
		return "", err
	}
	return destino, nil
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
		ruta, err := setup.ProjectConfigPath(target, rel)
		if err != nil {
			continue
		}
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
		if err := setup.WriteConfigFile(ruta, salida); err != nil {
			fmt.Printf("  ⚠️  %s: no se pudo escribir (%v)\n", rel, err)
			continue
		}
		fmt.Printf("  ✅ %s: entrada de gomemory retirada (se conservan los demás servidores)\n", rel)
	}
}
