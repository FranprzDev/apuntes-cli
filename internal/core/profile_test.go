package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchFilteredByActiveSubjects(t *testing.T) {
	root := t.TempDir()
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ingestFixtureSubjects(t, a)

	writeProfile(t, root, `{"institucion":"UTN","materias_activas":["redes"]}`)
	rs, err := a.Search("practicar", 10, a.ActiveSubjects())
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].Subject != "redes" {
		t.Fatalf("esperaba solo redes: %+v", rs)
	}

	// Sin perfil (o sin materias activas) no filtra.
	os.Remove(filepath.Join(root, "data", "profile.json"))
	rs, err = a.Search("practicar", 10, a.ActiveSubjects())
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) < 1 {
		t.Fatal("sin filtro debería devolver resultados")
	}
}

func TestSuggestStepsRespectsActiveSubjects(t *testing.T) {
	root := t.TempDir()
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ingestFixtureSubjects(t, a)

	writeProfile(t, root, `{"institucion":"UTN","materias_activas":["quimica"]}`)
	steps, sources, err := a.SuggestSteps("")
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) == 0 {
		t.Fatal("esperaba fuentes de quimica")
	}
	for _, s := range sources {
		if s.Subject != "quimica" {
			t.Fatalf("fuente fuera del filtro: %+v", s)
		}
	}
	if len(steps) == 0 {
		t.Fatal("esperaba pasos derivados de quimica")
	}
}

func writeProfile(t *testing.T, root, jsonBody string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "data", "profile.json"), []byte(jsonBody), 0644); err != nil {
		t.Fatal(err)
	}
}

func ingestFixtureSubjects(t *testing.T, a *App) {
	t.Helper()
	for _, rel := range []string{"materiales/redes/subnetting.md", "materiales/quimica/tabla.md"} {
		full := filepath.Join(a.Root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		body := "# subnetting\nMáscaras CIDR y subredes para practicar."
		if filepath.Base(rel) == "tabla.md" {
			body = "# tabla periódica\nElementos químicos y sus propiedades."
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Ingest(filepath.Join(a.Root, "materiales")); err != nil {
		t.Fatal(err)
	}
}
