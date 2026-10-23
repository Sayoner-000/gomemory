package main

import "runtime"

// exeName añade la extensión que Windows exige para ejecutar el binario
// compilado; sin ella exec no lo encuentra.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
