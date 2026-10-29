package ports

import "context"

type HCDUnitOfWork interface {
	WithTransaction(context.Context, func(HCDRepository) error) error
}
