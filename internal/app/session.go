package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Entry struct {
	Time     string `json:"tiempo"`
	Question string `json:"pregunta"`
	Answer   string `json:"respuesta,omitempty"`
}

type Session struct {
	Topic   string  `json:"tema"`
	Start   string  `json:"inicio"`
	End     string  `json:"fin,omitempty"`
	Entries []Entry `json:"entradas"`
}

func sessionsDir(root string) string {
	return filepath.Join(root, "data", "sessions")
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		switch r {
		case 'á':
			return 'a'
		case 'é', 'è':
			return 'e'
		case 'í':
			return 'i'
		case 'ó':
			return 'o'
		case 'ú', 'ü':
			return 'u'
		case 'ñ':
			return 'n'
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r == ' ', r == '-', r == '_':
			return '-'
		default:
			return -1
		}
	}, s)
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func currentSessionPath(root string) string {
	return filepath.Join(sessionsDir(root), "current.json")
}

func loadSession(path string) (*Session, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("sesión corrupta: %w", err)
	}
	return &s, nil
}

func saveSession(root string, path string, s *Session) error {
	if err := os.MkdirAll(sessionsDir(root), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}

// startSession opens a new class session; fails if one is already open.
func startSession(root, topic string) (string, error) {
	cur := currentSessionPath(root)
	if _, err := os.Stat(cur); err == nil {
		return "", fmt.Errorf("ya hay una sesión abierta (%s); cerrala con `apuntes clase end`", cur)
	}
	now := time.Now().Format(time.RFC3339)
	s := &Session{Topic: topic, Start: now, Entries: []Entry{}}
	name := time.Now().Format("2006-01-02-1504")
	if sl := slugify(topic); sl != "" {
		name += "-" + sl
	}
	path := filepath.Join(sessionsDir(root), name+".json")
	if err := saveSession(root, path, s); err != nil {
		return "", err
	}
	if err := saveSession(root, cur, s); err != nil {
		return "", err
	}
	_ = os.WriteFile(filepath.Join(sessionsDir(root), "current.path"), []byte(filepath.Base(path)), 0644)
	return path, nil
}

// addEntry appends a Q (and optional A) to the open session.
func addEntry(root, question, answer string) error {
	cur := currentSessionPath(root)
	s, err := loadSession(cur)
	if err != nil {
		return fmt.Errorf("no hay sesión abierta: iniciá una con `apuntes clase start <tema>`")
	}
	e := Entry{Time: time.Now().Format(time.RFC3339), Question: question, Answer: answer}
	s.Entries = append(s.Entries, e)
	path := ""
	if b, err := os.ReadFile(filepath.Join(sessionsDir(root), "current.path")); err == nil {
		path = filepath.Join(sessionsDir(root), strings.TrimSpace(string(b)))
	}
	if path == "" {
		return fmt.Errorf("archivo de sesión no encontrado")
	}
	if err := saveSession(root, path, s); err != nil {
		return err
	}
	return saveSession(root, cur, s)
}

// endSession closes the open session and returns its file path.
func endSession(root string) (string, error) {
	cur := currentSessionPath(root)
	s, err := loadSession(cur)
	if err != nil {
		return "", fmt.Errorf("no hay sesión abierta: iniciá una con `apuntes clase start <tema>`")
	}
	s.End = time.Now().Format(time.RFC3339)
	path := ""
	if b, err := os.ReadFile(filepath.Join(sessionsDir(root), "current.path")); err == nil {
		path = filepath.Join(sessionsDir(root), strings.TrimSpace(string(b)))
	}
	if path == "" {
		return "", fmt.Errorf("archivo de sesión no encontrado")
	}
	if err := saveSession(root, path, s); err != nil {
		return "", err
	}
	os.Remove(cur)
	os.Remove(filepath.Join(sessionsDir(root), "current.path"))
	return path, nil
}

func classCmd(a *App, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: apuntes clase start|ask|end")
	}
	switch args[0] {
	case "start":
		topic := strings.Join(args[1:], " ")
		if topic == "" {
			return fmt.Errorf("uso: apuntes clase start <tema>")
		}
		p, err := startSession(a.Root, topic)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "sesión iniciada: %s\n", p)
		return nil
	case "ask":
		q := strings.Join(args[1:], " ")
		if q == "" {
			return fmt.Errorf("uso: apuntes clase ask \"<pregunta>\" [--respuesta \"<texto>\"]")
		}
		answer := ""
		if i := indexOf(args[1:], "--respuesta"); i >= 0 && i+1 < len(args[1:]) {
			answer = args[1:][i+1]
			q = strings.Join(args[1:][:i], " ")
		}
		if err := addEntry(a.Root, q, answer); err != nil {
			return err
		}
		fmt.Fprintln(out, "registrado")
		return nil
	case "end":
		p, err := endSession(a.Root)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "sesión cerrada: %s\n", p)
		return nil
	default:
		return fmt.Errorf("subcomando desconocido: %s (usá start|ask|end)", args[0])
	}
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// latestClosedSession returns the most recent closed session file.
func latestClosedSession(root string) (string, *Session, error) {
	matches, err := filepath.Glob(filepath.Join(sessionsDir(root), "2*.json"))
	if err != nil || len(matches) == 0 {
		return "", nil, fmt.Errorf("no hay sesiones cerradas en data/sessions/")
	}
	var newest string
	var newestMod time.Time
	for _, m := range matches {
		info, e := os.Stat(m)
		if e == nil && info.ModTime().After(newestMod) {
			newest, newestMod = m, info.ModTime()
		}
	}
	s, err := loadSession(newest)
	if err != nil {
		return "", nil, err
	}
	return newest, s, nil
}
