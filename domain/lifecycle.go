package domain

import "sort"

// Desinstalación (feature 034, US2). El plan es la única fuente tanto de la
// simulación (--dry-run) como de la ejecución: lo que se muestra es
// exactamente lo que se retira (SC-004).

// UninstallScope es el alcance de la desinstalación (FR-007).
type UninstallScope string

const (
	UninstallProject UninstallScope = "project"
	UninstallSystem  UninstallScope = "system"
)

// MemoryChoice es qué se hace con la memoria antes de retirar (FR-009).
type MemoryChoice string

const (
	MemoryExport MemoryChoice = "export"
	MemoryDelete MemoryChoice = "delete"
	MemoryKeep   MemoryChoice = "keep"
)

// ItemCategory agrupa el inventario (FR-008) y fija el orden (FR-014).
type ItemCategory string

const (
	CategoryProjectFiles ItemCategory = "project-files"
	CategoryAgentConfig  ItemCategory = "agent-config"
	CategoryMemory       ItemCategory = "memory"
	CategoryBinary       ItemCategory = "binary"
)

// categoryOrder es FR-014: proyectos, configuración global de agentes,
// almacén y, al final, el binario (el propio proceso puede ser ese binario).
var categoryOrder = map[ItemCategory]int{
	CategoryProjectFiles: 0,
	CategoryAgentConfig:  1,
	CategoryMemory:       2,
	CategoryBinary:       3,
}

// ItemKind distingue borrar un archivo o directorio de quitar solo la entrada
// de gomemory dentro de un archivo compartido (FR-012).
type ItemKind string

const (
	KindFile  ItemKind = "file"
	KindDir   ItemKind = "dir"
	KindEntry ItemKind = "entry"
)

type ItemResult string

const (
	ResultPending ItemResult = "pending"
	ResultOK      ItemResult = "ok"
	ResultWarn    ItemResult = "warn"
)

// UninstallItem es una línea del inventario.
type UninstallItem struct {
	Category ItemCategory
	Path     string
	Kind     ItemKind
	// Label describe el elemento para la persona; vacío = la ruta.
	Label string
	Bytes int64
	// Key identifica la base de memoria de un proyecto en el almacén.
	Key    string
	Result ItemResult
	// Detail es el motivo de un warn; Manual, el comando para completarlo.
	Detail, Manual string
}

func NewUninstallItem(c ItemCategory, path string, k ItemKind) UninstallItem {
	return UninstallItem{Category: c, Path: path, Kind: k, Result: ResultPending}
}

// UninstallPlan es el inventario completo de una desinstalación.
type UninstallPlan struct {
	Scope     UninstallScope
	Memory    MemoryChoice
	ExportDir string
	Items     []UninstallItem
	// ScanRoot es la raíz del escaneo de proyectos; ScanSkipped, lo que no se
	// pudo leer.
	ScanRoot    string
	ScanSkipped []string
}

// Sort ordena el inventario según FR-014, estable dentro de cada categoría.
func (p *UninstallPlan) Sort() {
	sort.SliceStable(p.Items, func(i, j int) bool {
		return categoryOrder[p.Items[i].Category] < categoryOrder[p.Items[j].Category]
	})
}

// Execute aplica cada elemento en orden. Un error deja ese elemento en warn
// con su motivo y el recorrido sigue (FR-015). Un elemento ya resuelto antes
// (por ejemplo, en warn porque su exportación falló) no se toca.
func (p *UninstallPlan) Execute(apply func(*UninstallItem) error) {
	for i := range p.Items {
		it := &p.Items[i]
		if it.Result != ResultPending && it.Result != "" {
			continue
		}
		if err := apply(it); err != nil {
			it.Result = ResultWarn
			it.Detail = err.Error()
			continue
		}
		it.Result = ResultOK
	}
}

func (p *UninstallPlan) Warnings() int {
	n := 0
	for _, it := range p.Items {
		if it.Result == ResultWarn {
			n++
		}
	}
	return n
}

// UninstallScanMaxDepth es la profundidad del escaneo por defecto bajo el
// directorio personal (aclaración de la spec, 2026-09-26).
const UninstallScanMaxDepth = 6

// scanSkipDirs son directorios que nunca contienen proyectos de gomemory y
// que harían lento el escaneo: control de versiones, dependencias y cachés.
var scanSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".venv": true, "venv": true,
	"target": true, "dist": true, "build": true, "Library": true, ".cache": true,
	".npm": true, ".cargo": true, ".rustup": true, ".gradle": true, ".m2": true,
	".pyenv": true, ".nvm": true, ".Trash": true, "__pycache__": true,
}

// ShouldSkipScanDir indica si el escaneo debe saltarse un directorio.
func ShouldSkipScanDir(name string) bool { return scanSkipDirs[name] }

// ProjectRegistration asocia un proyecto del almacén global con su ruta en el
// disco (FR-013). Exists es false si la ruta ya no existe: el proyecto es
// huérfano y sus datos del almacén se retiran igualmente.
type ProjectRegistration struct {
	Key    string
	Root   string
	Exists bool
}
