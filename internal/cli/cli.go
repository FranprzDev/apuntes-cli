package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/franciscoperez/apuntes-cli/internal/core"
	"github.com/franciscoperez/apuntes-cli/internal/mcp"
	"github.com/franciscoperez/apuntes-cli/internal/progress"
	"github.com/franciscoperez/apuntes-cli/internal/session"
	"github.com/franciscoperez/apuntes-cli/internal/summary"
)

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
  doctor                   Verifica la salud del workspace local
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
	case "doctor":
		return "uso: apuntes doctor"
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
	a, e := core.New(root)
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
			if !core.SafePath(materiales, abs) {
				return fmt.Errorf("la ruta %s está fuera de materiales/", args[2])
			}
			p = abs
		default:
			return errors.New(commandUsage("ingest"))
		}
		n, e := a.Ingest(p)
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
		r, e := a.Search(query, 10, a.ActiveSubjects())
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
		steps, sources, e := a.SuggestSteps(subj)
		if e != nil {
			return e
		}
		return printJSON(out, map[string]any{"subject": subj, "steps": steps, "sources": sources}, nil)
	case "mcp":
		return mcp.Serve(a, in, out, args[1:])
	case "clase":
		return classCmd(a, args[1:], out)
	case "resumen":
		return summary.Cmd(a, args[1:], out)
	case "progreso":
		return progress.Cmd(a.Root, args[1:], out)
	case "doctor":
		if len(args) > 1 {
			return errors.New(commandUsage("doctor") + ": este comando no acepta argumentos")
		}
		return doctor(a, out)
	default:
		return fmt.Errorf("%s\n\nejecutá `apuntes help` para ver todos los comandos", commandUsage(args[0]))
	}
}

// Doctor reports the health of the local workspace so problems like an empty
// index or a missing materiales/ directory are easy to spot.
func doctor(a *core.App, out io.Writer) error {
	ok := true
	check := func(name string, healthy bool, detail string) {
		status := "ok"
		if !healthy {
			status = "fallo"
			ok = false
		}
		fmt.Fprintf(out, "%-12s [%s] %s\n", name, status, detail)
	}
	materiales := filepath.Join(a.Root, "materiales")
	_, statErr := os.Stat(materiales)
	check("workspace", statErr == nil, materiales)
	n, err := a.DocCount()
	check("índice", err == nil && n > 0, fmt.Sprintf("%d documentos indexados", n))
	var emb int
	a.DB.QueryRow(`SELECT COUNT(*) FROM embeddings`).Scan(&emb)
	if a.Embedder() != nil {
		fmt.Fprintf(out, "%-12s [ok] búsqueda semántica activa (%d/%d documentos con vector)\n", "embeddings", emb, n)
	} else {
		fmt.Fprintf(out, "%-12s [aviso] sin proveedor de embeddings (Ollama); se usa FTS\n", "embeddings")
	}
	if _, e := os.Stat(filepath.Join(a.Root, "data", "profile.json")); e != nil {
		check("perfil", false, "sin perfil; crealo con `apuntes profile init`")
	} else {
		check("perfil", true, filepath.Join(a.Root, "data", "profile.json"))
	}
	if _, serr := os.Stat(session.CurrentSessionPath(a.Root)); serr == nil {
		fmt.Fprintf(out, "%-12s [aviso] hay una sesión de clase abierta; cerrala con `apuntes clase end`\n", "clase")
	} else {
		fmt.Fprintf(out, "%-12s [ok] sin sesiones abiertas\n", "clase")
	}
	if !ok {
		return errors.New("se encontraron problemas; ejecutá `apuntes init` o `apuntes ingest` según corresponda")
	}
	return nil
}

func initWorkspace(a *core.App, out io.Writer) error {
	for _, p := range []string{"data/materias", "materiales"} {
		if e := os.MkdirAll(filepath.Join(a.Root, p), 0755); e != nil {
			return e
		}
	}
	fmt.Fprintln(out, "workspace inicializado en data/")
	return nil
}

func rebuild(a *core.App, out io.Writer) error {
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

func profileCmd(a *core.App, args []string, in io.Reader, out io.Writer) error {
	if len(args) > 0 && (args[0] == "init" || args[0] == "edit") {
		p := core.Profile{}
		if args[0] == "edit" {
			var e error
			p, e = a.GetProfile()
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
		if e := a.SaveProfile(p); e != nil {
			return e
		}
		fmt.Fprintln(out, "perfil guardado")
		return nil
	}
	p, e := a.GetProfile()
	return printJSON(out, p, e)
}

func classCmd(a *core.App, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: apuntes clase start|ask|end")
	}
	switch args[0] {
	case "start":
		topic := strings.Join(args[1:], " ")
		if topic == "" {
			return fmt.Errorf("uso: apuntes clase start <tema>")
		}
		p, err := session.Start(a.Root, topic)
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
		if err := session.AddEntry(a.Root, q, answer); err != nil {
			return err
		}
		fmt.Fprintln(out, "registrado")
		return nil
	case "end":
		p, err := session.End(a.Root)
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
