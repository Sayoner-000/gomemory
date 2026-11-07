//go:build !windows

package hostbind

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// psInfo lee el padre, la hora de arranque y el nombre de un proceso con ps.
func psInfo(pid int) (int, string, string, error) {
	if pid <= 0 {
		return 0, "", "", errors.New("hostbind: pid inválido")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-o", "ppid=,lstart=,comm=", "-p", strconv.Itoa(pid))
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.Output()
	if err != nil {
		// No fabricar start ni reutilizar un PID sin verificarlo: el resolver
		// puede usar ConversationEnv si la inspección no está disponible.
		return 0, "", "", fmt.Errorf("hostbind: inspeccionar proceso %d: %w", pid, err)
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
