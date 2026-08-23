package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"mem/application/usecases"
	"mem/domain"
)

// CmdDocs administra los documentos fijados del proyecto: las reglas de trabajo
// y la constitución que gomemory siembra (feature 021).
//
// Existe por una razón que no es de comodidad: sin una vía cómoda de reemplazo,
// sembrar reglas y constitución convertiría a la herramienta en AUTORA de las
// normas del equipo. Las plantillas que se envían son un punto de partida, no
// doctrina. Esto es lo que mantiene la memoria como contenedor neutral.
//
// Toda la maquinaria recorre domain.PinnedDocs: añadir un documento nuevo al
// catálogo no debe exigir tocar este archivo.
func CmdDocs(deps *Deps, args []string) {
	if err := runDocs(deps, args, os.Stdout, os.Stderr); err != nil {
		fail("%v", err)
	}
}

// runDocs es el núcleo testeable: devuelve error en vez de terminar el proceso,
// y escribe por los writers que reciba. La separación permite probar los
// contratos de stdout/stderr sin capturar descriptores globales.
func runDocs(deps *Deps, args []string, stdout, stderr io.Writer) error {
	sub := "list"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}

	switch sub {
	case "list", "ls":
		return docsList(deps, stdout)
	case "show", "export":
		return docsExport(deps, args, stdout, stderr)
	case "import":
		return docsImport(deps, args, stderr)
	case "reset":
		return docsReset(deps, args, stderr)
	default:
		return fmt.Errorf("subcomando desconocido %q\nUso: mem docs [list|show|export|import|reset]", sub)
	}
}


// flagsConValor son los flags de `mem docs` que consumen el argumento siguiente.
var flagsConValor = map[string]bool{"-o": true, "--o": true, "-topic": true, "--topic": true}

// reordenarFlags mueve los flags al principio para que flag.FlagSet los vea.
//
// flag.FlagSet de stdlib deja de parsear en el PRIMER argumento posicional, así
// que `docs export rules -o archivo` dejaba -o sin parsear: el contenido salía
// por stdout y el archivo quedaba vacío, en silencio. `mem uninstall` ya
// documenta la misma trampa y la resuelve a mano.
//
// El orden natural para quien teclea es `export rules -o archivo`, no
// `export -o archivo rules`; la herramienta se adapta, no al revés.
func reordenarFlags(args []string) []string {
	var flags, posicionales []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			posicionales = append(posicionales, a)
			continue
		}
		flags = append(flags, a)
		// Un flag con valor separado (-o archivo) arrastra el siguiente token,
		// salvo que venga pegado (-o=archivo) o sea el último.
		if flagsConValor[a] && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, posicionales...)
}

