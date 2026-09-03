package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// proyectoConOctopus crea un directorio de proyecto con el módulo encendido.
func proyectoConOctopus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	memDir := filepath.Join(dir, ".memory")
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		t.Fatalf("crear .memory: %v", err)
	}
	escribirArchivo(t, filepath.Join(memDir, "settings.json"), `{"octopus_enabled": true}`)
	return dir
}

func escribirArchivo(t *testing.T, ruta, contenido string) {
	t.Helper()
	if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
		t.Fatalf("escribir %s: %v", ruta, err)
	}
}

func repoRootIntegration(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..")
}

// storeAislado devuelve un entorno cuyo store global de gomemory es un
// directorio temporal, con EXACTAMENTE una carpeta de proyecto.
//
// Usa GOMEMORY_DATA_HOME, la variable que el propio persistence.DataHome()
// consulta primero y que existe justo para esto. Sobrescribir HOME no basta:
// DataHome mira la variable antes que el directorio de usuario.
//
// Las alternativas que se probaron antes fallan de formas silenciosas: coger "la
// base más reciente del store" devuelve la de otro proyecto (el store real tiene
// cientos de carpetas, y cualquier proceso de gomemory deja la suya más fresca —
// esta prueba llegó a contar 126 filas ajenas), y calcular la clave a mano
// obliga a replicar ProjectKey, donde un desajuste da un falso negativo.
func storeAislado(t *testing.T) []string {
	t.Helper()
	return append(os.Environ(), "GOMEMORY_DATA_HOME="+t.TempDir())
}

// baseDelStore abre el único mem.db que exista bajo el HOME indicado. Devuelve
// nil si no hay ninguno, y eso NO es un fallo: gomemory crea la base de forma
// perezosa, en la primera escritura. Un proyecto donde nadie escribió nada
// simplemente no tiene base — que para una prueba de huella cero es la evidencia
// más fuerte posible.
func baseDelStore(t *testing.T, env []string) *sql.DB {
	t.Helper()

	var dataHome string
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GOMEMORY_DATA_HOME="); ok {
			dataHome = v
		}
	}
	if dataHome == "" {
		t.Fatal("el entorno no declara GOMEMORY_DATA_HOME")
	}

	rutas, err := filepath.Glob(filepath.Join(dataHome, "projects", "*", "mem.db"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(rutas) == 0 {
		return nil
	}
	if len(rutas) > 1 {
		sort.Strings(rutas)
		t.Fatalf("el store aislado debería tener una sola base, hay %d: %v", len(rutas), rutas)
	}

	db, err := sql.Open("sqlite", rutas[0])
	if err != nil {
		t.Fatalf("abrir %s: %v", rutas[0], err)
	}
	return db
}
