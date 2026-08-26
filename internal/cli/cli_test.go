package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/franciscoperez/apuntes-cli/internal/core"
)

func TestDoctorReportsProblemsWhenWorkspaceIsEmpty(t *testing.T) {
	root := t.TempDir()
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	var out strings.Builder
	if err := doctor(a, &out); err == nil {
		t.Fatal("expected doctor to fail on an empty workspace")
	}
	s := out.String()
	if !strings.Contains(s, "[fallo]") {
		t.Fatalf("expected failures reported: %q", s)
	}
}

func TestDoctorPassesOnHealthyWorkspace(t *testing.T) {
	root := t.TempDir()
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := os.MkdirAll(filepath.Join(root, "materiales"), 0755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "materiales", "redes.md")
	if err := os.WriteFile(src, []byte("# Redes\\nCIDR y subnetting."), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Ingest(filepath.Join(root, "materiales")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "profile.json"), []byte(`{"institucion":"UTN","materias_activas":["redes"]}`), 0644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := doctor(a, &out); err != nil {
		t.Fatalf("doctor should pass on healthy workspace: %v (%s)", err, out.String())
	}
	if strings.Contains(out.String(), "[fallo]") {
		t.Fatalf("unexpected failure in healthy workspace: %q", out.String())
	}
}

func TestRunHelpListsDoctor(t *testing.T) {
	var out strings.Builder
	if err := Run([]string{"help"}, &out, os.Stdin); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "doctor") {
		t.Fatal("doctor missing from help output")
	}
}
