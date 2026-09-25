package native

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"mem/adapters/secondary/compression"
	"mem/application/ports"
	"mem/domain"
)

// Engine implementa ports.Compressor. Con CompressionNone o
// CompressionStructural delega en la compresión estructural de siempre, así
// que su salida es idéntica a la de la v2.25.0 (INV-C2). Con CompressionMax
// aplica el motor nativo completo.
//
// ports.Compressor no recibe context.Context (excepción E-002 de la
// constitución). Para llamar a los puertos nuevos, que sí lo reciben, el Engine
// crea un context.WithTimeout acotado por domain.StoreTimeout.
type Engine struct {
	store   ports.OriginalStoreRepository
	stats   ports.CompressionStatsRepository
	tuning  ports.CompressionTuningRepository
	project string
	// MinTokens sobrescribe domain.CompressionMinTokens si es positivo.
	MinTokens int
	// StoreTimeout sobrescribe domain.StoreTimeout si es positivo (el hook de
	// salidas de herramientas usa un plazo más corto).
	StoreTimeout time.Duration
}

// NewEngine construye el motor. store es obligatorio para comprimir con
// pérdida: sin él, toda petición max degrada a estructural (no se promete una
// ref que no se pueda recuperar). stats y tuning pueden ser nil.
func NewEngine(store ports.OriginalStoreRepository, stats ports.CompressionStatsRepository, tuning ports.CompressionTuningRepository, project string) *Engine {
	return &Engine{store: store, stats: stats, tuning: tuning, project: project}
}

// compressor por tipo: devuelve la salida y el número de omisiones. ok=false
// significa que el compresor no pudo tratar el bloque.
type typeCompressor func(content, lang, ref string, th domain.Thresholds) (string, int, bool)

func compressorFor(typ domain.ContentType) typeCompressor {
	switch typ {
	case domain.ContentJSON:
		return func(c, lang, ref string, th domain.Thresholds) (string, int, bool) {
			out, n := compressJSON(c, lang, ref, th)
			return out, n, true
		}
	case domain.ContentCode:
		return func(c, lang, ref string, th domain.Thresholds) (string, int, bool) {
			switch lang {
			case domain.LangGo:
				if out, n, ok := compressGo(c, ref, th); ok {
					return out, n, true
				}
				return compressBraces(c, ref, th)
			case domain.LangPython:
				return compressPython(c, ref, th)
			case domain.LangSQL:
				return compressSQL(c, ref, th)
			default:
				return compressBraces(c, ref, th)
			}
		}
	case domain.ContentLog:
		return func(c, _, ref string, th domain.Thresholds) (string, int, bool) {
			out, n := compressLog(c, ref, th)
			return out, n, true
		}
	case domain.ContentDiff:
		return func(c, _, ref string, th domain.Thresholds) (string, int, bool) {
			out, n := compressDiff(c, ref, th)
			return out, n, true
		}
	case domain.ContentTable:
		return func(c, _, ref string, th domain.Thresholds) (string, int, bool) {
			out, n := compressTable(c, ref, th)
			return out, n, true
		}
	default:
		return func(c, _, ref string, th domain.Thresholds) (string, int, bool) {
			out, n := compressProse(c, ref, th)
			return out, n, true
		}
	}
}

// pendingOriginal es un bloque comprimido con pérdida cuyo original hay que
// guardar antes de entregar la salida.
type pendingOriginal struct {
	ref, content, typ, compressor string
	start, end                    int
	storedRef                     string
}

func (e *Engine) ctx() (context.Context, context.CancelFunc) {
	d := e.StoreTimeout
	if d <= 0 {
		d = domain.StoreTimeout
	}
	return context.WithTimeout(context.Background(), d)
}

