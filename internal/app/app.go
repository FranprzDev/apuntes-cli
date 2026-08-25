package app

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

type Profile struct {
	Institution    string   `json:"institucion"`
	Career         string   `json:"carrera"`
	Year           int      `json:"año"`
	ActiveSubjects []string `json:"materias_activas"`
	Objective      string   `json:"objetivo"`
}
type Source struct {
	ID       int64    `json:"id"`
	Subject  string   `json:"subject"`
	Title    string   `json:"title"`
	Path     string   `json:"path"`
	Location string   `json:"location"`
	Text     string   `json:"text"`
	Year     int      `json:"year"`
	Topics   []string `json:"topics"`
}
type App struct {
	Root string
	DB   *sql.DB
}

func New(root string) (*App, error) {
	root, _ = filepath.Abs(root)
	if err := os.MkdirAll(filepath.Join(root, "data"), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "data", "index.db"))
	if err != nil {
		return nil, err
	}
	a := &App{root, db}
	if err = a.schema(); err != nil {
		db.Close()
	}
	return a, err
}
func (a *App) schema() error {
	_, err := a.DB.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE IF NOT EXISTS documents (id INTEGER PRIMARY KEY, subject TEXT NOT NULL, title TEXT, path TEXT UNIQUE NOT NULL, location TEXT, year INTEGER, topics TEXT, body TEXT NOT NULL); CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(subject,title,path,topics,body,content='documents',content_rowid='id'); CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN INSERT INTO documents_fts(rowid,subject,title,path,topics,body) VALUES (new.id,new.subject,new.title,new.path,new.topics,new.body); END; CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN INSERT INTO documents_fts(documents_fts,rowid,subject,title,path,topics,body) VALUES ('delete',old.id,old.subject,old.title,old.path,old.topics,old.body); END; CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN INSERT INTO documents_fts(documents_fts,rowid,subject,title,path,topics,body) VALUES ('delete',old.id,old.subject,old.title,old.path,old.topics,old.body); INSERT INTO documents_fts(rowid,subject,title,path,topics,body) VALUES (new.id,new.subject,new.title,new.path,new.topics,new.body); END;`)
	return err
}
func (a *App) Close() {
	if a.DB != nil {
		a.DB.Close()
	}
}
func (a *App) ingest(path string) (int, error) {
	n := 0
	materiales := filepath.Join(a.Root, "materiales")
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	check := abs
	if resolved, e := filepath.EvalSymlinks(abs); e == nil {
		check = resolved
	}
	if resolved, e := filepath.EvalSymlinks(materiales); e == nil {
		materiales = resolved
	}
	if !safePath(materiales, check) {
		return 0, fmt.Errorf("la ruta %s está fuera de materiales/", path)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return 0, err
	}
	walk := func(p string, d os.DirEntry) error {
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".md" && ext != ".txt" && ext != ".pdf" {
			return nil
		}
		target := p
		if resolved, e := filepath.EvalSymlinks(p); e == nil {
			target = resolved
		} else if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !safePath(materiales, target) {
			return fmt.Errorf("la ruta %s apunta fuera de materiales/", p)
		}
		b, err := readFile(p, ext)
		if err != nil {
			return nil
		}
		absolute, _ := filepath.Abs(p)
		rel, _ := filepath.Rel(a.Root, absolute)
		subj := subjectFromPath(p)
		_, err = a.DB.Exec(`INSERT INTO documents(subject,title,path,location,year,topics,body) VALUES(?,?,?,?,?,?,?) ON CONFLICT(path) DO UPDATE SET subject=excluded.subject,title=excluded.title,location=excluded.location,year=excluded.year,topics=excluded.topics,body=excluded.body`, subj, strings.TrimSuffix(filepath.Base(p), ext), rel, "", 0, "", string(b))
		if err != nil {
			return err
		}
		n++
		return nil
	}
	if info.IsDir() {
		err = filepath.WalkDir(abs, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			return walk(p, d)
		})
	} else {
		err = walk(path, dirEntry{info})
	}
	return n, err
}

type dirEntry struct{ os.FileInfo }

func (d dirEntry) Type() os.FileMode          { return d.Mode() }
func (d dirEntry) Info() (os.FileInfo, error) { return d.FileInfo, nil }
func (d dirEntry) Name() string               { return d.FileInfo.Name() }
func (d dirEntry) IsDir() bool                { return d.FileInfo.IsDir() }
func subjectFromPath(p string) string {
	parts := strings.Split(filepath.ToSlash(p), "/")
	for i, x := range parts {
		if (x == "materias" || x == "materiales") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "general"
}
func readFile(p, ext string) ([]byte, error) {
	b, e := os.ReadFile(p)
	if ext != ".pdf" || e != nil {
		return b, e
	}
	text, ok := extractPDF(b)
	if !ok {
		return nil, fmt.Errorf("%s: no se pudo extraer texto del PDF", p)
	}
	return text, nil
}
func extractPDF(b []byte) ([]byte, bool) {
	s := string(b)
	var out []string
	for _, x := range strings.Split(s, "stream")[1:] {
		x = strings.Split(x, "endstream")[0]
		x = strings.ReplaceAll(x, "\\n", " ")
		x = strings.ReplaceAll(x, "\\(", "(")
		x = strings.ReplaceAll(x, "\\)", ")")
		for _, line := range strings.Split(x, "\n") {
			if strings.Contains(line, "Tj") || strings.Contains(line, "TJ") {
				line = strings.TrimSpace(line)
				line = strings.TrimSuffix(line, "Tj")
				line = strings.TrimSuffix(line, "TJ")
				line = strings.Trim(line, "[]()")
				if line != "" {
					out = append(out, line)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return []byte(strings.Join(out, " ")), true
}

// activeSubjects returns the profile's active subjects; empty means no filter.
func (a *App) activeSubjects() []string {
	p, err := a.profile()
	if err != nil || len(p.ActiveSubjects) == 0 {
		return nil
	}
	return p.ActiveSubjects
}

func (a *App) search(q string, limit int, subjects []string) ([]Source, error) {
	q = ftsQuery(q)
	if q == "" {
		return nil, nil
	}
	args := []any{q}
	extra := ""
	if len(subjects) > 0 {
		ph := make([]string, len(subjects))
		for i, s := range subjects {
			ph[i] = "?"
			args = append(args, s)
		}
		extra = " AND d.subject IN (" + strings.Join(ph, ",") + ")"
	}
	args = append(args, limit)
	rows, err := a.DB.Query(`SELECT d.id,d.subject,d.title,d.path,d.location,d.year,d.topics,d.body FROM documents_fts f JOIN documents d ON d.id=f.rowid WHERE documents_fts MATCH ?`+extra+` ORDER BY bm25(documents_fts) LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var s Source
		var topics string
		if err := rows.Scan(&s.ID, &s.Subject, &s.Title, &s.Path, &s.Location, &s.Year, &topics, &s.Text); err != nil {
			return nil, err
		}
		s.Topics = strings.Fields(topics)
		s.Text = snippet(s.Text, q)
		out = append(out, s)
	}
	return out, rows.Err()
}
func ftsQuery(q string) string {
	var parts []string
	for _, x := range strings.Fields(q) {
		x = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`"'():*+-`, r) {
				return -1
			}
			return r
		}, x)
		if x != "" {
			parts = append(parts, "\""+x+"\"")
		}
	}
	return strings.Join(parts, " AND ")
}
func snippet(s, q string) string {
	if len(s) <= 500 {
		return s
	}
	words := strings.Fields(q)
	pos := strings.Index(strings.ToLower(s), strings.ToLower(words[0]))
	if pos < 0 {
		pos = 0
	}
	start := pos - 180
	if start < 0 {
		start = 0
	}
	end := start + 500
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
func (a *App) profile() (Profile, error) {
	var p Profile
	b, e := os.ReadFile(filepath.Join(a.Root, "data", "profile.json"))
	if os.IsNotExist(e) {
		return p, nil
	}
	if e != nil {
		return p, e
	}
	e = json.Unmarshal(b, &p)
	return p, e
}
func (a *App) saveProfile(p Profile) error {
	b, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(filepath.Join(a.Root, "data", "profile.json"), append(b, '\n'), 0644)
}

func usage(out io.Writer) {
	fmt.Fprint(out, `apuntes — índice local de material de estudio

Uso: apuntes <comando> [opciones]

Comandos:
  init                     Inicializa el workspace (data/, materiales/)
  ingest [--path DIR]      Indexa materiales/ (o un subdirectorio de materiales/)
  index                    Reconstruye el índice de texto completo
  search <consulta>        Busca en las fuentes indexadas (salida JSON)
  profile [init|edit]      Muestra o edita el perfil de estudio
  study-path [--subject X] Sugiere una ruta de estudio basada en el índice
  clase start|ask|end      Registra una clase: tema, preguntas y respuestas
  resumen [--sesion FILE] [--pdf]
                           Genera un .md (y PDF opcional) de repaso
  progreso [set T E]       Muestra o registra el progreso por tema
  mcp                      Sirve el servidor MCP por stdio
  help [comando]           Muestra esta ayuda o la de un comando
`)
}

func commandUsage(cmd string) string {
	switch cmd {
	case "ingest":
		return "uso: apuntes ingest [--path <subdirectorio de materiales/>]"
	case "search":
		return "uso: apuntes search <consulta>"
	case "profile":
		return "uso: apuntes profile [init|edit]"
	case "study-path":
		return "uso: apuntes study-path [--subject <materia>]"
	case "clase":
		return "uso: apuntes clase start <tema> | clase ask \"<pregunta>\" [--respuesta \"<texto>\"] | clase end"
	case "resumen":
		return "uso: apuntes resumen [--sesion <archivo.json>] [--pdf]"
	case "progreso":
		return "uso: apuntes progreso [set <tema> <pendiente|en_proceso|dominado>]"
	case "mcp":
		return "uso: apuntes mcp | apuntes mcp install --agent claude|codex"
	default:
		return "comando desconocido: " + cmd
	}
}

func Run(args []string, out io.Writer, in io.Reader) error {
	root, _ := os.Getwd()
	if len(args) == 0 || args[0] == "help" {
		if len(args) > 1 && args[0] == "help" {
			fmt.Fprintln(out, commandUsage(args[1]))
			return nil
		}
		usage(out)
		return nil
	}
	a, e := New(root)
	if e != nil {
		return e
	}
	defer a.Close()
	switch args[0] {
	case "init":
		if len(args) > 1 {
			return errors.New(commandUsage("init") + ": este comando no acepta argumentos")
		}
		return initWorkspace(a, out)
	case "ingest":
		materiales := filepath.Join(root, "materiales")
		p := materiales
		switch {
		case len(args) == 1:
		case len(args) == 3 && args[1] == "--path":
			abs, err := filepath.Abs(args[2])
			if err != nil {
				return err
			}
			if !safePath(materiales, abs) {
				return fmt.Errorf("la ruta %s está fuera de materiales/", args[2])
			}
			p = abs
		default:
			return errors.New(commandUsage("ingest"))
		}
		n, e := a.ingest(p)
		if e == nil {
			fmt.Fprintf(out, "%d fuentes indexadas\n", n)
		}
		return e
	case "index":
		if len(args) > 1 {
			return errors.New(commandUsage("index") + ": este comando no acepta argumentos")
		}
		return rebuild(a, out)
	case "search":
		if len(args) < 2 {
			return errors.New(commandUsage("search"))
		}
		query := strings.Join(args[1:], " ")
		r, e := a.search(query, 10, a.activeSubjects())
		return printJSON(out, r, e)
	case "profile":
		if len(args) > 2 || (len(args) == 2 && args[1] != "init" && args[1] != "edit") {
			return errors.New(commandUsage("profile"))
		}
		return profileCmd(a, args[1:], in, out)
	case "study-path":
		subj := ""
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--subject":
				if i+1 >= len(args) {
					return errors.New(commandUsage("study-path"))
				}
				subj = args[i+1]
				i++
			default:
				return errors.New(commandUsage("study-path"))
			}
		}
		steps, sources, e := a.suggestSteps(subj)
		if e != nil {
			return e
		}
		return printJSON(out, map[string]any{"subject": subj, "steps": steps, "sources": sources}, nil)
	case "mcp":
		return ServeMCP(a, in, out, args[1:])
	case "clase":
		return classCmd(a, args[1:], out)
	case "resumen":
		return summaryCmd(a, args[1:], out)
	case "progreso":
		return progressCmd(a, args[1:], out)
	default:
		return fmt.Errorf("%s\n\nejecutá `apuntes help` para ver todos los comandos", commandUsage(args[0]))
	}
}
func initWorkspace(a *App, out io.Writer) error {
	for _, p := range []string{"data/materias", "materiales"} {
		if e := os.MkdirAll(filepath.Join(a.Root, p), 0755); e != nil {
			return e
		}
	}
	fmt.Fprintln(out, "workspace inicializado en data/")
	return nil
}
func rebuild(a *App, out io.Writer) error {
	_, e := a.DB.Exec(`INSERT INTO documents_fts(documents_fts) VALUES('rebuild')`)
	if e == nil {
		fmt.Fprintln(out, "índice reconstruido")
	}
	return e
}
func printJSON(out io.Writer, v any, e error) error {
	if e != nil {
		return e
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func profileCmd(a *App, args []string, in io.Reader, out io.Writer) error {
	if len(args) > 0 && (args[0] == "init" || args[0] == "edit") {
		p := Profile{}
		if args[0] == "edit" {
			var e error
			p, e = a.profile()
			if e != nil {
				return e
			}
		}
		sc := bufio.NewScanner(in)
		for _, q := range []struct {
			k   string
			dst *string
		}{{"Institución", &p.Institution}, {"Carrera", &p.Career}, {"Objetivo", &p.Objective}} {
			fmt.Fprint(out, q.k+": ")
			sc.Scan()
			*q.dst = sc.Text()
		}
		fmt.Fprint(out, "Materias activas (coma separada): ")
		sc.Scan()
		p.ActiveSubjects = strings.FieldsFunc(sc.Text(), func(r rune) bool { return r == ',' || r == ';' })
		if e := a.saveProfile(p); e != nil {
			return e
		}
		fmt.Fprintln(out, "perfil guardado")
		return nil
	}
	p, e := a.profile()
	return printJSON(out, p, e)
}

var stopwords = map[string]bool{"de": true, "la": true, "el": true, "los": true, "las": true, "y": true, "o": true, "a": true, "en": true, "que": true, "del": true, "al": true, "con": true, "por": true, "para": true, "un": true, "una": true, "es": true, "se": true, "su": true, "lo": true, "como": true, "the": true, "of": true, "and": true, "to": true, "in": true}

func (a *App) suggestSteps(subject string) ([]string, []Source, error) {
	sources, err := a.allSources(subject)
	if err != nil {
		return nil, nil, err
	}
	freq := map[string]int{}
	docs := map[string]int{}
	for _, s := range sources {
		seen := map[string]bool{}
		for _, w := range strings.Fields(strings.ToLower(s.Title + " " + s.Text)) {
			w = strings.Trim(w, ".,;:()¡!¿?\"'")
			if len(w) < 4 || stopwords[w] || seen[w] {
				continue
			}
			seen[w] = true
			freq[w]++
			docs[w]++
		}
	}
	type kv struct {
		w string
		n int
	}
	var top []kv
	for w, n := range freq {
		top = append(top, kv{w, n})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].n > top[j].n || (top[i].n == top[j].n && top[i].w < top[j].w) })
	if len(top) > 6 {
		top = top[:6]
	}
	var steps []string
	for _, t := range top {
		steps = append(steps, fmt.Sprintf("Repasar «%s» (aparece en %d fuente(s))", t.w, docs[t.w]))
	}
	return steps, sources, nil
}

func (a *App) allSources(subject string) ([]Source, error) {
	q := `SELECT id,subject,title,path,location,year,topics,body FROM documents`
	args := []any{}
	if subject != "" {
		q += ` WHERE subject = ?`
		args = append(args, subject)
	} else if active := a.activeSubjects(); len(active) > 0 {
		q += ` WHERE subject IN (` + strings.TrimSuffix(strings.Repeat("?,", len(active)), ",") + `)`
		for _, s := range active {
			args = append(args, s)
		}
	}
	q += ` ORDER BY title LIMIT 50`
	rows, err := a.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var s Source
		var topics string
		if err := rows.Scan(&s.ID, &s.Subject, &s.Title, &s.Path, &s.Location, &s.Year, &topics, &s.Text); err != nil {
			return nil, err
		}
		s.Topics = strings.Fields(topics)
		out = append(out, s)
	}
	return out, rows.Err()
}
