package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSummaryWritesMDAndPDF(t *testing.T) {
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
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	mdPath, pdfPath, err := generateSummary(a, closed, true)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(mdPath) != filepath.Join(root, "resumenes") || !strings.HasSuffix(mdPath, ".md") {
		t.Fatalf("ruta md inesperada: %s", mdPath)
	}
	if info, err := os.Stat(mdPath); err != nil || info.Size() == 0 {
		t.Fatalf("md inválido: %v", err)
	}
	if !strings.HasSuffix(pdfPath, ".pdf") {
		t.Fatalf("ruta pdf inesperada: %s", pdfPath)
	}
	if info, err := os.Stat(pdfPath); err != nil || info.Size() == 0 {
		t.Fatalf("pdf inválido: %v", err)
	}
}

func TestGenerateSummaryWithoutSessionsFails(t *testing.T) {
	root := t.TempDir()
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, _, err := generateSummary(a, "", false); err == nil {
		t.Fatal("expected error with no sessions")
	}
}
