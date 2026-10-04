package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	"mem/adapters/primary/console"
	"mem/adapters/primary/setup"
	"mem/application/ports"
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
	yes := fs.Bool("yes", false, "No pedir confirmación (también -y)")
	fs.BoolVar(yes, "y", false, "No pedir confirmación")
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
		// --check también renueva la caché del aviso de versión (FR-028).
		if !updateCheckDisabled("") {
			refreshUpdateCache(context.Background(), releasePortOf(deps), updateCheckRepoOf(deps), time.Now())
		}
		return
	}

	if current == target {
		fmt.Println("Ya estás actualizado, nada que hacer.")
		// El aviso de hooks duplicados recomienda `mem update`: también al día
		// tiene que corregirlos (feature 035, FR-001).
		if root, err := deps.ProjectRepo.FindRoot(); err == nil {
			dedupProjectHooksReport(root)
		}
		return
	}

	self, err := os.Executable()
	if err != nil {
		fail("obtener ruta del binario actual: %v", err)
	}
	root, rootErr := deps.ProjectRepo.FindRoot()

	// El destino es el global cuando se ejecuta desde otra copia (FR-006): si
	// no, `./mem update` dejaba el global viejo, que es el que usan hooks y MCP.
	dest := self
	if rootErr == nil {
		dest, _ = resolveUpdateTarget(self, root)
	}
	if dest != self {
		fmt.Printf("  🎯 Se actualiza el binario global %s\n", dest)
	}
	// FR-025: con terminal interactiva se confirma antes de sustituir nada.
	mode := console.DetectMode(console.DetectEnv(), *yes)
	ui := console.New(mode)
	if ok, err := confirmUpdate(ui, current, target, dest); err != nil || !ok {
		fmt.Println("Actualización cancelada. No se modificó nada.")
		return
	}

	// FR-024: cada paso deja su ✓/⚠/✗ y la actualización termina siempre con
	// el resumen, también cuando un paso la aborta.
	var steps []console.StepResult
	var tmpDir string
	step := func(name, detail string, status console.StepStatus, manual string) {
		steps = append(steps, console.StepResult{Name: name, Detail: detail, Status: status, Manual: manual})
	}
	summary := func() {
		fmt.Println("\nResumen:")
		reporter := console.NewReporter(os.Stdout, mode == console.ModeRich)
		for _, s := range steps {
			reporter.Done(s)
		}
	}
	abort := func(name, detail, manual string) {
		step(name, detail, console.StepFail, manual)
		summary()
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
		os.Exit(1)
	}

	if err := checkReplaceable(dest); err != nil {
		fmt.Printf("  ⚠️  No se puede escribir en %s: %v\n", filepath.Dir(dest), err)
		fmt.Println("      Actualiza con permisos: sudo mem update (o reinstala con scripts/install.sh)")
		abort("Binario", "no se puede escribir en "+filepath.Dir(dest), "sudo mem update")
	}

	tmpDir, err = os.MkdirTemp("", "gomemory-update-*")
	if err != nil {
		abort("Descarga", fmt.Sprintf("crear directorio temporal: %v", err), "")
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	asset := assetName()
	url := fmt.Sprintf("%s/download/%s/%s", releaseDownloadBase, target, asset)
	archivePath := filepath.Join(tmpDir, asset)

	fmt.Printf("  ⬇️  Descargando %s\n", url)
	if err := downloadFile(client, url, archivePath); err != nil {
		abort("Descarga", err.Error(), "")
	}
	step("Descarga", asset+" "+target, console.StepOK, "")

	// FR-032: sin checksum verificado no se toca el binario instalado.
	fmt.Println("  🔐 Verificando checksum...")
	if err := verifyReleaseChecksum(context.Background(), releasePortOf(deps), target, asset, archivePath); err != nil {
		fmt.Printf("  ✗ checksum: %v\n", err)
		fmt.Println("      No se modificó el binario instalado.")
		abort("Checksum", err.Error()+"; no se modificó el binario instalado", "")
	}
	fmt.Println("  ✅ Checksum verificado")
	step("Checksum", "verificado", console.StepOK, "")

	fmt.Println("  📦 Extrayendo binario...")
	newBin, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		abort("Binario", fmt.Sprintf("extraer: %v", err), "")
	}

	fmt.Println("  🔄 Reemplazando binario actual...")
	if err := replaceSelf(dest, newBin); err != nil {
		abort("Binario", fmt.Sprintf("reemplazar %s: %v", dest, err), "")
	}
	fmt.Printf("  ✅ Binario actualizado a %s\n", target)
	step("Binario", dest+" ("+current+" → "+target+")", console.StepOK, "")

	if rootErr != nil {
		fmt.Println("  ℹ️  No se detectó un proyecto con .memory/ en el cwd; solo se actualizó el binario.")
		summary()
		return
	}

	// Con el global al día, una copia local del proyecto sobra (FR-003).
	if dest != self {
		if c, retired, err := retireLocalCopy(root, dest); retired {
			fmt.Printf("  ✅ %s\n", retiredNotice(c, dest))
			step("Copia local", retiredNotice(c, dest), console.StepOK, "")
		} else if err != nil {
			fmt.Printf("  ⚠️  No se pudo retirar %s: %v → bórralo a mano\n", c.Path, err)
			step("Copia local", fmt.Sprintf("no se pudo retirar %s: %v", c.Path, err), console.StepWarn, "rm "+c.Path)
		}
	}

	// El refresco lo ejecuta el binario recién instalado: `self` puede ser la
	// copia que se acaba de retirar.
	fmt.Println("  🔌 Refrescando integración del proyecto (hooks, MCP, permisos)...")
	if err := runIn(root, dest, "install", root); err != nil {
		fmt.Printf("  ⚠️  No se pudo refrescar la integración automáticamente: %v\n", err)
		fmt.Printf("      Ejecuta manualmente: %s install %s\n", dest, root)
		step("Integración del proyecto", err.Error(), console.StepWarn, dest+" install "+root)
		summary()
		return
	}
	fmt.Println("  ✅ Integración del proyecto refrescada")
	step("Integración del proyecto", "refrescada", console.StepOK, "")
	summary()
}

