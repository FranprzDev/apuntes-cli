package summary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FranprzDev/apuntes-cli/internal/core"
	"github.com/FranprzDev/apuntes-cli/internal/session"
)

func TestSummaryFromClosedSession(t *testing.T) {
	root := t.TempDir()
	if _, err := session.Start(root, "Subnetting"); err != nil {
		t.Fatal(err)
	}
	if err := session.AddEntry(root, "¿Qué es una máscara CIDR?", "Notación /n que indica bits de red"); err != nil {
		t.Fatal(err)
	}
	if err := session.AddEntry(root, "¿Cómo calculo hosts disponibles?", ""); err != nil {
		t.Fatal(err)
	}
	closed, err := session.End(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	var out strings.Builder
	if err := Cmd(a, []string{"--sesion", closed}, &out); err != nil {
		t.Fatalf("resumen: %v", err)
	}
	mdPath := filepath.Join(root, "resumenes", filepath.Base(strings.TrimSuffix(closed, ".json")+".md"))
	b, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("resumen no generado: %v", err)
	}
	md := string(b)
	for _, want := range []string{"# Resumen de clase: Subnetting", "## Brief", "## Resumen gerencial", "¿Qué es una máscara CIDR?", "_Sin respuesta registrada._"} {
		if !strings.Contains(md, want) {
			t.Errorf("falta %q en el resumen", want)
		}
	}
}

func TestSummaryWithoutSessionsFails(t *testing.T) {
	root := t.TempDir()
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	var out strings.Builder
	if err := Cmd(a, nil, &out); err == nil {
		t.Fatal("expected error with no sessions")
	}
}

func TestSummaryPDF(t *testing.T) {
	root := t.TempDir()
	if _, err := session.Start(root, "Subnetting"); err != nil {
		t.Fatal(err)
	}
	if err := session.AddEntry(root, "¿Qué es CIDR?", "Notación /n"); err != nil {
		t.Fatal(err)
	}
	closed, err := session.End(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.LoadSession(closed)
	if err != nil {
		t.Fatal(err)
	}
	md := filepath.Join(root, "resumenes", "test.md")
	os.MkdirAll(filepath.Dir(md), 0755)
	os.WriteFile(md, []byte("x"), 0644)
	pdfPath, err := ToPDF(s, md)
	if err != nil {
		t.Fatalf("pdf: %v", err)
	}
	info, err := os.Stat(pdfPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("pdf inválido: %v", err)
	}
}

// TestSummaryRelativeSessionFromForeignCwd checks that a relative --sesion is
// resolved against the workspace root, not the process cwd.
func TestSummaryRelativeSessionFromForeignCwd(t *testing.T) {
	root := t.TempDir()
	if _, err := session.Start(root, "Subnetting"); err != nil {
		t.Fatal(err)
	}
	if err := session.AddEntry(root, "¿Qué es CIDR?", "Notación /n"); err != nil {
		t.Fatal(err)
	}
	closed, err := session.End(root)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, closed)
	if err != nil {
		t.Fatal(err)
	}
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	elsewhere := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	var out strings.Builder
	if err := Cmd(a, []string{"--sesion", rel}, &out); err != nil {
		t.Fatalf("resumen --sesion relativo: %v (%s)", err, out.String())
	}
	if _, err := os.Stat(filepath.Join(root, "resumenes", filepath.Base(strings.TrimSuffix(rel, ".json")+".md"))); err != nil {
		t.Fatalf("resumen no generado bajo el root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "resumenes")); !os.IsNotExist(err) {
		t.Fatalf("resumen must not resolve --sesion against the foreign cwd")
	}
}

// TestSummaryAbsoluteSessionStillWorks preserves absolute --sesion values.
func TestSummaryAbsoluteSessionStillWorks(t *testing.T) {
	root := t.TempDir()
	if _, err := session.Start(root, "Subnetting"); err != nil {
		t.Fatal(err)
	}
	closed, err := session.End(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	var out strings.Builder
	if err := Cmd(a, []string{"--sesion", closed}, &out); err != nil {
		t.Fatalf("resumen --sesion absoluto: %v", err)
	}
}
