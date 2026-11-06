//go:build windows

package hostbind

import (
	"fmt"
	"strconv"
	"syscall"
	"unsafe"
)

// processQueryLimitedInformation basta para GetProcessTimes y, a diferencia de
// PROCESS_QUERY_INFORMATION, se concede sobre procesos del mismo usuario aunque
// tengan otro nivel de integridad.
const processQueryLimitedInformation = 0x1000

// psInfo lee el padre, la hora de arranque y el nombre de un proceso con la API
// de Windows (HCD-038). Antes devolvía un error y los hooks sin session_id no
// encontraban la conversación de su host.
func psInfo(pid int) (int, string, string, error) {
	ppid, exe, err := snapshotEntry(pid)
	if err != nil {
		return 0, "", "", err
	}
	start, err := creationTime(pid)
	if err != nil {
		return 0, "", "", err
	}
	// Windows no actualiza el padre cuando muere: su PID puede reutilizarlo otro
	// proceso, posterior al hijo. Ese no es el padre, y se trata como huérfano.
	if ppid > 0 {
		if parentStart, perr := creationTime(ppid); perr != nil || parentStart > start {
			ppid = 0
		}
	}
	return ppid, strconv.FormatInt(start, 10), exe, nil
}

// snapshotEntry busca el proceso en una instantánea de la tabla de procesos.
func snapshotEntry(pid int) (int, string, error) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, "", err
	}
	defer syscall.CloseHandle(snap)
	var pe syscall.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err = syscall.Process32First(snap, &pe); err == nil; err = syscall.Process32Next(snap, &pe) {
		if int(pe.ProcessID) == pid {
			return int(pe.ParentProcessID), syscall.UTF16ToString(pe.ExeFile[:]), nil
		}
	}
	return 0, "", fmt.Errorf("hostbind: proceso %d no encontrado", pid)
}

// creationTime es la hora de arranque del proceso, en ns desde 1970: junto al
// PID identifica al proceso aunque su número se reutilice.
func creationTime(pid int) (int64, error) {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(h)
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return 0, err
	}
	return created.Nanoseconds(), nil
}
