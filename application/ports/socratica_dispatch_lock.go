package ports

type SocraticaDispatchLock interface {
	TryAcquire(string) (release func(), acquired bool, err error)
	// Remove drops the lock file of a purged flow; a missing file is not an error.
	Remove(string) error
}
