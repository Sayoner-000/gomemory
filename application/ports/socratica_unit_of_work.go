package ports

import "context"

type SocraticaUnitOfWork interface {
	WithTransaction(context.Context, func(SocraticaRepository) error) error
}
