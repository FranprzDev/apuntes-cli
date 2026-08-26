package core

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeEmbedder maps a few domain words to fixed dimensions so tests are
// deterministic and need no external service.
type fakeEmbedder struct{}

var fakeDims = map[string]int{"gpu": 0, "video": 1, "placa": 2, "redes": 3, "subnetting": 4, "mascaras": 5}

func (fakeEmbedder) Embed(texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, t := range texts {
		v := make([]float64, len(fakeDims))
		for w, d := range fakeDims {
			if containsFoldWord(t, w) {
				v[d] = 1
			}
		}
		out[i] = v
	}
	return out, nil
}

func containsFoldWord(s, w string) bool {
	s = lower(s)
	w = lower(w)
	for i := 0; i+len(w) <= len(s); i++ {
		if s[i:i+len(w)] == w {
			return true
		}
	}
	return false
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

func TestSemanticSearchFindsSynonymsFTSMisses(t *testing.T) {
	root := t.TempDir()
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.embedCache = fakeEmbedder{}
	writeDoc(t, root, "materiales/hardware/gpu.md", "La GPU NVIDIA es la placa de video del equipo.")
	writeDoc(t, root, "materiales/redes/subnetting.md", "Subnetting y máscaras de redes.")
	if _, err := a.Ingest(filepath.Join(root, "materiales")); err != nil {
		t.Fatal(err)
	}

	// FTS puro no encuentra nada: la palabra "placa" está, pero "aceleradora" no.
	rs, err := a.Search("aceleradora gráfica", 5, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Con embeddings el doc de GPU debe quedar primero por similitud.
	if len(rs) == 0 || rs[0].Subject != "hardware" {
		t.Fatalf("esperaba resultado semántico de hardware: %+v", rs)
	}
}

func TestSearchFallsBackToFTSWithoutEmbeddings(t *testing.T) {
	root := t.TempDir()
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	writeDoc(t, root, "materiales/redes/clase.md", "Máscaras y subnetting con CIDR")
	if n, err := a.Ingest(filepath.Join(root, "materiales")); err != nil || n != 1 {
		t.Fatalf("ingest: %d %v", n, err)
	}
	rs, err := a.Search("subnetting", 5, nil)
	if err != nil || len(rs) != 1 {
		t.Fatalf("fallback FTS roto: %+v err=%v", rs, err)
	}
}

func TestCosineAndBlobRoundTrip(t *testing.T) {
	v := []float64{0.25, -0.5, 1}
	got := blobToVec(vecToBlob(v))
	for i := range v {
		if diff := got[i] - v[i]; diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("round trip mismatch at %d: %v vs %v", i, got[i], v[i])
		}
	}
	if c := cosine(v, v); c < 0.9999 {
		t.Fatalf("self cosine = %v", c)
	}
}

func writeDoc(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
