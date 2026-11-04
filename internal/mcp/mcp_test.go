package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FranprzDev/apuntes-cli/internal/core"
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

func TestInstallConfigIncludesRoot(t *testing.T) {
	a := newMCPFixture(t)
	var out bytes.Buffer
	if err := Serve(a, bytes.NewBufferString(""), &out, []string{"install", "--agent", "claude"}); err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(out.Bytes(), &cfg); err != nil {
		t.Fatalf("invalid install JSON %q: %v", out.String(), err)
	}
	srv, ok := cfg.MCPServers["apuntes"]
	if !ok {
		t.Fatalf("missing apuntes server: %s", out.String())
	}
	want := []string{"--root", a.Root, "mcp"}
	if strings.Join(srv.Args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %v, want %v", srv.Args, want)
	}
	if !filepath.IsAbs(srv.Command) {
		t.Fatalf("command should be absolute: %q", srv.Command)
	}
}

func TestInstallCodexEmitsTOML(t *testing.T) {
	a := newMCPFixture(t)
	var out bytes.Buffer
	if err := Serve(a, bytes.NewBufferString(""), &out, []string{"install", "--agent", "codex"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "mcpServers") || strings.Contains(got, "{") {
		t.Fatalf("codex output should be TOML, not JSON: %s", got)
	}
	for _, want := range []string{
		"[mcp_servers.apuntes]",
		`command = ` + tomlQuote(filepath.Join(a.Root, "apuntes")),
		`args = ["--root", ` + tomlQuote(a.Root) + `, "mcp"]`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("codex TOML missing %q:\n%s", want, got)
		}
	}
}

func TestTomlQuoteUnusualCharacters(t *testing.T) {
	// strconv.Quote would emit Go-only escapes (\x01, \a, \v, \U...) that
	// TOML rejects, so paths with control characters must use \u escapes.
	cases := map[string]string{
		"plain/path":      `"plain/path"`,
		"a\tb":            `"a\tb"`,
		"a\x01b":          `"a\u0001b"`,
		"a\\b":            `"a\\b"`,
		"a\"b":            `"a\"b"`,
		"acentuado/ñandú": `"acentuado/ñandú"`,
	}
	for in, want := range cases {
		if got := tomlQuote(in); got != want {
			t.Fatalf("tomlQuote(%q) = %s, want %s", in, got, want)
		}
	}
	for _, s := range []string{"a\x01b", "a\x07b", "a\x0bb"} {
		got := tomlQuote(s)
		if strings.Contains(got, `\x`) || strings.Contains(got, `\a`) || strings.Contains(got, `\v`) {
			t.Fatalf("tomlQuote(%q) contains a Go-only escape TOML rejects: %s", s, got)
		}
	}
}

func TestInstallRejectsRelativeCommand(t *testing.T) {
	a := newMCPFixture(t)
	for _, agent := range []string{"claude", "codex"} {
		err := Serve(a, bytes.NewBufferString(""), &bytes.Buffer{}, []string{"install", "--agent", agent, "--command", "apuntes"})
		if err == nil {
			t.Fatalf("%s: relative --command should be rejected", agent)
		}
	}
}

func TestInstallAcceptsAbsoluteCommand(t *testing.T) {
	a := newMCPFixture(t)
	bin := filepath.Join(a.Root, "bin", "apuntes")
	var out bytes.Buffer
	if err := Serve(a, bytes.NewBufferString(""), &out, []string{"install", "--agent", "claude", "--command", bin}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), bin) {
		t.Fatalf("output should use the custom command: %s", out.String())
	}
}

func TestInstallRequiresAgent(t *testing.T) {
	a := newMCPFixture(t)
	cases := [][]string{
		{"install"},
		{"install", "--agent"},
		{"install", "--agent", "vim"},
		{"install", "--agent", "claude", "--bogus"},
	}
	for _, args := range cases {
		if err := Serve(a, bytes.NewBufferString(""), &bytes.Buffer{}, args); err == nil {
			t.Fatalf("%v: expected error", args)
		}
	}
}
