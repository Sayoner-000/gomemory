package ports

// LocaleCatalog resuelve mensajes y plantillas localizadas sin acoplar los
// casos de uso al sistema de archivos embebido.
type LocaleCatalog interface {
	Supports(code string) bool
	SupportedCodes() []string
	Get(code, key string, params ...map[string]string) string
	GetTemplate(code, name string) (string, error)
}
