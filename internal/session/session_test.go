package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionLifecycle(t *testing.T) {
	root := t.TempDir()
	p, err := Start(root, "Subnetting y CIDR")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !strings.Contains(p, "subnetting") || filepath.Dir(p) != SessionsDir(root) {
		t.Fatalf("ruta inesperada: %s", p)
	}
	if _, err := Start(root, "otra"); err == nil {
		t.Fatal("expected error when a session is already open")
	}
	if err := AddEntry(root, "¿Qué es una máscara CIDR?", "/24 = 24 bits de red"); err != nil {
		t.Fatalf("ask with answer: %v", err)
	}
	if err := AddEntry(root, "¿Cómo calculo subredes?", ""); err != nil {
		t.Fatalf("ask without answer: %v", err)
	}
	closed, err := End(root)
	if err != nil {
		t.Fatalf("end: %v", err)
	}
	if closed != p {
		t.Fatalf("closed path %s != started path %s", closed, p)
	}
	s, err := LoadSession(closed)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.Topic != "Subnetting y CIDR" || s.End == "" || len(s.Entries) != 2 {
		t.Fatalf("sesión inválida: %+v", s)
	}
	if s.Entries[0].Answer == "" || s.Entries[1].Answer != "" {
		t.Fatalf("respuestas mal guardadas: %+v", s.Entries)
	}
	if _, err := os.Stat(CurrentSessionPath(root)); !os.IsNotExist(err) {
		t.Fatal("current.json debería eliminarse al cerrar")
	}
}

func TestAddEntryWithoutSessionFails(t *testing.T) {
	root := t.TempDir()
	if err := AddEntry(root, "pregunta", ""); err == nil {
		t.Fatal("expected error with no open session")
	}
	if _, err := End(root); err == nil {
		t.Fatal("expected error ending with no session")
	}
}

func TestSlugify(t *testing.T) {
	got := Slugify("  Máscaras CIDR: subredes!! ")
	want := "mascaras-cidr-subredes"
	if got != want {
		t.Fatalf("slugify = %q, want %q", got, want)
	}
}
