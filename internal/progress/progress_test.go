package progress

import (
	"os"
	"strings"
	"testing"

	"github.com/franciscoperez/apuntes-cli/internal/core"
)

func TestProgressLifecycle(t *testing.T) {
	root := t.TempDir()
	if _, err := Set(root, "CIDR", "otro"); err == nil {
		t.Fatal("expected invalid status error")
	}
	if _, err := Set(root, "CIDR", "en_proceso"); err != nil {
		t.Fatal(err)
	}
	if _, err := Set(root, "CIDR", "en_proceso"); err != nil {
		t.Fatal(err)
	}
	p, err := Set(root, "CIDR", "dominado")
	if err != nil {
		t.Fatal(err)
	}
	if p.Repetitions != 3 || p.Status != "dominado" || p.LastReviewed == "" {
		t.Fatalf("progreso inesperado: %+v", p)
	}
	back, _ := Set(root, "CIDR", "pendiente")
	if back.Repetitions != 0 || back.Status != "pendiente" {
		t.Fatalf("volver a pendiente debería resetear: %+v", back)
	}
}

func TestProgressPersistenceAndOrder(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ topic, status string }{
		{"subnetting", "en_proceso"},
		{"máscaras", "dominado"},
		{"routing", "pendiente"},
	} {
		if _, err := Set(root, tc.topic, tc.status); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := History(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[0].Topic != "máscaras" || rows[0].Progress.Status != "dominado" {
		t.Fatalf("dominado debe ir primero: %+v", rows[0])
	}
	if rows[len(rows)-1].Topic != "routing" || rows[len(rows)-1].Progress.Status != "pendiente" {
		t.Fatalf("pendiente debe ir último: %+v", rows[len(rows)-1])
	}
	b, _ := os.ReadFile(Path(root))
	if !strings.Contains(string(b), "\"temas\"") {
		t.Fatal("progress.json mal formado")
	}
}

func TestProgressCmdOutput(t *testing.T) {
	root := t.TempDir()
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	var out strings.Builder
	if err := Cmd(a.Root, []string{"set", "CIDR", "dominado"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Cmd(a.Root, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "CIDR") || !strings.Contains(out.String(), "dominado") {
		t.Fatalf("salida inesperada: %q", out.String())
	}
	out.Reset()
	if err := Cmd(a.Root, []string{"invalido"}, &out); err == nil {
		t.Fatal("expected usage error")
	}
}
