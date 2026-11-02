package domain

import "strings"

// IsExactReadCommand preserves the existing automatic-output policy for shell
// reads and diagnostics whose literal representation the agent requested.
func IsExactReadCommand(command string) bool {
	command = strings.TrimSpace(command)
	if isPackRetrieveCommand(command) {
		return true
	}
	for _, prefix := range []string{"sed ", "cat ", "head ", "tail ", "git diff", "git show", "mem doctor"} {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}

// isPackRetrieveCommand matches the CLI recovery of an omitted original, by
// any binary path: compressing that output would omit the original again.
func isPackRetrieveCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) < 3 || fields[1] != "pack" || fields[2] != "retrieve" {
		return false
	}
	return fields[0][strings.LastIndex(fields[0], "/")+1:] == "mem"
}