func (e *Engine) Compress(input string, opts ports.CompressionOptions) (res ports.CompressionResult, err error) {
	if opts.Level != ports.CompressionMax {
		r, err := compression.StructuralCompressor{}.Compress(input, opts)
		if opts.Level == ports.CompressionNone {
			r.Compressor = "none"
		} else {
			r.Compressor = "structural"
		}
		return r, err
	}

	start := time.Now()
	defer func() {
		res.LatencyMicros = time.Since(start).Microseconds()
		e.record(res)
	}()

	structural, _ := compression.StructuralCompressor{}.Compress(input, ports.CompressionOptions{Level: ports.CompressionStructural})
	fallback := func(reason string) ports.CompressionResult {
		r := structural
		r.Compressor = "structural"
		r.StructuralTokens = structural.Tokens
		r.FallbackReason = reason
		return r
	}

	rawTokens := approxTokens(input)
	minTokens := e.MinTokens
	if minTokens <= 0 {
		minTokens = domain.CompressionMinTokens
	}
	switch {
	case len(input) > domain.CompressionMaxInputBytes:
		return fallback("too_large"), nil
	case rawTokens < minTokens:
		// FR-004: un bloque pequeño sale sin ningún cambio.
		return ports.CompressionResult{Content: input, RawTokens: rawTokens, Tokens: rawTokens, StructuralTokens: structural.Tokens, Compressor: "none"}, nil
	case domain.IsPrivateContent(input):
		// FR-024: lo privado no genera originales recuperables.
		return fallback("private"), nil
	case e.store == nil:
		return fallback("store_unavailable"), nil
	}

	var out strings.Builder
	var pending []pendingOriginal
	omissions := 0
	types := map[domain.ContentType]bool{}
	guardFailed := false
	for _, b := range segment(input) {
		content, n, name, gerr := e.compressBlock(b)
		if gerr != nil {
			guardFailed = true
		}
		types[b.typ] = true
		if b.fenced {
			out.WriteString(b.open)
			start := out.Len()
			out.WriteString(content)
			end := out.Len()
			out.WriteString(b.close)
			if n > 0 {
				omissions += n
				pending = append(pending, pendingOriginal{ref: refOf(b.content), content: b.content, typ: string(b.typ), compressor: name, start: start, end: end})
			}
		} else {
			start := out.Len()
			out.WriteString(content)
			if n > 0 {
				omissions += n
				pending = append(pending, pendingOriginal{ref: refOf(b.content), content: b.content, typ: string(b.typ), compressor: name, start: start, end: out.Len()})
			}
		}
	}

	result := out.String()
	tokens := approxTokens(result)
	if len(pending) == 0 || tokens >= structural.Tokens {
		if guardFailed {
			return fallback("literal_guard"), nil
		}
		return fallback("no_gain"), nil
	}

	// Guardar los originales ANTES de entregar: una ref sin original sería una
	// promesa rota (INV-C4).
	ctx, cancel := e.ctx()
	defer cancel()
	var refs []string
	for i := range pending {
		p := &pending[i]
		ref, perr := e.store.Put(ctx, e.project, p.content, p.typ, p.compressor)
		if perr != nil {
			return fallback("store_unavailable"), nil
		}
		p.storedRef = ref
		refs = append(refs, ref)
	}
	// Reescribir de atrás hacia delante conserva las posiciones de los bloques
	// que faltan por tratar. Dentro de cada bloque se cambian todas sus marcas:
	// JSON puede emitir más de una omisión para el mismo original.
	for i := len(pending) - 1; i >= 0; i-- {
		p := pending[i]
		if p.storedRef == p.ref {
			continue
		}
		result = replaceBlockRef(result, p)
	}

	name, typ := summarizeTypes(types)
	return ports.CompressionResult{
		Content:          result,
		RawTokens:        rawTokens,
		Tokens:           approxTokens(result),
		Compressed:       true,
		Compressor:       name,
		ContentType:      typ,
		StructuralTokens: structural.Tokens,
		Refs:             refs,
		Omissions:        omissions,
	}, nil
}

func replaceBlockRef(result string, p pendingOriginal) string {
	block := replaceMarkerRefs(result[p.start:p.end], p.ref, p.storedRef)
	return result[:p.start] + block + result[p.end:]
}

