package ports

type ProjectRepository interface {
	FindRoot() (string, error)
	EnsureDir(root string) error
	MemDir() string
	DbPath(root string) string
	Init(root string) error
}
