package ports

// RevisionFiles lee el contenido de texto de un alcance tal como está en una
// revisión del repositorio (HCD-035), para medir el ahorro de una revisión
// enlazada. Devuelve error si la revisión no es un commit o no hay repositorio.
type RevisionFiles interface {
	TextFilesAt(revision string, scope []string) (map[string]string, error)
}