// docsList muestra el estado DERIVADO de cada documento del catálogo. El estado
// se calcula comparando con la plantilla embebida en vez de almacenarse: una
// columna nueva podría quedar desincronizada del contenido real.
func docsList(deps *Deps, stdout io.Writer) error {
	_, topics, ok := seedDeps(deps)
	if !ok {
		return fmt.Errorf("no hay acceso a la memoria del proyecto")
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "ALIAS\tDOCUMENTO\tESTADO\tLÍNEAS\tÚLTIMA MODIFICACIÓN\n")
	fmt.Fprintf(w, "-----\t---------\t------\t------\t-------------------\n")
	for _, d := range domain.PinnedDocs {
		st := usecases.PinnedDocState(topics, deps.Project, d.TopicKey, embeddedTemplate(d.Template))
		fecha := st.UpdatedAt
		if len(fecha) > 16 {
			fecha = fecha[:16]
		}
		if fecha == "" {
			fecha = "—"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", d.Alias, d.Label, st.State, st.Lines, fecha)
	}
	return w.Flush()
}

// docsExport escribe el contenido VIGENTE —el guardado, no la plantilla— en
// stdout o en un archivo.
//
// Contrato de stdout: sin -o, stdout lleva SOLO el documento. Cualquier aviso va
// a stderr, para que `mem docs show rules > reglas.md` produzca un archivo
// limpio y `mem docs show rules | diff - reglas.md` funcione.
func docsExport(deps *Deps, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("docs export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "archivo de salida (default: stdout)")
	todos := fs.Bool("all", false, "exportar todo el catálogo al directorio indicado con -o")
	if err := fs.Parse(reordenarFlags(args)); err != nil {
		return err
	}

	_, topics, ok := seedDeps(deps)
	if !ok {
		return fmt.Errorf("no hay acceso a la memoria del proyecto")
	}

	if *todos {
		if *out == "" {
			return fmt.Errorf("--all necesita un directorio de destino: usa -o <dir>")
		}
		if err := os.MkdirAll(*out, 0o755); err != nil {
			return fmt.Errorf("crear %s: %w", *out, err)
		}
		for _, d := range domain.PinnedDocs {
			res, err := usecases.ResolvePinnedDoc(topics, deps.Project, d.TopicKey, embeddedTemplate(d.Template))
			if err != nil {
				return err
			}
			destino := filepath.Join(*out, d.Alias+".md")
			if err := os.WriteFile(destino, []byte(res.Content), 0o644); err != nil {
				return fmt.Errorf("escribir %s: %w", destino, err)
			}
			fmt.Fprintf(stderr, "✅ %s → %s\n", d.Alias, destino)
		}
		return nil
	}

	doc, err := docFromArgs(fs.Args())
	if err != nil {
		return err
	}

	res, err := usecases.ResolvePinnedDoc(topics, deps.Project, doc.TopicKey, embeddedTemplate(doc.Template))
	if err != nil {
		return err
	}
	if !res.FromMemory {
		fmt.Fprintf(stderr, "⚠️  No hay %s en la memoria de este proyecto; se muestra el contenido por defecto.\n", doc.Alias)
	}

	if *out == "" {
		_, err := io.WriteString(stdout, res.Content)
		return err
	}
	if err := os.WriteFile(*out, []byte(res.Content), 0o644); err != nil {
		return fmt.Errorf("escribir %s: %w", *out, err)
	}
	fmt.Fprintf(stderr, "✅ %s → %s (%d líneas)\n", doc.Alias, *out, contarLineasCLI(res.Content))
	return nil
}

// docsImport reemplaza —o crea— el contenido de un documento fijado desde un
// archivo. Rechaza contenido vacío dejando el anterior INTACTO: un import
// fallido que destruya lo que había es el peor modo de fallo de esta capacidad.
func docsImport(deps *Deps, args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("docs import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// --topic permite apuntar a CUALQUIER clave, dentro o fuera del catálogo:
	// el catálogo es una comodidad, no un límite.
	topic := fs.String("topic", "", "clave de tópico arbitraria, en vez de un alias del catálogo")
	if err := fs.Parse(reordenarFlags(args)); err != nil {
		return err
	}
	rest := fs.Args()

	var topicKey, title string
	var memType domain.MemoryType
	var ruta string

	if *topic != "" {
		if len(rest) < 1 {
			return fmt.Errorf("uso: mem docs import --topic <clave> <archivo>")
		}
		topicKey, ruta = *topic, rest[0]
		memType, title = domain.Learning, *topic
		if d, ok := domain.PinnedDocByTopicKey(*topic); ok {
			memType, title = d.Type, d.Title
		}
	} else {
		if len(rest) < 2 {
			return fmt.Errorf("uso: mem docs import <alias> <archivo>")
		}
		doc, err := docFromArgs(rest[:1])
		if err != nil {
			return err
		}
		topicKey, memType, title, ruta = doc.TopicKey, doc.Type, doc.Title, rest[1]
	}

	datos, err := os.ReadFile(ruta)
	if err != nil {
		return fmt.Errorf("leer %s: %w", ruta, err)
	}

	seeder, topics, ok := seedDeps(deps)
	if !ok {
		return fmt.Errorf("no hay acceso a la memoria del proyecto")
	}

	res, err := usecases.ImportPinnedDoc(seeder, topics, deps.Project, topicKey, memType, title, string(datos))
	if err != nil {
		return fmt.Errorf("importar %s: %w", ruta, err)
	}

	nombre := topicKey
	if d, ok := domain.PinnedDocByTopicKey(topicKey); ok {
		nombre = d.Alias
	}
	switch {
	case res.Unchanged:
		fmt.Fprintf(stderr, "ℹ️  %s sin cambios\n", nombre)
	case res.Created:
		fmt.Fprintf(stderr, "✅ %s creado desde %s (%d líneas)\n", nombre, ruta, res.Lines)
	default:
		fmt.Fprintf(stderr, "✅ %s actualizado desde %s (%d líneas)\n", nombre, ruta, res.Lines)
	}
	return nil
}

// docsReset devuelve un documento a su contenido por defecto.
func docsReset(deps *Deps, args []string, stderr io.Writer) error {
	doc, err := docFromArgs(args)
	if err != nil {
		return err
	}
	seeder, topics, ok := seedDeps(deps)
	if !ok {
		return fmt.Errorf("no hay acceso a la memoria del proyecto")
	}

	res, err := usecases.ResetPinnedDoc(seeder, topics, deps.Project,
		doc.TopicKey, doc.Type, doc.Title, embeddedTemplate(doc.Template))
	if err != nil {
		return fmt.Errorf("restaurar %s: %w", doc.Alias, err)
	}
	if res.Unchanged {
		fmt.Fprintf(stderr, "ℹ️  %s ya estaba en su contenido por defecto\n", doc.Alias)
		return nil
	}
	fmt.Fprintf(stderr, "✅ %s restaurado al contenido por defecto (%d líneas)\n", doc.Alias, res.Lines)
	return nil
}

// docFromArgs resuelve el alias del catálogo, listando los válidos si falla:
// un mensaje de error que no dice qué se podía teclear obliga a leer el código.
func docFromArgs(args []string) (domain.PinnedDoc, error) {
	if len(args) == 0 {
		return domain.PinnedDoc{}, fmt.Errorf("indica un documento. Disponibles: %s",
			strings.Join(domain.PinnedDocAliases(), ", "))
	}
	doc, ok := domain.PinnedDocByAlias(args[0])
	if !ok {
		return domain.PinnedDoc{}, fmt.Errorf("documento desconocido %q. Disponibles: %s",
			args[0], strings.Join(domain.PinnedDocAliases(), ", "))
	}
	return doc, nil
}

func contarLineasCLI(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
