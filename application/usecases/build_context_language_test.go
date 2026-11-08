package usecases

import (
	"strings"
	"testing"

	"mem/domain"
)

type dummyMemoryLister struct{}

func (d *dummyMemoryLister) List(project string, limit int) ([]domain.Memory, error) {
	return []domain.Memory{}, nil
}

type dummySessionQuerier struct{}

func (d *dummySessionQuerier) Active(project string) (*domain.Session, error) {
	return nil, nil
}
func (d *dummySessionQuerier) Recent(project string, limit int) ([]domain.Session, error) {
	return nil, nil
}

type dummyRelationLister struct{}

func (d *dummyRelationLister) List(project string, limit int) ([]domain.Relation, error) {
	return []domain.Relation{}, nil
}

type dummyLocaleCatalog struct {
	titles map[string]string
}

func (c *dummyLocaleCatalog) Supports(code string) bool {
	_, ok := c.titles[code]
	return ok
}

func (c *dummyLocaleCatalog) SupportedCodes() []string {
	return []string{"es", "en"}
}

func (c *dummyLocaleCatalog) Get(code, key string, _ ...map[string]string) string {
	if key == "context_title" {
		if v, ok := c.titles[code]; ok {
			return v
		}
		return c.titles["es"]
	}
	return key
}

func (c *dummyLocaleCatalog) GetTemplate(code, name string) (string, error) {
	return "", nil
}

func TestBuildContextLanguage(t *testing.T) {
	cat := &dummyLocaleCatalog{
		titles: map[string]string{
			"es": "# Memoria del Proyecto",
			"en": "# Project Memory",
		},
	}

	// 1. Builder con Language "es" o vacío
	bES := &Builder{
		Lister:    &dummyMemoryLister{},
		Session:   &dummySessionQuerier{},
		Relations: &dummyRelationLister{},
		Project:   "test",
		Language:  "es",
		Catalog:   cat,
	}
	outES, err := bES.Build()
	if err != nil {
		t.Fatalf("Build() en español falló: %v", err)
	}
	if !strings.Contains(outES, "# Memoria del Proyecto") {
		t.Errorf("se esperaba '# Memoria del Proyecto', se obtuvo: %s", outES)
	}

	// 2. Builder con Language "en"
	bEN := bES.WithLanguage("en")
	outEN, err := bEN.Build()
	if err != nil {
		t.Fatalf("Build() en inglés falló: %v", err)
	}
	if !strings.Contains(outEN, "# Project Memory") {
		t.Errorf("se esperaba '# Project Memory', se obtuvo: %s", outEN)
	}
}
