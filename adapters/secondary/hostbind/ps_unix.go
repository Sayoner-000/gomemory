//go:build !windows

package hostbind

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// psInfo lee el padre, la hora de arranque y el nombre de un proceso con ps.
func psInfo(pid int) (int, string, string, error) {
	out, err := exec.Command("ps", "-o", "ppid=,lstart=,comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, "", "", err
	}
	// «PPID  Día Mes DD HH:MM:SS AAAA  comando»: lstart ocupa cinco campos.
	f := strings.Fields(string(out))
	if len(f) < 7 {
		return 0, "", "", errors.New("hostbind: salida de ps inesperada")
	}
	ppid, err := strconv.Atoi(f[0])
	if err != nil {
		return 0, "", "", err
	}
	return ppid, strings.Join(f[1:6], " "), strings.Join(f[6:], " "), nil
}
