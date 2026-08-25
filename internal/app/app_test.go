package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFTSQuerySanitizesOperators(t *testing.T) {
	got := ftsQuery(`CIDR (subredes) + práctica`)
	if got != `"CIDR" AND "subredes" AND "práctica"` {
		t.Fatalf("unexpected query: %q", got)
	}
}

func TestIngestAndSearchKeepsRelativeSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "materiales", "redes", "clase.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("Máscaras y subnetting con CIDR"), 0644); err != nil {
		t.Fatal(err)
	}
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if n, err := a.ingest(filepath.Join(root, "materiales")); err != nil || n != 1 {
		t.Fatalf("ingest: n=%d err=%v", n, err)
	}
	results, err := a.search("subnetting", 5, []string{"redes"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Path != "materiales/redes/clase.md" || results[0].Subject != "redes" {
		t.Fatalf("bad result: %+v", results)
	}
}

func TestSafePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if safePath(root, filepath.Join(root, "..", "secret.txt")) {
		t.Fatal("traversal accepted")
	}
}

func TestExtractPDFDoesNotFailOnCorruptInput(t *testing.T) {
	if _, ok := extractPDF([]byte("not a pdf")); ok {
		t.Fatal("expected extraction failure on corrupt input")
	}
}
