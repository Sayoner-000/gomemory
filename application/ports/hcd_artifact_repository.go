package ports

import (
	"context"
	"mem/domain"
)

type HCDArtifactRepository interface {
	Read(context.Context, string, string) ([]byte, error)
	Write(context.Context, string, string, []byte) error
	Delete(context.Context, string, string) error
	Hash(context.Context, string, string) (string, error)
	Exists(context.Context, string, string) (bool, error)
	ListUnits(context.Context) ([]string, error)
	Root() string
}

type HCDContextRepository interface {
	ActiveHCDUnit(context.Context) (domain.Unit, error)
}

type HCDContextDetailRepository interface {
	ActiveHCDContext(context.Context) (domain.HCDActiveContext, error)
}

type HCDApproachReader interface {
	ApproachCandidates(context.Context) ([]domain.HCDApproachCandidate, error)
}

// HCDLegacySourceReader lee, sin escribir, los artefactos Spec Kit de una carpeta bajo specs/.
type HCDLegacySourceReader interface {
	ReadLegacySources(context.Context, string) (map[string][]byte, error)
}