// confirmUpdate pide confirmación con la consola; sin ella (--yes, sin TTY)
// continúa.
func confirmUpdate(ui console.UI, current, target, dest string) (bool, error) {
	if ui == nil {
		return true, nil
	}
	return ui.Confirm(fmt.Sprintf("Actualizar %s: %s → %s. ¿Continuar?", dest, current, target), true)
}

// verifyReleaseChecksum compara el SHA-256 del archivo descargado con el que
// publica la release en checksums.txt.
func verifyReleaseChecksum(ctx context.Context, rel ports.ReleasePort, tag, asset, path string) error {
	want, err := rel.Checksum(ctx, tag, asset)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("no coincide (esperado %s, descargado %s)", want, got)
	}
	return nil
}

// resolveUpdateTarget devuelve el binario que debe sustituir `mem update` y si
// se redirigió al global porque el ejecutable en curso es otra copia (R6).
func resolveUpdateTarget(self, root string) (string, bool) {
	global, ok := resolveGlobalBinary(root)
	if !ok {
		return self, false
	}
	if si, err := os.Stat(self); err == nil {
		if gi, err := os.Stat(global); err == nil && os.SameFile(si, gi) {
			return self, false
		}
	}
	return global, true
}

// checkReplaceable comprueba antes de descargar que se puede escribir junto al
// binario destino (replaceSelf crea el respaldo y el nuevo ahí mismo).
func checkReplaceable(path string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".mem-update-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("descarga respondió %d (¿existe el asset %s?)", resp.StatusCode, filepath.Base(destPath))
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

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
	if err := os.Chmod(destPath, 0755); err != nil {
		return "", err
	}
	return destPath, nil
}

func extractBinaryFromTarGz(archivePath, binName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

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
		defer func() { _ = out.Close() }()
		_, err = io.Copy(out, tr)
		return err
	}
}

func extractBinaryFromZip(archivePath, binName, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if filepath.Base(f.Name) != binName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() { _ = rc.Close() }()

		out, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer func() { _ = out.Close() }()
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
			"windows bloquea el binario en ejecución. El nuevo binario quedó en %s.\n"+
				"Cierra este proceso y ejecuta:\n"+
				"  move /Y \"%s\" \"%s\"",
			finalPath, finalPath, currentPath,
		)
	}

	backup := currentPath + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(currentPath, backup); err != nil {
		return fmt.Errorf("respaldar binario actual: %w", err)
	}
	if err := copyFile(newPath, currentPath); err != nil {
		_ = os.Rename(backup, currentPath)
		return fmt.Errorf("instalar binario nuevo: %w", err)
	}
	if err := os.Chmod(currentPath, 0755); err != nil {
		return err
	}
	_ = os.Remove(backup)
	return nil
}

// dedupProjectHooksReport retira del proyecto los hooks de Claude Code que ya
// cubre el ámbito global e informa el resultado. Best-effort.
func dedupProjectHooksReport(root string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	removed, err := setup.DedupClaudeProjectHooks(home, root)
	switch {
	case err != nil:
		fmt.Printf("  ⚠️  No se pudieron retirar los hooks duplicados: %v\n", err)
	case len(removed) > 0:
		fmt.Printf("  ✅ Hooks duplicados retirados del proyecto (ya los cubre el global): %s\n", strings.Join(removed, ", "))
	}
}
