package main

import "testing"

// HCD-034: el binario real reporta las tres lentes como MEDIUM y cierra igual.
func TestHCD034Lentes(t *testing.T) {
	h := newHCDBinary(t)
	r := h.run(true, "start", "--kind", "evolution", "--lane", "fast", "--title", "Lentes", "--why", "aviso", "--intent", "avisar sin tokens")
	doc := r.DocPath
	h.edit(doc, append(append([]string{}, hcdCommonFill...),
		"## Checklist\n", "## Checklist\n- [ ] 1. Tocar `save.go` y `falta/nuevo.go` → git grep -q previo -- save.go\n- [ ] 2. Medir la salida → observar a mano\n")...)
	h.run(true, "verify", "--task", "1,2", "--", "git grep -q previo -- save.go")
	r = h.run(true, "analyze")
	got := map[string]string{}
	for _, f := range r.Findings {
		got[f.Code+":"+f.Ref] = f.Severity
	}
	for key := range map[string]bool{"TASK-NO-FILE:2": true, "CHEAPEST-FILLER:Alternativa más barata": true, "PATH-MISSING:falta/nuevo.go": true} {
		if got[key] != "MEDIUM" {
			t.Errorf("%s = %q, want MEDIUM (all %v)", key, got[key], got)
		}
	}
	if got["PATH-MISSING:save.go"] != "" || got["TASK-NO-FILE:1"] != "" {
		t.Errorf("falsos positivos: %v", got)
	}
	if r = h.run(true, "close", "--quote", "apruebo"); r.State != "closed" {
		t.Fatalf("las lentes no bloquean el cierre: %+v", r)
	}
}
