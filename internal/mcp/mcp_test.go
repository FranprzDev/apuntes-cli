package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/franciscoperez/apuntes-cli/internal/core"
)

func TestMCPStdioEndToEnd(t *testing.T) {
	a := newMCPFixture(t)

	initialize := callMCP(t, a, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if initialize["error"] != nil {
		t.Fatalf("initialize failed: %#v", initialize["error"])
	}
	if initialize["result"].(map[string]any)["protocolVersion"] != "2024-11-05" {
		t.Fatalf("unexpected initialize response: %#v", initialize)
	}

	list := callMCP(t, a, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	result := list["result"].(map[string]any)
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) != 12 {
		t.Fatalf("expected twelve MCP tools, got %#v", result["tools"])
	}

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "listar_materias", args: map[string]any{}, want: "redes"},
		{name: "buscar_material", args: map[string]any{"query": "subnetting", "subject": "redes"}, want: "subnetting"},
		{name: "leer_fuente", args: map[string]any{"path": "materiales/redes/subnetting.md"}, want: "Fuente:"},
		{name: "sugerir_ruta_de_estudio", args: map[string]any{"subject": "redes"}, want: "Repasar «cidr»"},
		{name: "buscar_ejercicios", args: map[string]any{"query": "CIDR"}, want: "ejercicios"},
		{name: "obtener_perfil", args: map[string]any{}, want: "UTN FRT"},
		{name: "iniciar_clase", args: map[string]any{"tema": "Subnetting"}, want: "sesión iniciada"},
		{name: "registrar_pregunta", args: map[string]any{"pregunta": "¿Qué es CIDR?", "respuesta": "/24 = 24 bits de red"}, want: "registrado"},
		{name: "registrar_pregunta", args: map[string]any{"pregunta": "¿Cómo calculo hosts?"}, want: "registrado"},
		{name: "cerrar_clase", args: map[string]any{}, want: "sesión cerrada"},
		{name: "guardar_progreso", args: map[string]any{"tema": "CIDR", "estado": "dominado"}, want: "dominado"},
		{name: "resumir_historial", args: map[string]any{}, want: "CIDR"},
		{name: "generar_resumen", args: map[string]any{}, want: "markdown"},
	}
	for i, tc := range cases {
		params, _ := json.Marshal(map[string]any{"name": tc.name, "arguments": tc.args})
		request := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":%s}`, i+3, params)
		response := callMCP(t, a, request)
		if response["error"] != nil {
			t.Fatalf("%s returned an error: %#v", tc.name, response["error"])
		}
		text := mcpText(t, response)
		if !containsFold(text, tc.want) {
			t.Errorf("%s response does not contain %q: %s", tc.name, tc.want, text)
		}
	}
}

func TestMCPRejectsSourceTraversalEndToEnd(t *testing.T) {
	a := newMCPFixture(t)
	response := callMCP(t, a, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"leer_fuente","arguments":{"path":"../secret.txt"}}}`)
	if response["error"] == nil {
		t.Fatal("expected traversal request to be rejected")
	}
}

func newMCPFixture(t *testing.T) *core.App {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "materiales", "redes", "subnetting.md")
	if err := os.MkdirAll(filepath.Dir(source), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("# Subnetting\nMáscaras, CIDR, subredes y ejercicios prácticos."), 0644); err != nil {
		t.Fatal(err)
	}
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	if _, err := a.Ingest(filepath.Join(root, "materiales")); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveProfile(core.Profile{Institution: "UTN FRT", Career: "Ingeniería en Sistemas", Year: 2}); err != nil {
		t.Fatal(err)
	}
	return a
}

func callMCP(t *testing.T, a *core.App, request string) map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := Serve(a, bytes.NewBufferString(request+"\n"), &out, nil); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatalf("invalid MCP JSON response %q: %v", out.String(), err)
	}
	return response
}

func mcpText(t *testing.T, response map[string]any) string {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %#v", response)
	}
	content, ok := result["content"].([]any)
	if !ok {
		t.Fatalf("missing content: %#v", result)
	}
	var text string
	for _, item := range content {
		if block, ok := item.(map[string]any); ok {
			if value, ok := block["text"].(string); ok {
				text += value + "\n"
			}
		}
	}
	return text
}

func containsFold(value, needle string) bool {
	return bytes.Contains(bytes.ToLower([]byte(value)), bytes.ToLower([]byte(needle)))
}

func TestMCPSearchUsesProfileFilterEndToEnd(t *testing.T) {
	root := t.TempDir()
	a, err := core.New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, rel := range []string{"materiales/redes/subnetting.md", "materiales/quimica/tabla.md"} {
		full := filepath.Join(a.Root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		body := "# subnetting\nMáscaras CIDR y subredes para practicar."
		if strings.Contains(rel, "quimica") {
			body = "# tabla periódica\nElementos químicos y sus propiedades."
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Ingest(filepath.Join(a.Root, "materiales")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "profile.json"), []byte(`{"institucion":"UTN","materias_activas":["quimica"]}`), 0644); err != nil {
		t.Fatal(err)
	}

	respRaw := callMCP(t, a, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"buscar_material","arguments":{"query":"elementos"}}}`)
	resp := fmt.Sprintf("%v", respRaw["result"])
	if !strings.Contains(resp, "quimica") || !strings.Contains(resp, "tabla") {
		t.Fatalf("esperaba resultado de quimica: %s", resp)
	}
	if strings.Contains(resp, "subnetting") {
		t.Fatalf("el filtro por perfil no se aplicó: %s", resp)
	}
}
