package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FranprzDev/apuntes-cli/internal/core"
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

func TestParseRoot(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		args     []string
		wantRoot string
		wantRest []string
		wantErr  bool
	}{
		{name: "absent uses cwd", args: []string{"mcp"}, wantRoot: wd, wantRest: []string{"mcp"}},
		{name: "separate value", args: []string{"--root", "/tmp/repo", "mcp"}, wantRoot: "/tmp/repo", wantRest: []string{"mcp"}},
		{name: "equals value", args: []string{"--root=/tmp/repo", "search", "x"}, wantRoot: "/tmp/repo", wantRest: []string{"search", "x"}},
		{name: "empty args", args: nil, wantRoot: wd, wantRest: nil},
		{name: "missing value", args: []string{"--root"}, wantErr: true},
		{name: "empty value", args: []string{"--root", "", "mcp"}, wantErr: true},
		{name: "flag as value", args: []string{"--root", "--http", "mcp"}, wantErr: true},
		{name: "flag as value help", args: []string{"--root", "--help", "mcp"}, wantErr: true},
		{name: "empty equals", args: []string{"--root="}, wantErr: true},
		{name: "unknown flag", args: []string{"--bogus", "mcp"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, rest, err := parseRoot(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got root=%q rest=%v", root, rest)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if root != tc.wantRoot {
				t.Fatalf("root = %q, want %q", root, tc.wantRoot)
			}
			if strings.Join(rest, "\x00") != strings.Join(tc.wantRest, "\x00") {
				t.Fatalf("rest = %v, want %v", rest, tc.wantRest)
			}
		})
	}
}

func TestRunRejectsInvalidRoot(t *testing.T) {
	cases := [][]string{
		{"--root"},
		{"--root", "", "init"},
		{"--root=", "init"},
		{"--root", "--http", "init"},
		{"--unknown", "init"},
	}
	for _, args := range cases {
		var out strings.Builder
		if err := Run(args, &out, strings.NewReader("")); err == nil {
			t.Fatalf("%v: expected an error for invalid --root usage", args)
		}
	}
}

// TestRunRootFromForeignCwd drives init/ingest/search against a fixture root
// while the process working directory stays untouched, emulating an MCP
// client that launches the binary from somewhere else.
func TestRunRootFromForeignCwd(t *testing.T) {
	root := t.TempDir()
	elsewhere := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if wd == root || wd == elsewhere {
		t.Fatalf("unexpected shared cwd %q", wd)
	}

	run := func(args ...string) string {
		t.Helper()
		var out strings.Builder
		full := append([]string{"--root", root}, args...)
		if err := Run(full, &out, strings.NewReader("")); err != nil {
			t.Fatalf("%v: %v (%s)", args, err, out.String())
		}
		return out.String()
	}

	if out := run("init"); !strings.Contains(out, "inicializado") {
		t.Fatalf("init output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "materiales")); err != nil {
		t.Fatalf("init should create materiales/ in the root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "materiales")); !os.IsNotExist(err) {
		t.Fatalf("init must not touch the foreign cwd")
	}

	src := filepath.Join(root, "materiales", "redes", "subnetting.md")
	if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("# Subnetting\nCIDR, mascaras y subredes."), 0644); err != nil {
		t.Fatal(err)
	}
	if out := run("ingest"); !strings.Contains(out, "1 fuentes indexadas") {
		t.Fatalf("ingest output: %q", out)
	}

	out := run("search", "subnetting")
	if !strings.Contains(out, "subnetting") {
		t.Fatalf("search should find the fixture source: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "index.db")); err != nil {
		t.Fatalf("index should live in the root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "data")); !os.IsNotExist(err) {
		t.Fatalf("search must not write to the foreign cwd")
	}

	if err := Run([]string{"--root", root, "doctor"}, &strings.Builder{}, strings.NewReader("")); err != nil {
		// doctor fails without a profile; only assert the index was seen.
		t.Logf("doctor after ingest: %v", err)
	}
}

// TestRunIngestRelativePathFromForeignCwd checks that a relative --path is
// resolved against the selected root, not the process cwd, so
// `--root <repo> --path materiales/<dir>` works when launched elsewhere.
func TestRunIngestRelativePathFromForeignCwd(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(repo, "materiales", "redes")
	if info, err := os.Stat(fixture); err != nil || !info.IsDir() {
		t.Skipf("fixture %s no disponible: %v", fixture, err)
	}
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
	args := []string{"--root", repo, "ingest", "--path", filepath.Join("materiales", "redes")}
	if err := Run(args, &out, strings.NewReader("")); err != nil {
		t.Fatalf("ingest --path relativo: %v (%s)", err, out.String())
	}
	if !strings.Contains(out.String(), "fuentes indexadas") {
		t.Fatalf("ingest output: %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "materiales")); !os.IsNotExist(err) {
		t.Fatalf("ingest must not resolve --path against the foreign cwd")
	}
}

// TestRunIngestAbsolutePathStillWorks keeps backward compatibility for
// absolute --path values while --root points elsewhere.
func TestRunIngestAbsolutePathStillWorks(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "materiales", "redes", "subnetting.md")
	if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("# Subnetting\nCIDR."), 0644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	args := []string{"--root", root, "ingest", "--path", filepath.Join(root, "materiales", "redes")}
	if err := Run(args, &out, strings.NewReader("")); err != nil {
		t.Fatalf("ingest --path absoluto: %v (%s)", err, out.String())
	}
	if !strings.Contains(out.String(), "1 fuentes indexadas") {
		t.Fatalf("ingest output: %q", out.String())
	}
}
