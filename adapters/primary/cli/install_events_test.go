package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"mem/adapters/primary/console"
)

func TestInstallEventsFlagAndProtocol(t *testing.T) {
	o, err := parseInstallArgs([]string{".", "--events", "--agents", "opencode", "--scope", "project"})
	if err != nil || !o.events || !o.yes {
		t.Fatalf("events debe ser no interactivo: %+v %v", o, err)
	}
	var b bytes.Buffer
	writeInstallEvent(&b, installEvent{Version: 1, Type: "step", Name: "Memoria", Status: "warn", Detail: "línea\nnueva", Manual: "mem init"})
	if strings.Count(b.String(), "\n") != 1 {
		t.Fatal("NDJSON dividido por detalle multilínea")
	}
	var event installEvent
	if err := json.Unmarshal(b.Bytes(), &event); err != nil || event.Manual != "mem init" || event.Version != 1 {
		t.Fatalf("evento inválido: %s %v", b.String(), err)
	}
	step := eventFromStep(console.StepResult{Name: "Memoria", Status: console.StepWarn, Detail: "falló", Manual: "mem init"})
	if step.Status != "warn" || step.Type != "step" {
		t.Fatalf("aviso marcado éxito: %+v", step)
	}
}

func TestEventBridgeSeparatesHumanLogsAndRejectsMalformedEvents(t *testing.T) {
	var events, logs bytes.Buffer
	input := "Configurando agentes\n" + installEventPrefix + `{"contract_version":1,"type":"step","name":"Memoria","status":"ok"}` + "\n"
	warnings, err := bridgeInstallEvents(strings.NewReader(input), &events, &logs)
	if err != nil || warnings != 0 || !strings.Contains(logs.String(), "Configurando") || strings.Contains(events.String(), "Configurando") {
		t.Fatalf("mezcla canales: %s %s %v", events.String(), logs.String(), err)
	}
	if _, err := bridgeInstallEvents(strings.NewReader(installEventPrefix+"{}\n"), &events, &logs); err == nil {
		t.Fatal("evento mal formado aceptado")
	}
}

func TestInstallExplicitNoAgentsDoesNotFallBackToDetectedOrSaved(t *testing.T) {
	o, err := parseInstallArgs([]string{"--agents", "none", "--events"})
	if err != nil {
		t.Fatal(err)
	}
	sel := resolveInstallSelection(o, installSelection{agents: []string{"opencode"}, scope: "project"}, nil)
	if len(sel.agents) != 0 {
		t.Fatalf("selección vacía ignorada: %v", sel.agents)
	}
}
