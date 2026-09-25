package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/application/ports"
)

// actualizarGolden regenera los golden de la compresión estructural. Solo se
// usa una vez, con el código de la v2.25.0, para fijar la referencia.
var actualizarGolden = flag.Bool("update-golden-structural", false, "regenera los golden de la compresión estructural")

// T004 — SC-008: el nivel structural produce exactamente la salida de la
// v2.25.0 en todo el corpus. Tiene que pasar antes y después de la feature 033.
func TestCompressionStructuralRegression(t *testing.T) {
	root := repoRootIntegration(t)
	corpus := filepath.Join(root, "tests", "testdata", "compression_corpus")
	golden := filepath.Join(root, "tests", "integration", "testdata", "golden_structural")

	muestras, err := filepath.Glob(filepath.Join(corpus, "*"))
	if err != nil || len(muestras) == 0 {
		t.Fatalf("corpus vacío en %s: %v", corpus, err)
	}
	for _, ruta := range muestras {
		nombre := filepath.Base(ruta)
		if nombre == "README.md" || nombre == "expectations.json" {
			continue
		}
		t.Run(nombre, func(t *testing.T) {
			entrada, err := os.ReadFile(ruta)
			if err != nil {
				t.Fatal(err)
			}
			res, err := compression.StructuralCompressor{}.Compress(string(entrada), ports.CompressionOptions{Level: ports.CompressionStructural})
			if err != nil {
				t.Fatal(err)
			}
			destino := filepath.Join(golden, nombre+".golden")
			if *actualizarGolden {
				if err := os.WriteFile(destino, []byte(res.Content), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			esperado, err := os.ReadFile(destino)
			if err != nil {
				t.Fatalf("falta el golden %s (generarlo con -update-golden-structural sobre la v2.25.0): %v", destino, err)
			}
			if res.Content != string(esperado) {
				t.Errorf("la salida structural de %s cambió respecto a la v2.25.0", nombre)
			}
		})
	}
}
