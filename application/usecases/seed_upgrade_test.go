package usecases_test

import (
	"reflect"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// constitucionV1 es una plantilla "distribuida antes"; la semilla que la
// contiene intacta nunca la editó nadie.
const constitucionV1 = "CONSTITUCIÓN v1"

func semillaV2() []usecases.Seed {
	return []usecases.Seed{{
		TopicKey: domain.TopicConstitution, Type: domain.Architecture,
		Title: "Constitución del proyecto (spec-kit)", Content: "CONSTITUCIÓN v2",
		PreviousDefaultSHA256: []string{domain.ContentFingerprint(constitucionV1)},
	}}
}

func sembrarCon(t *testing.T, contenido string) (func(string) string, usecases.SeedUpgradeReport) {
	t.Helper()
	_, topics, seeder := repoDocs(t)
	doc, _ := domain.PinnedDocByAlias("constitution")
	if contenido != "" {
		if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, contenido); err != nil {
			t.Fatalf("sembrar: %v", err)
		}
	}
	rep, err := usecases.UpgradePristineSeeds(seeder, topics, "proj", semillaV2())
	if err != nil {
		t.Fatalf("actualizar: %v", err)
	}
	leer := func(key string) string {
		m, _ := topics.ByTopicKey("proj", key)
		if m == nil {
			return ""
		}
		return m.Content
	}
	return leer, rep
}

// Una semilla intacta de una versión anterior se lleva a la plantilla actual:
// es lo que hace que `mem update` entregue la constitución nueva sin pasos.
func TestUpgradePristineSeeds_ActualizaLaSemillaIntacta(t *testing.T) {
	leer, rep := sembrarCon(t, constitucionV1+"\n")
	if got := leer(domain.TopicConstitution); got != "CONSTITUCIÓN v2" {
		t.Errorf("la semilla intacta debía actualizarse; contenido %q", got)
	}
	if !reflect.DeepEqual(rep.Upgraded, []string{domain.TopicConstitution}) || len(rep.Customized) != 0 {
		t.Errorf("reporte = %+v; want Upgraded=[constitution]", rep)
	}
}

// La edición del equipo gana siempre: una semilla personalizada no se toca y
// se informa que hay una plantilla nueva disponible.
func TestUpgradePristineSeeds_NoTocaLaPersonalizada(t *testing.T) {
	leer, rep := sembrarCon(t, "CONSTITUCIÓN DEL EQUIPO")
	if got := leer(domain.TopicConstitution); got != "CONSTITUCIÓN DEL EQUIPO" {
		t.Errorf("la edición del equipo no debía tocarse; contenido %q", got)
	}
	if len(rep.Upgraded) != 0 || !reflect.DeepEqual(rep.Customized, []string{domain.TopicConstitution}) {
		t.Errorf("reporte = %+v; want Customized=[constitution]", rep)
	}
}

func TestUpgradePristineSeeds_ActualYAusenteNoHacenNada(t *testing.T) {
	leer, rep := sembrarCon(t, "CONSTITUCIÓN v2")
	if leer(domain.TopicConstitution) != "CONSTITUCIÓN v2" || len(rep.Upgraded)+len(rep.Customized) != 0 {
		t.Errorf("la semilla ya actual no debía tocarse ni reportarse: %+v", rep)
	}
	leer, rep = sembrarCon(t, "")
	if leer(domain.TopicConstitution) != "" || len(rep.Upgraded)+len(rep.Customized) != 0 {
		t.Errorf("una semilla ausente es trabajo de SeedDefaults, no de la actualización: %+v", rep)
	}
}

// Idempotencia: la segunda pasada no vuelve a escribir ni a reportar.
func TestUpgradePristineSeeds_EsIdempotente(t *testing.T) {
	_, topics, seeder := repoDocs(t)
	doc, _ := domain.PinnedDocByAlias("constitution")
	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, constitucionV1); err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	if _, err := usecases.UpgradePristineSeeds(seeder, topics, "proj", semillaV2()); err != nil {
		t.Fatalf("actualizar 1: %v", err)
	}
	rep, err := usecases.UpgradePristineSeeds(seeder, topics, "proj", semillaV2())
	if err != nil || len(rep.Upgraded)+len(rep.Customized) != 0 {
		t.Errorf("la segunda pasada no debía hacer nada: %+v, %v", rep, err)
	}
}
