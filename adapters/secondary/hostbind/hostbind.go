// Package hostbind enlaza el proceso host de un agente (Claude Code, Codex,
// OpenCode) con la conversación que atiende (HCD-036, Q1).
//
// Los hooks, el servidor MCP y la CLI que el agente lanza desde su terminal son
// descendientes del mismo proceso host. Los hooks traen el id de la
// conversación y lo publican aquí como «pid del host → conversación»; el MCP y
// la CLI, que no lo reciben, lo buscan recorriendo sus procesos ancestros.
//
// Mismo patrón que hostbind de engram: el enlace nunca inventa una identidad,
// solo registra la que dio el host, y vale mientras el host sea el mismo
// proceso (pid y hora de arranque). Un enlace ausente, ilegible o caducado
// significa «sin enlace» y el llamador sigue con su resolución local.
package hostbind

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"mem/application/ports"
)

// maxDepth acota el recorrido de ancestros: un host está a pocos niveles.
const maxDepth = 12

// shells son los intermediarios que un host puede usar para lanzar un hook o
// un comando; el host es el primer ancestro que no es uno de ellos.
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "fish": true, "ksh": true}

type binding struct {
	PID            int    `json:"pid"`
	Start          string `json:"start"`
	ConversationID string `json:"conversation_id"`
	UpdatedAt      string `json:"updated_at"`
}

// Store implementa ports.HostBinding sobre un directorio de enlaces.
type Store struct {
	dir  string
	self func() int
	// info devuelve el padre, la hora de arranque y el nombre de un proceso.
	info func(pid int) (ppid int, start, comm string, err error)
}

var _ ports.HostBinding = (*Store)(nil)

// New crea el almacén de enlaces en dir.
func New(dir string) *Store {
	return &Store{dir: dir, self: os.Getpid, info: psInfo}
}

// Publish anota que el host del proceso actual atiende conversationID.
func (s *Store) Publish(conversationID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if s.dir == "" || conversationID == "" {
		return errors.New("hostbind: hacen falta el directorio y el id de la conversación")
	}
	host, start, ok := s.host()
	if !ok {
		return errors.New("hostbind: no se pudo identificar el proceso host")
	}
	data, err := json.Marshal(binding{PID: host, Start: start, ConversationID: conversationID, UpdatedAt: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".bind-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), s.path(host)); err != nil {
		return err
	}
	s.prune(host)
	return nil
}

// Lookup devuelve la conversación enlazada al primer ancestro con un enlace
// vigente.
func (s *Store) Lookup() (string, bool) {
	if s.dir == "" {
		return "", false
	}
	pid := s.self()
	for i := 0; i < maxDepth && pid > 1; i++ {
		ppid, start, _, err := s.info(pid)
		if err != nil {
			return "", false
		}
		if b, ok := s.read(pid); ok && b.Start == start {
			return b.ConversationID, true
		}
		pid = ppid
	}
	return "", false
}

// host es el primer ancestro del proceso actual que no es un shell.
func (s *Store) host() (int, string, bool) {
	ppid, _, _, err := s.info(s.self())
	for i := 0; err == nil && i < maxDepth && ppid > 1; i++ {
		next, start, comm, ierr := s.info(ppid)
		if ierr != nil {
			return 0, "", false
		}
		if !shells[strings.TrimPrefix(filepath.Base(comm), "-")] {
			return ppid, start, true
		}
		ppid = next
	}
	return 0, "", false
}

func (s *Store) path(pid int) string { return filepath.Join(s.dir, strconv.Itoa(pid)+".json") }

func (s *Store) read(pid int) (binding, bool) {
	data, err := os.ReadFile(s.path(pid))
	if err != nil {
		return binding{}, false
	}
	var b binding
	if json.Unmarshal(data, &b) != nil || b.PID != pid || b.Start == "" || strings.TrimSpace(b.ConversationID) == "" {
		return binding{}, false
	}
	return b, true
}

// prune borra los enlaces cuyo host ya no es el mismo proceso. Best-effort.
func (s *Store) prune(keep int) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil || pid == keep {
			continue
		}
		b, ok := s.read(pid)
		if _, start, _, ierr := s.info(pid); !ok || ierr != nil || start != b.Start {
			_ = os.Remove(s.path(pid))
		}
	}
}

// psInfo lee el padre, la hora de arranque y el nombre de un proceso con ps.
// En Windows no hay enlace: cada proceso usa su conversación local.
func psInfo(pid int) (int, string, string, error) {
	if runtime.GOOS == "windows" {
		return 0, "", "", errors.New("hostbind: no disponible en Windows")
	}
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
