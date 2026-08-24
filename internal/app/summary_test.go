package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSummaryFromClosedSession(t *testing.T) {
	root := t.TempDir()
	if _, err := startSession(root, "Subnetting"); err != nil {
		t.Fatal(err)
	}
	if err := addEntry(root, "¿Qué es una máscara CIDR?", "Notación /n que indica bits de red"); err != nil {
		t.Fatal(err)
	}
	if err := addEntry(root, "¿Cómo calculo hosts disponibles?", ""); err != nil {
		t.Fatal(err)
	}
	closed, err := endSession(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	var out strings.Builder
	if err := summaryCmd(a, []string{"--sesion", closed}, &out); err != nil {
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
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	var out strings.Builder
	if err := summaryCmd(a, nil, &out); err == nil {
		t.Fatal("expected error with no sessions")
	}
}

func TestSummaryPDF(t *testing.T) {
	root := t.TempDir()
	if _, err := startSession(root, "Subnetting"); err != nil {
		t.Fatal(err)
	}
	if err := addEntry(root, "¿Qué es CIDR?", "Notación /n"); err != nil {
		t.Fatal(err)
	}
	closed, err := endSession(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := loadSession(closed)
	if err != nil {
		t.Fatal(err)
	}
	md := filepath.Join(root, "resumenes", "test.md")
	os.MkdirAll(filepath.Dir(md), 0755)
	os.WriteFile(md, []byte("x"), 0644)
	pdfPath, err := summaryToPDF(s, md)
	if err != nil {
		t.Fatalf("pdf: %v", err)
	}
	info, err := os.Stat(pdfPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("pdf inválido: %v", err)
	}
}
