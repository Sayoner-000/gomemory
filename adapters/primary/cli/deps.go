package cli

import "mem/adapters/primary/cli/clikit"

// Deps vive en clikit para que los paquetes de cada slice bajo cli/ lo
// compartan sin importar cli, que a su vez los importa para despacharlos.
type Deps = clikit.Deps
