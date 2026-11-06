package ports

// RevisionDiff da los archivos que cambiaron entre dos revisiones del
// repositorio (HCD-032, Q3). Devuelve error si alguna no es un commit o no hay
// repositorio: el llamador entonces usa el scope declarado.
type RevisionDiff interface {
	ChangedFiles(from, to string) ([]string, error)
}
