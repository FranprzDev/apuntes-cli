package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postRPC(t *testing.T, url, token, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var decoded map[string]any
	_ = json.NewDecoder(res.Body).Decode(&decoded)
	return res.StatusCode, decoded
}

func TestMCPHTTPEndToEnd(t *testing.T) {
	a := newMCPFixture(t)
	srv := httptest.NewServer(HTTPHandler(a, HTTPOptions{}))
	defer srv.Close()

	status, initialize := postRPC(t, srv.URL, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	if status != http.StatusOK {
		t.Fatalf("initialize status: %d", status)
	}
	result := initialize["result"].(map[string]any)
	if result["protocolVersion"] != "2025-06-18" {
		t.Fatalf("expected the server to echo the client protocol version, got %#v", result)
	}

	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("notification should get 202, got %d", res.StatusCode)
	}

	_, list := postRPC(t, srv.URL, "", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	tools := list["result"].(map[string]any)["tools"].([]any)
	if len(tools) != len(readOnlyTools) {
		t.Fatalf("HTTP should list only the %d read-only tools, got %#v", len(readOnlyTools), tools)
	}
	for _, tool := range tools {
		name := tool.(map[string]any)["name"].(string)
		if !readOnlyTools[name] {
			t.Fatalf("HTTP listed a writing tool by default: %s", name)
		}
	}

	_, search := postRPC(t, srv.URL, "", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"buscar_material","arguments":{"query":"subnetting","subject":"redes"}}}`)
	if search["error"] != nil {
		t.Fatalf("buscar_material failed: %#v", search["error"])
	}
	if !containsFold(mcpText(t, search), "subnetting") {
		t.Fatalf("buscar_material should find the fixture note: %#v", search)
	}

	_, blocked := postRPC(t, srv.URL, "", `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"iniciar_clase","arguments":{"tema":"X"}}}`)
	if blocked["error"] == nil {
		t.Fatal("a writing tool should be rejected over read-only HTTP")
	}

	res2, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	res2.Body.Close()
	if res2.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET should be 405, got %d", res2.StatusCode)
	}
}

func TestMCPHTTPAllToolsAndToken(t *testing.T) {
	a := newMCPFixture(t)
	srv := httptest.NewServer(HTTPHandler(a, HTTPOptions{AllTools: true, Token: "secreto"}))
	defer srv.Close()

	status, denied := postRPC(t, srv.URL, "otro", `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	if status != http.StatusUnauthorized || denied["error"] == nil {
		t.Fatalf("wrong token should get 401 with an error, got %d %#v", status, denied)
	}

	_, list := postRPC(t, srv.URL, "secreto", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	tools := list["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 12 {
		t.Fatalf("--all-tools should list every tool, got %d", len(tools))
	}
}

func TestMCPHTTPBatch(t *testing.T) {
	a := newMCPFixture(t)
	srv := httptest.NewServer(HTTPHandler(a, HTTPOptions{}))
	defer srv.Close()

	batch := fmt.Sprintf(`[%s,%s,%s]`,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(batch))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("batch should get 200, got %d", res.StatusCode)
	}
	var out []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("the notification must not produce a response entry: %#v", out)
	}
}

func TestMCPHTTPOriginAndCORS(t *testing.T) {
	a := newMCPFixture(t)
	srv := httptest.NewServer(HTTPHandler(a, HTTPOptions{AllowedOrigins: []string{"https://inspector.example"}}))
	defer srv.Close()

	// A browser on an unrelated origin is rejected (DNS-rebinding guard).
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Origin", "https://evil.example")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin should get 403, got %d", res.StatusCode)
	}

	// A whitelisted browser origin gets CORS headers and its request through.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req2.Header.Set("Origin", "https://inspector.example")
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	res2.Body.Close()
	if res2.StatusCode != http.StatusOK || res2.Header.Get("Access-Control-Allow-Origin") != "https://inspector.example" {
		t.Fatalf("whitelisted origin should pass with CORS headers, got %d %q", res2.StatusCode, res2.Header.Get("Access-Control-Allow-Origin"))
	}

	// Preflight from the whitelisted origin answers 204 with allow headers.
	req3, _ := http.NewRequest(http.MethodOptions, srv.URL, nil)
	req3.Header.Set("Origin", "https://inspector.example")
	req3.Header.Set("Access-Control-Request-Method", "POST")
	res3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	res3.Body.Close()
	if res3.StatusCode != http.StatusNoContent || !strings.Contains(res3.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("preflight should be 204 with allow headers, got %d %#v", res3.StatusCode, res3.Header)
	}

	// Same-host origin is allowed without any whitelist.
	host := strings.TrimPrefix(srv.URL, "http://")
	req4, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req4.Header.Set("Origin", "http://"+host)
	res4, err := http.DefaultClient.Do(req4)
	if err != nil {
		t.Fatal(err)
	}
	res4.Body.Close()
	if res4.StatusCode != http.StatusOK {
		t.Fatalf("same-host origin should pass, got %d", res4.StatusCode)
	}
}
