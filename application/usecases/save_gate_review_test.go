package usecases_test

import (
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// S-001 (ACR 031): la misma topic_key ya resuelve la identidad. El gate no
// debe depender de que el upsert la haya fusionado para callar.
func TestCheckNearDuplicates_MismaTopicKeyNoAvisa(t *testing.T) {
	saved := domain.Memory{Type: domain.Decision, Title: "caché redis compartida", Content: "latencia caché redis compartida", TopicKey: "cache-redis"}
	snapshot := []domain.Memory{
		{ID: 2, Type: domain.Decision, Title: saved.Title, Content: saved.Content, TopicKey: "cache-redis"},
		{ID: 3, Type: domain.Decision, Title: saved.Title, Content: saved.Content, TopicKey: "otra-clave"},
	}
	got := usecases.CheckNearDuplicates(snapshot, saved, 99)
	if len(got.Candidates) != 1 || got.Candidates[0].ID != 3 {
		t.Fatalf("solo debe avisar del candidato con otra clave: %#v", got.Candidates)
	}
}

// S-003 (segunda revisión 031): la instantánea lleva la anotación de impacto
// que insertMemory añade a las memorias de hotspots; la memoria nueva todavía
// no. En cuerpos cortos esa nota diluía el Jaccard por debajo del umbral.
func TestCheckNearDuplicates_IgnoraLaAnotacionDeImpacto(t *testing.T) {
	saved := domain.Memory{Type: domain.Decision, Title: "alfa", Content: "redis"}
	candidate := domain.Memory{ID: 4, Type: domain.Decision, Title: "beta", Content: "redis" + domain.ImpactAnnotation("BuildContext", 42)}
	got := usecases.CheckNearDuplicates([]domain.Memory{candidate}, saved, 99)
	if len(got.Candidates) != 1 || got.Candidates[0].ID != 4 {
		t.Fatalf("la anotación de impacto no debe ocultar el duplicado: %#v", got)
	}
}

// S-002 (3ª revisión 031): la nota se descuenta en los dos lados. Si la memoria
// nueva la trae pegada, la asimetría no debe reaparecer al revés.
func TestCheckNearDuplicates_DescuentaLaNotaEnAmbosLados(t *testing.T) {
	saved := domain.Memory{Type: domain.Decision, Title: "alfa", Content: "redis" + domain.ImpactAnnotation("BuildContext", 42)}
	candidate := domain.Memory{ID: 6, Type: domain.Decision, Title: "beta", Content: "redis"}
	got := usecases.CheckNearDuplicates([]domain.Memory{candidate}, saved, 99)
	if len(got.Candidates) != 1 || got.Candidates[0].ID != 6 {
		t.Fatalf("una nota pegada en la memoria nueva no debe ocultar el duplicado: %#v", got)
	}
}

// S-003 (ACR 031): la instantánea guarda el texto redactado, así que la
// memoria nueva debe compararse también redactada.
func TestCheckNearDuplicates_ComparaElTextoRedactado(t *testing.T) {
	secret := "<private>" + strings.Repeat("secreto único irrepetible número ", 1) +
		"uno dos tres cuatro cinco seis siete ocho nueve diez once doce trece catorce quince dieciséis diecisiete dieciocho</private>"
	raw := "despliegue del servicio de pagos con rotación de credenciales " + secret
	saved := domain.Memory{Type: domain.Bugfix, Title: "registro alfa", Content: raw}
	candidate := domain.Memory{ID: 5, Type: domain.Bugfix, Title: "registro beta", Content: domain.RedactSecrets(domain.RedactPrivate(raw))}

	got := usecases.CheckNearDuplicates([]domain.Memory{candidate}, saved, 99)
	if len(got.Candidates) != 1 || got.Candidates[0].ID != 5 {
		t.Fatalf("con el mismo texto una vez redactado debe avisar: %#v", got)
	}
}
