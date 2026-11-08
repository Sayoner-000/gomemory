package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	domI18n "mem/domain/i18n"
)

// Catalog orquesta el descubrimiento de idiomas y la resolución de cadenas y plantillas.
type Catalog struct {
	mu           sync.RWMutex
	fsys         fs.FS
	basePath     string
	locales      map[string]domI18n.Locale
	messages     map[string]map[string]string
	templates    map[string]map[string]string
	defaultCode  string
	orderedCodes []string
}

// NewCatalog crea y carga un catálogo a partir del sistema de archivos dado.
func NewCatalog(fsys fs.FS, basePath string) (*Catalog, error) {
	c := &Catalog{
		fsys:        fsys,
		basePath:    basePath,
		locales:     make(map[string]domI18n.Locale),
		messages:    make(map[string]map[string]string),
		templates:   make(map[string]map[string]string),
		defaultCode: "es",
	}
	if fsys != nil {
		if err := c.reload(); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Catalog) reload() error {
	entries, err := fs.ReadDir(c.fsys, c.basePath)
	if err != nil {
		return fmt.Errorf("leer directorio de idiomas %s: %w", c.basePath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		code := entry.Name()
		localeDir := path.Join(c.basePath, code)

		// Leer meta.json
		metaBytes, err := fs.ReadFile(c.fsys, path.Join(localeDir, "meta.json"))
		if err != nil {
			continue // No es un directorio de idioma válido
		}
		var loc domI18n.Locale
		if err := json.Unmarshal(metaBytes, &loc); err != nil {
			return fmt.Errorf("parsear meta.json en %s: %w", localeDir, err)
		}
		if loc.Code == "" {
			loc.Code = code
		}
		c.locales[loc.Code] = loc
		if loc.Default {
			c.defaultCode = loc.Code
		}

		// Leer messages.json
		msgBytes, err := fs.ReadFile(c.fsys, path.Join(localeDir, "messages.json"))
		if err == nil {
			var msgs map[string]string
			if err := json.Unmarshal(msgBytes, &msgs); err == nil {
				c.messages[loc.Code] = msgs
			}
		}

		// Escanear archivos de plantilla *.tmpl
		dirEntries, err := fs.ReadDir(c.fsys, localeDir)
		if err == nil {
			c.templates[loc.Code] = make(map[string]string)
			for _, de := range dirEntries {
				if !de.IsDir() && strings.HasSuffix(de.Name(), ".tmpl") {
					content, err := fs.ReadFile(c.fsys, path.Join(localeDir, de.Name()))
					if err == nil {
						tmplName := strings.TrimSuffix(de.Name(), ".tmpl")
						c.templates[loc.Code][tmplName] = string(content)
						cleanName := strings.TrimSuffix(tmplName, ".md")
						c.templates[loc.Code][cleanName] = string(content)
					}
				}
			}
		}
	}

	// Ordenar códigos
	codes := make([]string, 0, len(c.locales))
	for k := range c.locales {
		codes = append(codes, k)
	}
	sort.Slice(codes, func(i, j int) bool {
		locI := c.locales[codes[i]]
		locJ := c.locales[codes[j]]
		if locI.Order != locJ.Order {
			return locI.Order < locJ.Order
		}
		return codes[i] < codes[j]
	})
	c.orderedCodes = codes
	return nil
}

// AvailableLocales devuelve la lista ordenada de idiomas disponibles.
func (c *Catalog) AvailableLocales() []domI18n.Locale {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make([]domI18n.Locale, 0, len(c.orderedCodes))
	for _, code := range c.orderedCodes {
		res = append(res, c.locales[code])
	}
	return res
}

// SupportedCodes devuelve la lista de códigos de idioma soportados.
func (c *Catalog) SupportedCodes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make([]string, len(c.orderedCodes))
	copy(res, c.orderedCodes)
	return res
}

// DefaultLocale devuelve el idioma marcado como default (o "es").
func (c *Catalog) DefaultLocale() domI18n.Locale {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if loc, ok := c.locales[c.defaultCode]; ok {
		return loc
	}
	return domI18n.Locale{Code: "es", Name: "Español neutral", Default: true}
}

// Supports indica si un código de idioma está soportado.
func (c *Catalog) Supports(code string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.locales[code]
	return ok
}

// Get resuelve una clave en el idioma solicitado, con fallback al idioma predeterminado.
func (c *Catalog) Get(code, key string, params ...map[string]string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var msg string
	found := false

	// 1. Probar en el idioma solicitado
	if msgs, ok := c.messages[code]; ok {
		if val, exists := msgs[key]; exists {
			msg = val
			found = true
		}
	}

	// 2. Fallback al idioma por defecto si no se encontró
	if !found && code != c.defaultCode {
		if msgs, ok := c.messages[c.defaultCode]; ok {
			if val, exists := msgs[key]; exists {
				msg = val
				found = true
			}
		}
	}

	// 3. Si aún no existe, devolver la clave
	if !found {
		msg = key
	}

	var p map[string]string
	if len(params) > 0 {
		p = params[0]
	}
	return domI18n.Interpolate(msg, p)
}

// GetTemplate devuelve el contenido de una plantilla para el idioma dado, con fallback.
func (c *Catalog) GetTemplate(code, name string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if tmpls, ok := c.templates[code]; ok {
		if content, exists := tmpls[name]; exists {
			return content, nil
		}
	}

	if code != c.defaultCode {
		if tmpls, ok := c.templates[c.defaultCode]; ok {
			if content, exists := tmpls[name]; exists {
				return content, nil
			}
		}
	}

	return "", fmt.Errorf("plantilla %q no encontrada para idioma %q", name, code)
}
