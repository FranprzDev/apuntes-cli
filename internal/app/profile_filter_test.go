package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		if strings.Contains(rel, "quimica") {
			body = "# tabla periódica\nElementos químicos y sus propiedades."
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.ingest(filepath.Join(a.Root, "materiales")); err != nil {
		t.Fatal(err)
	}
}

func TestSearchFilteredByActiveSubjects(t *testing.T) {
	root := t.TempDir()
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ingestFixtureSubjects(t, a)

	writeProfile(t, root, `{"institucion":"UTN","materias_activas":["redes"]}`)
	rs, err := a.search("practicar", 10, a.activeSubjects())
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].Subject != "redes" {
		t.Fatalf("esperaba solo redes: %+v", rs)
	}

	// Sin perfil (o sin materias activas) no filtra.
	os.Remove(filepath.Join(root, "data", "profile.json"))
	rs, err = a.search("practicar", 10, a.activeSubjects())
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
	steps, sources, err := a.suggestSteps("")
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

func TestMCPSearchUsesProfileFilterEndToEnd(t *testing.T) {
	root := t.TempDir()
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ingestFixtureSubjects(t, a)
	writeProfile(t, root, `{"institucion":"UTN","materias_activas":["quimica"]}`)

	respRaw := callMCP(t, a, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"buscar_material","arguments":{"query":"elementos"}}}`)
	resp := fmt.Sprintf("%v", respRaw["result"])
	if !strings.Contains(resp, "quimica") || !strings.Contains(resp, "tabla") {
		t.Fatalf("esperaba resultado de quimica: %s", resp)
	}
	if strings.Contains(resp, "subnetting") {
		t.Fatalf("el filtro por perfil no se aplicó: %s", resp)
	}
}