func replaceMarkerRefs(block, shortRef, longRef string) string {
	// Cada marcador textual contiene exactamente un `ref=` después de
	// MarkerTag. Se cambia solo la primera coincidencia posterior a cada marca;
	// otras referencias literales de la misma línea permanecen intactas.
	target := "ref=" + shortRef
	from := 0
	for {
		markerRel := strings.Index(block[from:], domain.MarkerTag)
		if markerRel < 0 {
			break
		}
		marker := from + markerRel
		afterMarker := marker + len(domain.MarkerTag)
		if marker > 0 && block[marker-1] == '"' && strings.HasPrefix(block[afterMarker:], `"`+":") {
			from = marker + len(domain.MarkerTag)
			continue
		}
		refRel := strings.Index(block[afterMarker:], target)
		if refRel < 0 {
			from = marker + len(domain.MarkerTag)
			continue
		}
		at := afterMarker + refRel
		replacement := "ref=" + longRef
		block = block[:at] + replacement + block[at+len(target):]
		from = at + len(replacement)
	}

	// Los marcadores JSON son objetos planos que contienen MarkerTag. Se
	// limita el reemplazo al objeto marcador para no alterar un campo `ref`
	// literal conservado dentro del mismo bloque.
	for _, needle := range []string{`"ref": "` + shortRef + `"`, `"ref":"` + shortRef + `"`} {
		from := 0
		for {
			rel := strings.Index(block[from:], needle)
			if rel < 0 {
				break
			}
			at := from + rel
			leftRel := strings.LastIndex(block[:at], "{")
			rightRel := strings.Index(block[at+len(needle):], "}")
			if leftRel >= 0 && rightRel >= 0 {
				right := at + len(needle) + rightRel + 1
				if strings.Contains(block[leftRel:right], `"`+domain.MarkerTag+`"`) {
					replacement := strings.Replace(needle, shortRef, longRef, 1)
					block = block[:at] + replacement + block[at+len(needle):]
					from = at + len(replacement)
					continue
				}
			}
			from = at + len(needle)
		}
	}
	return block
}

// compressBlock aplica el compresor del tipo con recuperación ante pánico y
// pasa la guarda de literalidad. Ante cualquier fallo devuelve el bloque
// literal (sin omisiones) y el error.
func (e *Engine) compressBlock(b block) (out string, omitted int, name string, err error) {
	name = string(b.typ)
	defer func() {
		if r := recover(); r != nil {
			out, omitted, err = b.content, 0, fmt.Errorf("compresor %s: pánico: %v", name, r)
		}
	}()
	th := domain.ThresholdsFor(e.aggressiveness(b.typ))
	if !th.Enabled {
		return b.content, 0, name, nil
	}
	ref := refOf(b.content)
	c, n, ok := compressorFor(b.typ)(b.content, b.lang, ref, th)
	if !ok {
		return b.content, 0, name, fmt.Errorf("compresor %s: estructura no reconocida", name)
	}
	if n == 0 {
		return b.content, 0, name, nil
	}
	if gerr := checkLiteral(b.content, c); gerr != nil {
		return b.content, 0, name, gerr
	}
	// La prosa y las tablas admiten además la limpieza estructural (espacios y
	// párrafos repetidos); el código y los logs no, porque el espacio importa.
	if (b.typ == domain.ContentProse || b.typ == domain.ContentTable) && !b.fenced {
		if s, serr := (compression.StructuralCompressor{}).Compress(c, ports.CompressionOptions{Level: ports.CompressionStructural}); serr == nil {
			c = s.Content
		}
	}
	return c, n, name, nil
}

func (e *Engine) aggressiveness(t domain.ContentType) domain.Aggressiveness {
	if e.tuning == nil {
		return domain.AggressivenessMax
	}
	ctx, cancel := e.ctx()
	defer cancel()
	a, err := e.tuning.Get(ctx, e.project, string(t))
	if err != nil {
		return domain.AggressivenessMax
	}
	return a
}

// record suma el resultado a las estadísticas. Best-effort: un fallo aquí nunca
// cambia lo que se entrega.
func (e *Engine) record(r ports.CompressionResult) {
	if e.stats == nil {
		return
	}
	ctx, cancel := e.ctx()
	defer cancel()
	_ = e.stats.Record(ctx, e.project, r)
}

func summarizeTypes(types map[domain.ContentType]bool) (string, string) {
	// Sin contar la prosa que rodea a los bloques con cercas.
	var main []domain.ContentType
	for t := range types {
		if t != domain.ContentProse {
			main = append(main, t)
		}
	}
	switch {
	case len(main) == 0:
		return string(domain.ContentProse), string(domain.ContentProse)
	case len(main) == 1 && !types[domain.ContentProse]:
		return string(main[0]), string(main[0])
	case len(main) == 1:
		return string(main[0]), string(domain.ContentMixed)
	default:
		return string(domain.ContentMixed), string(domain.ContentMixed)
	}
}

func refOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return domain.RefFromHash(hex.EncodeToString(sum[:]), domain.RefLen)
}

// charsPerToken es la misma heurística que compression.StructuralCompressor y
// tokens.ApproximateTokenCounter (~4 caracteres por token).
const charsPerToken = 4

func approxTokens(s string) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return (n + charsPerToken - 1) / charsPerToken
}
