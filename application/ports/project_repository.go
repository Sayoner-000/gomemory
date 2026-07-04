package ports

type ProjectRepository interface {
	FindRoot() (string, error)
	EnsureDir(root string) error
	MemDir() string
	DbPath(root string) string
	Init(root string) error
	// Key deriva el identificador estable de un proyecto a partir de su ruta
	// absoluta (ver adapters/secondary/persistence.ProjectKey). Reemplaza el
	// uso disperso de filepath.Base(root) como identidad de proyecto.
	Key(root string) string
	// MigrateLegacy mueve un `.memory/mem.db` legado (modelo de instalación
	// por proyecto anterior a esta feature) al store global. Devuelve si
	// migró algo; force sobrescribe el store global si ya tenía datos.
	MigrateLegacy(root string, force bool) (bool, error)
}
