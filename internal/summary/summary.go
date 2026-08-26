package summary

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/franciscoperez/apuntes-cli/internal/core"
	"github.com/franciscoperez/apuntes-cli/internal/session"
)

// keywords extracts the most frequent significant terms from the
// session's own Q&A content, so every part of the generated document is
// grounded in what actually happened during the class.
func keywords(s *session.Session, n int) []string {
	freq := map[string]int{}
	for _, e := range s.Entries {
		for _, w := range strings.Fields(strings.ToLower(e.Question + " " + e.Answer)) {
			w = strings.Trim(w, ".,;:()¡!¿?\"'«»")
			if len(w) < 4 || core.Stopwords[w] {
				continue
			}
			freq[w]++
		}
	}
	type kv struct {
		w string
		c int
	}
	var top []kv
	for w, c := range freq {
		top = append(top, kv{w, c})
	}
	sort.Slice(top, func(i, j int) bool {
		return top[i].c > top[j].c || (top[i].c == top[j].c && top[i].w < top[j].w)
	})
	if len(top) > n {
		top = top[:n]
	}
	out := make([]string, 0, len(top))
	for _, t := range top {
		out = append(out, t.w)
	}
	return out
}

func relatedSources(a *core.App, kws []string) []core.Source {
	seen := map[int64]bool{}
	var out []core.Source
	for _, k := range kws {
		rs, err := a.Search(k, 3, nil)
		if err != nil {
			continue
		}
		for _, s := range rs {
			if !seen[s.ID] {
				seen[s.ID] = true
				s.Text = ""
				out = append(out, s)
			}
		}
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func buildMarkdown(s *session.Session, kws []string, sources []core.Source) string {
	var b strings.Builder
	b.WriteString("# Resumen de clase: " + s.Topic + "\n\n")
	fmt.Fprintf(&b, "- **Inicio:** %s\n- **Fin:** %s\n- **Preguntas registradas:** %d\n\n", s.Start, s.End, len(s.Entries))

	b.WriteString("## Brief\n\n")
	if len(kws) > 0 {
		b.WriteString("Conceptos centrales trabajados en la sesión: " + strings.Join(kws, ", ") + ".\n\n")
	} else {
		b.WriteString("La sesión no registró preguntas.\n\n")
	}

	b.WriteString("## Resumen gerencial\n\n")
	fmt.Fprintf(&b, "Durante la clase de «%s» se registraron %d preguntas", s.Topic, len(s.Entries))
	answered := 0
	for _, e := range s.Entries {
		if e.Answer != "" {
			answered++
		}
	}
	fmt.Fprintf(&b, ", de las cuales %d quedaron con respuesta cerrada en la sesión", answered)
	if unanswered := len(s.Entries) - answered; unanswered > 0 {
		fmt.Fprintf(&b, " y %d quedaron pendientes de resolver", unanswered)
	}
	b.WriteString(". Usá la lista de preguntas como checklist de repaso.\n\n")

	b.WriteString("## Preguntas de la clase\n\n")
	for i, e := range s.Entries {
		fmt.Fprintf(&b, "### %d. %s\n\n", i+1, e.Question)
		if e.Time != "" {
			fmt.Fprintf(&b, "_%s_\n\n", e.Time)
		}
		if e.Answer != "" {
			fmt.Fprintf(&b, "%s\n\n", e.Answer)
		} else {
			b.WriteString("_Sin respuesta registrada._\n\n")
		}
	}

	if len(sources) > 0 {
		b.WriteString("## Fuentes relacionadas en tu índice\n\n")
		for _, s2 := range sources {
			fmt.Fprintf(&b, "- [%s](%s) (%s)\n", s2.Title, s2.Path, s2.Subject)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Generate builds the markdown (and optional PDF) for a session and
// writes it under resumenes/. It returns the markdown path and PDF path.
func Generate(a *core.App, sessionPath string, wantPDF bool) (string, string, error) {
	if sessionPath == "" {
		p, _, err := session.LatestClosed(a.Root)
		if err != nil {
			return "", "", err
		}
		sessionPath = p
	}
	s, err := session.LoadSession(sessionPath)
	if err != nil {
		return "", "", err
	}
	kws := keywords(s, 6)
	md := buildMarkdown(s, kws, relatedSources(a, kws))

	destDir := filepath.Join(a.Root, "resumenes")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", "", err
	}
	name := strings.TrimSuffix(filepath.Base(sessionPath), filepath.Ext(sessionPath)) + ".md"
	dest := filepath.Join(destDir, name)
	if err := os.WriteFile(dest, []byte(md), 0644); err != nil {
		return "", "", err
	}
	pdfPath := ""
	if wantPDF {
		pdfPath, err = ToPDF(s, dest)
		if err != nil {
			return "", "", err
		}
	}
	return dest, pdfPath, nil
}

func Cmd(a *core.App, args []string, out io.Writer) error {
	sessionPath := ""
	wantPDF := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--sesion":
			if i+1 >= len(args) {
				return fmt.Errorf("uso: apuntes resumen [--sesion <archivo.json>] [--pdf]")
			}
			sessionPath = args[i+1]
			i++
		case "--pdf":
			wantPDF = true
		default:
			return fmt.Errorf("uso: apuntes resumen [--sesion <archivo.json>] [--pdf]")
		}
	}
	dest, pdfPath, err := Generate(a, sessionPath, wantPDF)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "resumen generado: %s\n", dest)
	if pdfPath != "" {
		fmt.Fprintf(out, "pdf generado: %s\n", pdfPath)
	}
	return nil
}
