package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"mem/version"
)

// releaseAPIBase y releaseRepo son var (no const) para poder apuntarlos a un
// httptest.Server en tests sin tocar la red real. Los tests de integración
// corren `mem update` como subproceso, así que además de sobreescribir estas
// vars in-process (tests unitarios del mismo paquete), se puede overridear
// por entorno (GOMEMORY_RELEASE_API_BASE / GOMEMORY_RELEASE_DOWNLOAD_BASE)
// para alcanzar un subproceso real.
var releaseAPIBase = envOr("GOMEMORY_RELEASE_API_BASE", "https://api.github.com")
var releaseRepo = "Sayoner-000/gomemory"

// releaseDownloadBase es la base de descarga de assets (releases de GitHub).
var releaseDownloadBase = envOr("GOMEMORY_RELEASE_DOWNLOAD_BASE", "https://github.com/"+releaseRepo+"/releases")

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func CmdUpdate(deps *Deps, args []string) {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	versionFlag := fs.String("version", "", "Versión específica a instalar (ej. v1.8.0), default: latest")
	checkOnly := fs.Bool("check", false, "Solo mostrar versión actual vs. disponible, sin instalar")
	if err := fs.Parse(args); err != nil {
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}

	target := *versionFlag
	if target == "" {
		latest, err := latestReleaseTag(client)
		if err != nil {
			fail("no se pudo consultar la última versión: %v", err)
		}
		target = latest
	}
	if !strings.HasPrefix(target, "v") {
		target = "v" + target
	}

	current := "v" + strings.TrimPrefix(version.Version, "v")
	fmt.Printf("Actual: %s → Disponible: %s\n", current, target)

	if *checkOnly {
		if current == target {
			fmt.Println("Ya estás en la última versión.")
		}
		return
	}

	if current == target {
		fmt.Println("Ya estás actualizado, nada que hacer.")
		return
	}

	self, err := os.Executable()
	if err != nil {
		fail("obtener ruta del binario actual: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "gomemory-update-*")
	if err != nil {
		fail("crear directorio temporal: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	asset := assetName()
	url := fmt.Sprintf("%s/download/%s/%s", releaseDownloadBase, target, asset)
	archivePath := filepath.Join(tmpDir, asset)

	fmt.Printf("  ⬇️  Descargando %s\n", url)
	if err := downloadFile(client, url, archivePath); err != nil {
		fail("descargar release: %v", err)
	}

	fmt.Println("  📦 Extrayendo binario...")
	newBin, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		fail("extraer binario: %v", err)
	}

	fmt.Println("  🔄 Reemplazando binario actual...")
	if err := replaceSelf(self, newBin); err != nil {
		fail("reemplazar binario: %v", err)
	}
	fmt.Printf("  ✅ Binario actualizado a %s\n", target)

	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fmt.Println("  ℹ️  No se detectó un proyecto con .memory/ en el cwd; solo se actualizó el binario.")
		return
	}

	fmt.Println("  🔌 Refrescando integración del proyecto (hooks, MCP, permisos)...")
	if err := runIn(root, self, "install", root); err != nil {
		fmt.Printf("  ⚠️  No se pudo refrescar la integración automáticamente: %v\n", err)
		fmt.Printf("      Ejecuta manualmente: %s install %s\n", self, root)
		return
	}
	fmt.Println("  ✅ Integración del proyecto refrescada")
}

func assetName() string {
	return assetNameFor(runtime.GOOS, runtime.GOARCH)
}

// assetNameFor replica el naming de scripts/install.sh (mem_${os}_${arch}.tar.gz)
// e install.ps1 (mem_windows_${arch}.zip).
func assetNameFor(goos, goarch string) string {
	if goos == "windows" {
		return fmt.Sprintf("mem_windows_%s.zip", goarch)
	}
	return fmt.Sprintf("mem_%s_%s.tar.gz", goos, goarch)
}

func latestReleaseTag(client *http.Client) (string, error) {
	url := releaseAPIBase + "/repos/" + releaseRepo + "/releases/latest"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "gomemory/"+version.Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API respondió %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("respuesta sin tag_name")
	}
	return payload.TagName, nil
}

func downloadFile(client *http.Client, url, destPath string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "gomemory/"+version.Version)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("descarga respondió %d (¿existe el asset %s?)", resp.StatusCode, filepath.Base(destPath))
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return out.Sync()
}

// extractBinary extrae el binario "mem"/"mem.exe" del archivo descargado
// (tar.gz en unix, zip en Windows) y devuelve su ruta dentro de destDir.
func extractBinary(archivePath, destDir string) (string, error) {
	binName := "mem"
	if runtime.GOOS == "windows" {
		binName = "mem.exe"
	}
	destPath := filepath.Join(destDir, binName)

	if strings.HasSuffix(archivePath, ".zip") {
		if err := extractBinaryFromZip(archivePath, binName, destPath); err != nil {
			return "", err
		}
	} else {
		if err := extractBinaryFromTarGz(archivePath, binName, destPath); err != nil {
			return "", err
		}
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return "", fmt.Errorf("el archivo no contiene el binario %s: %w", binName, err)
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("el binario extraído %s está vacío", binName)
	}
	os.Chmod(destPath, 0755)
	return destPath, nil
}

func extractBinaryFromTarGz(archivePath, binName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("binario %s no encontrado en el tar.gz", binName)
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) != binName {
			continue
		}
		out, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, tr)
		return err
	}
}

func extractBinaryFromZip(archivePath, binName, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) != binName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		out, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, rc)
		return err
	}
	return fmt.Errorf("binario %s no encontrado en el zip", binName)
}

// replaceSelf reemplaza el binario en ejecución de forma atómica. En unix se
// puede renombrar sobre un binario que está corriendo (el inode viejo sigue
// vivo hasta que el proceso actual termina). En Windows el ejecutable está
// bloqueado mientras corre, así que se deja el nuevo binario listo y se avisa
// al usuario que complete el reemplazo manualmente.
func replaceSelf(currentPath, newPath string) error {
	if runtime.GOOS == "windows" {
		finalPath := currentPath + ".new"
		if err := copyFile(newPath, finalPath); err != nil {
			return err
		}
		return fmt.Errorf(
			"Windows bloquea el binario en ejecución. El nuevo binario quedó en %s.\n"+
				"Cierra este proceso y ejecuta:\n"+
				"  move /Y \"%s\" \"%s\"",
			finalPath, finalPath, currentPath,
		)
	}

	backup := currentPath + ".old"
	os.Remove(backup)
	if err := os.Rename(currentPath, backup); err != nil {
		return fmt.Errorf("respaldar binario actual: %w", err)
	}
	if err := copyFile(newPath, currentPath); err != nil {
		os.Rename(backup, currentPath)
		return fmt.Errorf("instalar binario nuevo: %w", err)
	}
	os.Chmod(currentPath, 0755)
	os.Remove(backup)
	return nil
}
