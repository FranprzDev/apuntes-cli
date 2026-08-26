package core

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// Embedder turns text into dense vectors for semantic search.
type Embedder interface {
	Embed(texts []string) ([][]float64, error)
}

const (
	DefaultOllamaURL  = "http://127.0.0.1:11434"
	DefaultEmbedModel = "all-minilm" // ~46MB, 384 dims
	embedMaxChars     = 4000         // one compact vector per document keeps it light
)

// OllamaEmbedder calls a local Ollama server's /api/embed endpoint.
type OllamaEmbedder struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewOllamaEmbedder() *OllamaEmbedder {
	url := os.Getenv("APUNTES_OLLAMA_URL")
	if url == "" {
		url = DefaultOllamaURL
	}
	model := os.Getenv("APUNTES_EMBED_MODEL")
	if model == "" {
		model = DefaultEmbedModel
	}
	return &OllamaEmbedder{BaseURL: strings.TrimSuffix(url, "/"), Model: model, Client: &http.Client{Timeout: 30 * time.Second}}
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}
type ollamaEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func (o *OllamaEmbedder) Embed(texts []string) ([][]float64, error) {
	for i, t := range texts {
		if len(t) > embedMaxChars {
			texts[i] = t[:embedMaxChars]
		}
	}
	body, err := json.Marshal(ollamaEmbedRequest{Model: o.Model, Input: texts})
	if err != nil {
		return nil, err
	}
	resp, err := o.Client.Post(o.BaseURL+"/api/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errEmbedUnavailable
	}
	var out ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Embeddings, nil
}

var errEmbedUnavailable = &embedError{"proveedor de embeddings no disponible"}

type embedError struct{ msg string }

func (e *embedError) Error() string { return e.msg }

// embedder lazily detects a usable provider; returns nil when none is
// available so every caller falls back to plain FTS.
func (a *App) embedder() Embedder {
	if e, ok := a.embedCache.(Embedder); ok {
		return e
	}
	e := NewOllamaEmbedder()
	ping := *e
	ping.Client = &http.Client{Timeout: 800 * time.Millisecond}
	if _, err := ping.Embed([]string{"ping"}); err == nil {
		a.embedCache = e
		return e
	}
	a.embedCache = unavailableEmbedder{}
	return nil
}

// unavailableEmbedder marks a failed probe so we don't re-dial per query.
type unavailableEmbedder struct{}

func (unavailableEmbedder) Embed([]string) ([][]float64, error) { return nil, errEmbedUnavailable }

func vecToBlob(v []float64) []byte {
	b := make([]byte, len(v)*4)
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(float32(x)))
	}
	return b
}

func blobToVec(b []byte) []float64 {
	v := make([]float64, len(b)/4)
	for i := range v {
		v[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:])))
	}
	return v
}

func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func normalize01(x float64) float64 { return (x + 1) / 2 }

// backfillEmbeddings indexes vectors for documents that still lack one.
// Best-effort: any failure just leaves the docs to FTS-only search.
func (a *App) backfillEmbeddings() {
	e := a.embedder()
	if e == nil {
		return
	}
	rows, err := a.DB.Query(`SELECT d.id,d.path,d.body FROM documents d LEFT JOIN embeddings x ON x.path = d.path WHERE x.path IS NULL LIMIT 64`)
	if err != nil {
		return
	}
	type pending struct {
		path, body string
	}
	var batch []pending
	for rows.Next() {
		var id int64
		var p, body string
		if rows.Scan(&id, &p, &body) == nil {
			batch = append(batch, pending{p, body})
		}
	}
	rows.Close()
	if len(batch) == 0 {
		return
	}
	texts := make([]string, len(batch))
	for i, b := range batch {
		texts[i] = b.body
	}
	vecs, err := e.Embed(texts)
	if err != nil || len(vecs) != len(batch) {
		return
	}
	for i, v := range vecs {
		a.DB.Exec(`INSERT OR REPLACE INTO embeddings(path,vec) VALUES(?,?)`, batch[i].path, vecToBlob(v))
	}
}

// semanticScores returns cosine similarity per document path against the
// query vector, or nil when semantic search is unavailable.
func (a *App) semanticScores(query string, subjects []string) map[string]float64 {
	e := a.embedder()
	if e == nil {
		return nil
	}
	vecs, err := e.Embed([]string{query})
	if err != nil || len(vecs) != 1 {
		return nil
	}
	qvec := vecs[0]
	q := `SELECT d.path,x.vec FROM documents d JOIN embeddings x ON x.path = d.path`
	args := []any{}
	if len(subjects) > 0 {
		q += ` WHERE d.subject IN (` + strings.TrimSuffix(strings.Repeat("?,", len(subjects)), ",") + `)`
		for _, s := range subjects {
			args = append(args, s)
		}
	}
	rows, err := a.DB.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	scores := map[string]float64{}
	for rows.Next() {
		var path string
		var blob []byte
		if rows.Scan(&path, &blob) != nil {
			continue
		}
		scores[path] = normalize01(cosine(qvec, blobToVec(blob)))
	}
	if len(scores) == 0 {
		return nil
	}
	return scores
}
