package summary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/franciscoperez/apuntes-cli/internal/core"
	"github.com/franciscoperez/apuntes-cli/internal/session"
)

func TestGenerateSummaryWritesMDAndPDF(t *testing.T) {
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
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	mdPath, pdfPath, err := Generate(a, closed, true)
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
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, _, err := Generate(a, "", false); err == nil {
		t.Fatal("expected error with no sessions")
	}
}
