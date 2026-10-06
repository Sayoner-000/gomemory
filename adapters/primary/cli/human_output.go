package cli

import "mem/adapters/primary/console"

func humanf(format string, args ...any) { console.Printf(format, args...) }
func humanln(args ...any)               { console.Println(args...) }
