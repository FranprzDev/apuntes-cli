package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

// DocCount returns how many sources are currently indexed.
func (a *App) DocCount() (int, error) {
	var n int
	err := a.DB.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&n)
	return n, err
}

func (a *App) Ingest(path string) (int, error) {
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
	if !SafePath(materiales, check) {
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
		if !SafePath(materiales, target) {
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

// ActiveSubjects returns the profile's active subjects; empty means no filter.
func (a *App) ActiveSubjects() []string {
	p, err := a.GetProfile()
	if err != nil || len(p.ActiveSubjects) == 0 {
		return nil
	}
	return p.ActiveSubjects
}

func (a *App) Search(q string, limit int, subjects []string) ([]Source, error) {
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
func (a *App) GetProfile() (Profile, error) {
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
func (a *App) SaveProfile(p Profile) error {
	b, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(filepath.Join(a.Root, "data", "profile.json"), append(b, '\n'), 0644)
}

var Stopwords = map[string]bool{"de": true, "la": true, "el": true, "los": true, "las": true, "y": true, "o": true, "a": true, "en": true, "que": true, "del": true, "al": true, "con": true, "por": true, "para": true, "un": true, "una": true, "es": true, "se": true, "su": true, "lo": true, "como": true, "the": true, "of": true, "and": true, "to": true, "in": true}

func (a *App) SuggestSteps(subject string) ([]string, []Source, error) {
	sources, err := a.AllSources(subject)
	if err != nil {
		return nil, nil, err
	}
	freq := map[string]int{}
	docs := map[string]int{}
	for _, s := range sources {
		seen := map[string]bool{}
		for _, w := range strings.Fields(strings.ToLower(s.Title + " " + s.Text)) {
			w = strings.Trim(w, ".,;:()¡!¿?\"'")
			if len(w) < 4 || Stopwords[w] || seen[w] {
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

func (a *App) AllSources(subject string) ([]Source, error) {
	q := `SELECT id,subject,title,path,location,year,topics,body FROM documents`
	args := []any{}
	if subject != "" {
		q += ` WHERE subject = ?`
		args = append(args, subject)
	} else if active := a.ActiveSubjects(); len(active) > 0 {
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

// SafePath reports whether p stays inside root.
func SafePath(root, p string) bool {
	r, _ := filepath.Abs(root)
	x, _ := filepath.Abs(p)
	return x == r || strings.HasPrefix(x, r+string(os.PathSeparator))
}

func JSONString(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }
