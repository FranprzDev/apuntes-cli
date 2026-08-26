package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/franciscoperez/apuntes-cli/internal/core"
	"github.com/franciscoperez/apuntes-cli/internal/progress"
	"github.com/franciscoperez/apuntes-cli/internal/session"
	"github.com/franciscoperez/apuntes-cli/internal/summary"
)

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}
type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func Serve(a *core.App, in io.Reader, out io.Writer, args []string) error {
	if len(args) > 0 && args[0] == "install" {
		return install(a, args, out)
	}
	sc := bufio.NewScanner(in)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		var req rpc
		if json.Unmarshal(sc.Bytes(), &req) != nil {
			continue
		}
		res := handleRPC(a, req)
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
	return sc.Err()
}
func tools() []tool {
	return []tool{
		{Name: "listar_materias", Description: "Lista las materias locales y su relevancia para el perfil.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "buscar_material", Description: "Busca evidencia en los materiales locales.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}, "subject": map[string]any{"type": "string"}}, "required": []string{"query"}}},
		{Name: "leer_fuente", Description: "Lee una fuente por su ruta relativa segura.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}}},
		{Name: "sugerir_ruta_de_estudio", Description: "Sugiere una secuencia basada en evidencia disponible.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"subject": map[string]any{"type": "string"}}}},
		{Name: "buscar_ejercicios", Description: "Busca ejercicios relacionados en fuentes locales.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"}}},
		{Name: "obtener_perfil", Description: "Devuelve el perfil local del estudiante.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "iniciar_clase", Description: "Abre una sesión de clase con un tema.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"tema": map[string]any{"type": "string"}}, "required": []string{"tema"}}},
		{Name: "registrar_pregunta", Description: "Registra una pregunta (y opcionalmente su respuesta) en la sesión abierta.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"pregunta": map[string]any{"type": "string"}, "respuesta": map[string]any{"type": "string"}}, "required": []string{"pregunta"}}},
		{Name: "cerrar_clase", Description: "Cierra la sesión de clase abierta.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "guardar_progreso", Description: "Registra el estado de estudio de un tema.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"tema": map[string]any{"type": "string"}, "estado": map[string]any{"type": "string", "enum": []string{"pendiente", "en_proceso", "dominado"}}}, "required": []string{"tema", "estado"}}},
		{Name: "resumir_historial", Description: "Devuelve el progreso por tema ordenado por prioridad.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "generar_resumen", Description: "Genera el resumen .md (y PDF opcional) de la última sesión cerrada.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"sesion": map[string]any{"type": "string"}, "pdf": map[string]any{"type": "boolean"}}}},
	}
}
func handleRPC(a *core.App, r rpc) response {
	ok := func(v any) response { return response{"2.0", r.ID, v, nil} }
	fail := func(s string) response {
		return response{"2.0", r.ID, nil, map[string]any{"code": -32602, "message": s}}
	}
	switch r.Method {
	case "initialize":
		return ok(map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "apuntes-cli", "version": "0.1.0"}})
	case "notifications/initialized":
		return response{JSONRPC: "2.0"}
	case "tools/list":
		return ok(map[string]any{"tools": tools()})
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if json.Unmarshal(r.Params, &p) != nil {
			return fail("parámetros inválidos")
		}
		return callTool(a, r.ID, p.Name, p.Arguments, ok, fail)
	default:
		return fail("método no soportado: " + r.Method)
	}
}
func callTool(a *core.App, id any, name string, args map[string]any, ok func(any) response, fail func(string) response) response {
	switch name {
	case "obtener_perfil":
		p, e := a.GetProfile()
		if e != nil {
			return fail(e.Error())
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": core.JSONString(p)}}})
	case "iniciar_clase":
		tema, _ := args["tema"].(string)
		if strings.TrimSpace(tema) == "" {
			return fail("falta el tema")
		}
		p, e := session.Start(a.Root, tema)
		if e != nil {
			return fail(e.Error())
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": "sesión iniciada: " + p}}})
	case "registrar_pregunta":
		pregunta, _ := args["pregunta"].(string)
		respuesta, _ := args["respuesta"].(string)
		if strings.TrimSpace(pregunta) == "" {
			return fail("falta la pregunta")
		}
		if e := session.AddEntry(a.Root, pregunta, respuesta); e != nil {
			return fail(e.Error())
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": "registrado"}}})
	case "guardar_progreso":
		tema, _ := args["tema"].(string)
		estado, _ := args["estado"].(string)
		if estado == "" {
			estado = "en_proceso"
		}
		t, e := progress.Set(a.Root, tema, estado)
		if e != nil {
			return fail(e.Error())
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": core.JSONString(t)}}})
	case "resumir_historial":
		type row struct {
			Topic    string                 `json:"tema"`
			Progress progress.TopicProgress `json:"progreso"`
		}
		rows, e := progress.History(a.Root)
		if e != nil {
			return fail(e.Error())
		}
		out := make([]row, 0, len(rows))
		for _, r := range rows {
			out = append(out, row{r.Topic, r.Progress})
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": core.JSONString(out)}}})
	case "generar_resumen":
		sesion, _ := args["sesion"].(string)
		wantPDF, _ := args["pdf"].(bool)
		mdPath, pdfPath, e := summary.Generate(a, sesion, wantPDF)
		if e != nil {
			return fail(e.Error())
		}
		payload := map[string]any{"markdown": mdPath}
		if pdfPath != "" {
			payload["pdf"] = pdfPath
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": core.JSONString(payload)}}})
	case "cerrar_clase":
		p, e := session.End(a.Root)
		if e != nil {
			return fail(e.Error())
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": "sesión cerrada: " + p}}})
	case "buscar_material", "buscar_ejercicios":
		q, _ := args["query"].(string)
		sub, _ := args["subject"].(string)
		if name == "buscar_ejercicios" {
			q += " ejercicios prácticos"
		}
		subjects := []string{}
		if sub != "" {
			subjects = []string{sub}
		} else {
			subjects = a.ActiveSubjects()
		}
		r, e := a.Search(q, 10, subjects)
		if e != nil {
			return fail(e.Error())
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": core.JSONString(r)}}})
	case "listar_materias":
		rows, e := a.DB.Query(`SELECT subject,count(*) FROM documents GROUP BY subject ORDER BY subject`)
		if e != nil {
			return fail(e.Error())
		}
		defer rows.Close()
		var vals []map[string]any
		for rows.Next() {
			var s string
			var n int
			rows.Scan(&s, &n)
			vals = append(vals, map[string]any{"subject": s, "sources": n})
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": core.JSONString(vals)}}})
	case "leer_fuente":
		p, _ := args["path"].(string)
		full := filepath.Join(a.Root, p)
		if resolved, e := filepath.EvalSymlinks(full); e == nil {
			full = resolved
		}
		materiales := filepath.Join(a.Root, "materiales")
		if resolved, e := filepath.EvalSymlinks(materiales); e == nil {
			materiales = resolved
		}
		if !core.SafePath(materiales, full) {
			return fail("la fuente debe estar dentro de materiales/")
		}
		var n int
		if e := a.DB.QueryRow(`SELECT COUNT(*) FROM documents WHERE path = ?`, filepath.ToSlash(p)).Scan(&n); e != nil || n == 0 {
			return fail("la fuente no está indexada: " + p)
		}
		b, e := os.ReadFile(full)
		if e != nil {
			return fail(e.Error())
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": string(b)}, map[string]any{"type": "text", "text": "Fuente: " + p}}})
	case "sugerir_ruta_de_estudio":
		sub, _ := args["subject"].(string)
		steps, r, e := a.SuggestSteps(sub)
		if e != nil {
			return fail(e.Error())
		}
		if steps == nil {
			steps = []string{}
		}
		return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": core.JSONString(map[string]any{"steps": steps, "sources": r})}}})
	default:
		return fail("herramienta no soportada: " + name)
	}
}

func install(a *core.App, args []string, out io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("uso: mcp install --agent claude|codex")
	}
	cfg := map[string]any{"mcpServers": map[string]any{"apuntes": map[string]any{"command": filepath.Join(a.Root, "apuntes"), "args": []string{"mcp"}}}}
	fmt.Fprintln(out, core.JSONString(cfg))
	return nil
}
