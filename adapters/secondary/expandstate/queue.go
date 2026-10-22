// Package expandstate guarda en archivos bajo .memory/ el estado que los
// casos de uso de expand necesitan entre procesos de hook (feature 040,
// research R16): la cola de observaciones de instincts y el estado de
// shield. Todo con permisos 0600; la lectura es tolerante (una línea o un
// archivo ilegibles no rompen nada).
package expandstate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"mem/application/ports"
	"mem/domain"
)

// QueueMaxBytes es el tope de la cola. Al superarlo se descarta la mitad
// más antigua: la cola es transitoria y perder observaciones viejas solo
// ralentiza el aprendizaje, nunca rompe nada.
const QueueMaxBytes = 256 * 1024

// Queue implementa ports.ObservationQueueRepository sobre un NDJSON.
type Queue struct {
	path string
}

var _ ports.ObservationQueueRepository = (*Queue)(nil)

// NewQueue crea la cola del proyecto en <memDir>/expand-observations.ndjson.
func NewQueue(memDir string) *Queue {
	return &Queue{path: filepath.Join(memDir, "expand-observations.ndjson")}
}

// withLock serializa entre procesos las operaciones que escriben la cola.
// Remove, rotate y Sanitize la reemplazan entera (temporal + rename): sin el
// candado, un Append concurrente escribe en el archivo que el rename va a
// descartar y la observación se pierde (A-001). El candado vive en un archivo
// aparte porque el de la cola cambia de inodo en cada reemplazo.
func (q *Queue) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(q.path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(q.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := lockQueueFile(f); err != nil {
		return err
	}
	defer func() { _ = unlockQueueFile(f) }()
	return fn()
}

func (q *Queue) Append(ctx context.Context, obs domain.Observation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	line, err := json.Marshal(obs)
	if err != nil {
		return err
	}
	return q.withLock(func() error { return q.appendLine(ctx, line) })
}

func (q *Queue) appendLine(ctx context.Context, line []byte) error {
	f, err := os.OpenFile(q.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	info, serr := f.Stat()
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	if serr == nil && info.Size() > QueueMaxBytes {
		return q.rotate(ctx)
	}
	return nil
}

// rotate conserva la mitad más reciente de las observaciones.
func (q *Queue) rotate(ctx context.Context) error {
	all, err := q.readAll(ctx)
	if err != nil {
		return err
	}
	return q.write(all[len(all)/2:])
}

func (q *Queue) ReadBatch(ctx context.Context, max int) ([]domain.Observation, error) {
	all, err := q.readAll(ctx)
	if err != nil || max <= 0 || len(all) <= max {
		return all, err
	}
	return all[:max], nil
}

func (q *Queue) Remove(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return q.withLock(func() error { return q.remove(ctx, ids) })
}

func (q *Queue) remove(ctx context.Context, ids []string) error {
	all, err := q.readAll(ctx)
	if err != nil {
		return err
	}
	drop := make(map[string]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}
	kept := all[:0]
	for _, o := range all {
		if !drop[o.ID] {
			kept = append(kept, o)
		}
	}
	return q.write(kept)
}

func (q *Queue) Sanitize(ctx context.Context, redact func(string) string) (int, error) {
	changed := 0
	err := q.withLock(func() error {
		var err error
		changed, err = q.sanitize(ctx, redact)
		return err
	})
	return changed, err
}

func (q *Queue) sanitize(ctx context.Context, redact func(string) string) (int, error) {
	all, err := q.readAll(ctx)
	if err != nil {
		return 0, err
	}
	changed := 0
	for i, o := range all {
		clean, key := redact(o.Sample), domain.ProtectObservationKey(o.Key)
		if clean != o.Sample || key != o.Key {
			all[i].Sample, all[i].Key = clean, key
			changed++
		}
	}
	if changed == 0 {
		return 0, nil
	}
	return changed, q.write(all)
}

func (q *Queue) readAll(ctx context.Context) ([]domain.Observation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(q.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []domain.Observation
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), QueueMaxBytes)
	for sc.Scan() {
		var o domain.Observation
		if json.Unmarshal(sc.Bytes(), &o) != nil || o.ID == "" {
			continue
		}
		out = append(out, o)
	}
	return out, nil
}

// write reemplaza la cola de forma atómica (temporal + rename).
func (q *Queue) write(obs []domain.Observation) error {
	var buf bytes.Buffer
	for _, o := range obs {
		line, err := json.Marshal(o)
		if err != nil {
			continue
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return writeAtomic(q.path, buf.Bytes())
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".expand-tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, 0o600)
	}
	if werr == nil {
		werr = os.Rename(name, path)
	}
	if werr != nil {
		_ = os.Remove(name)
	}
	return werr
}
