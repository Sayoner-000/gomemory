package main

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// rutaTiposDelSDK localiza la declaración de la superficie de hooks que publica
// OpenCode. No está versionada en el repositorio: llega con las dependencias
// del agente, así que puede no existir en un entorno de integración continua.
func rutaTiposDelSDK() string {
	for _, p := range []string{
		"../../.opencode/node_modules/@opencode-ai/plugin/dist/index.d.ts",
		os.ExpandEnv("$HOME/.config/opencode/node_modules/@opencode-ai/plugin/dist/index.d.ts"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// hooksQueRegistraElComplemento extrae los nombres de hook que el complemento
// declara. Son las claves entre comillas seguidas de `:` en su objeto de
// retorno, más la clave `event`, que no lleva comillas.
func hooksQueRegistraElComplemento(t *testing.T) []string {
	t.Helper()
	datos, err := os.ReadFile("../../infrastructure/plugin/opencode/gomemory.ts")
	if err != nil {
		t.Fatalf("leer el complemento: %v", err)
	}
	texto := string(datos)

	re := regexp.MustCompile(`(?m)^\s{4}"([a-z][a-zA-Z.]+)":\s*async`)
	vistos := map[string]bool{}
	var out []string
	for _, m := range re.FindAllStringSubmatch(texto, -1) {
		if !vistos[m[1]] {
			vistos[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

// TestRiesgo2_LosHooksDelComplementoExistenEnLaInterfazDelAgente cubre FR-017 y
// FR-018 de la especificación 024.
//
// El complemento depende de operaciones que OpenCode marca como experimentales.
// Si las renombra, la inyección del protocolo y del contexto se pierde SIN
// ninguna señal: las rutas de error del complemento absorben el fallo y el
// informe de estado sigue en verde, porque comprueba que el archivo exista y no
// que funcione.
//
// Este contrato convierte ese cambio silencioso en un fallo de la batería,
// antes de publicar y no en la máquina de otra persona.
func TestRiesgo2_LosHooksDelComplementoExistenEnLaInterfazDelAgente(t *testing.T) {
	ruta := rutaTiposDelSDK()
	if ruta == "" {
		t.Skip("la interfaz publicada por OpenCode no está disponible en este entorno; el contrato se omite en vez de fallar por una causa ajena al código")
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer la interfaz del agente: %v", err)
	}
	superficie := string(datos)

	registrados := hooksQueRegistraElComplemento(t)
	if len(registrados) == 0 {
		t.Fatal("no se detectó ningún hook en el complemento: el extractor dejó de reconocer su forma")
	}

	for _, h := range registrados {
		if !strings.Contains(superficie, `"`+h+`"?:`) {
			t.Errorf("el complemento registra %q y la interfaz publicada por OpenCode ya no lo declara: la inyección se perdería en silencio", h)
		}
	}
	t.Logf("hooks verificados contra la interfaz del agente: %s", strings.Join(registrados, ", "))
}
