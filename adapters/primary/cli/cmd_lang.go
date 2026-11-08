package cli

import (
	"fmt"
	"os"
	"strings"

	"mem/adapters/primary/console"
)

var langUI console.UI

// CmdLang permite consultar o cambiar en caliente el idioma del proyecto.
func CmdLang(deps *Deps, args []string) {
	cat := getCatalog()
	supported := cat.SupportedCodes()

	if deps == nil || deps.ProjectRepo == nil || deps.SettingsRepo == nil {
		fail("repositorio no inicializado")
		return
	}
	root, err := deps.ProjectRepo.FindRoot()
	if err != nil {
		fail("no hay proyecto con memoria: %v", err)
		return
	}

	settings := deps.SettingsRepo.Read(root)
	currentLang := settings.Language
	if currentLang == "" {
		currentLang = "es"
	}

	if len(args) == 0 {
		env := console.DetectEnv()
		mode := console.DetectMode(env, false)
		ui := langUI
		if ui == nil {
			ui = console.New(mode)
		}
		if ui != nil && len(cat.AvailableLocales()) > 1 {
			opts := make([]console.Option, 0, len(cat.AvailableLocales()))
			for _, loc := range cat.AvailableLocales() {
				opts = append(opts, console.Option{
					Value:       loc.Code,
					Label:       loc.Name,
					Recommended: loc.Code == currentLang,
				})
			}
			prompt := cat.Get(currentLang, "install_prompt_lang")
			selected, err := ui.Select(prompt, opts)
			if err != nil {
				humanln(cat.Get(currentLang, "lang_canceled"))
				return
			}
			target := selected
			settings.Language = target
			if err := deps.SettingsRepo.Write(root, settings); err != nil {
				fail("guardar configuración: %v", err)
				return
			}
			locName := target
			for _, loc := range cat.AvailableLocales() {
				if loc.Code == target {
					locName = loc.Name
					break
				}
			}
			humanln(cat.Get(target, "lang_switched", map[string]string{
				"name": locName,
				"code": target,
			}))
			return
		}

		locName := currentLang
		for _, loc := range cat.AvailableLocales() {
			if loc.Code == currentLang {
				locName = loc.Name
				break
			}
		}
		humanln(cat.Get(currentLang, "lang_current", map[string]string{
			"code": currentLang,
			"name": locName,
		}))
		humanln(cat.Get(currentLang, "lang_available_list", map[string]string{
			"available": strings.Join(supported, ", "),
		}))
		humanln(cat.Get(currentLang, "lang_usage"))
		return
	}

	target := strings.ToLower(strings.TrimSpace(args[0]))
	if target == "--help" || target == "-h" {
		humanln(cat.Get(currentLang, "lang_usage"))
		return
	}

	if !cat.Supports(target) {
		fmt.Fprintf(os.Stderr, "Error: %s\n", cat.Get(currentLang, "lang_unsupported", map[string]string{
			"code":      target,
			"available": strings.Join(supported, ", "),
		}))
		exitProcess(2)
		return
	}

	settings.Language = target
	if err := deps.SettingsRepo.Write(root, settings); err != nil {
		fail("guardar configuración: %v", err)
		return
	}

	locName := target
	for _, loc := range cat.AvailableLocales() {
		if loc.Code == target {
			locName = loc.Name
			break
		}
	}

	humanln(cat.Get(target, "lang_switched", map[string]string{
		"name": locName,
		"code": target,
	}))
}
