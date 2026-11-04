package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FranprzDev/apuntes-cli/internal/core"
)

// readOnlyTools are the tools exposed over HTTP by default: they only read
// the local index and materiales/, never write local state (sessions,
// progress, summaries or the profile).
var readOnlyTools = map[string]bool{
	"listar_materias":         true,
	"buscar_material":         true,
	"leer_fuente":             true,
	"sugerir_ruta_de_estudio": true,
	"buscar_ejercicios":       true,
}

// HTTPOptions controls the optional streamable-HTTP transport.
type HTTPOptions struct {
	// AllTools exposes every tool, including the ones that write local
	// state. Only reasonable for a private, single-user deployment.
	AllTools bool
	// Token, when set, requires clients to send "Authorization: Bearer <token>".
	Token string
	// AllowedOrigins lists extra browser origins (besides the server's own
	// host) that may call the endpoint. Comes from
	// APUNTES_MCP_ALLOWED_ORIGINS (comma-separated), never from the CLI.
	AllowedOrigins []string
}

func (o HTTPOptions) allows(name string) bool {
	return o.AllTools || readOnlyTools[name]
}

// ServeHTTP serves the MCP server over streamable HTTP on addr
// (e.g. "127.0.0.1:8080" or ":8080"). Each POST carries one JSON-RPC
// message (or a batch) and gets a JSON response; notifications get 202.
// Other methods return 405: no SSE streams and no sessions.
func ServeHTTP(a *core.App, addr string, opts HTTPOptions) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           HTTPHandler(a, opts),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}

// HTTPHandler exposes the MCP server as an http.Handler so it can be mounted
// behind a reverse proxy or exercised with httptest.
func HTTPHandler(a *core.App, opts HTTPOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !originAllowed(origin, r.Host, opts.AllowedOrigins) {
			// MCP servers must validate Origin to stop DNS-rebinding: a
			// browser on an unrelated site must not reach the server.
			http.Error(w, "origen no permitido", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, MCP-Protocol-Version")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Session-Id, MCP-Protocol-Version, Last-Event-ID")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		if opts.Token != "" && r.Header.Get("Authorization") != "Bearer "+opts.Token {
			writeHTTPJSON(w, http.StatusUnauthorized, response{"2.0", nil, nil, map[string]any{"code": -32001, "message": "token inválido o ausente"}})
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "método no soportado: este endpoint solo acepta POST JSON-RPC", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeHTTPJSON(w, http.StatusRequestEntityTooLarge, response{"2.0", nil, nil, map[string]any{"code": -32700, "message": "no se pudo leer el cuerpo"}})
			return
		}
		body = bytes.TrimSpace(body)
		if len(body) == 0 {
			writeHTTPJSON(w, http.StatusBadRequest, response{"2.0", nil, nil, map[string]any{"code": -32700, "message": "cuerpo vacío"}})
			return
		}
		if body[0] == '[' {
			var reqs []rpc
			if err := json.Unmarshal(body, &reqs); err != nil || len(reqs) == 0 {
				writeHTTPJSON(w, http.StatusBadRequest, response{"2.0", nil, nil, map[string]any{"code": -32700, "message": "JSON-RPC inválido"}})
				return
			}
			out := make([]response, 0, len(reqs))
			for _, req := range reqs {
				if res, ok := handleHTTPRPC(a, opts, req); ok {
					out = append(out, res)
				}
			}
			if len(out) == 0 {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			writeHTTPJSON(w, http.StatusOK, out)
			return
		}
		var req rpc
		if err := json.Unmarshal(body, &req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest, response{"2.0", nil, nil, map[string]any{"code": -32700, "message": "JSON-RPC inválido"}})
			return
		}
		if res, ok := handleHTTPRPC(a, opts, req); ok {
			writeHTTPJSON(w, http.StatusOK, res)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
}

// handleHTTPRPC runs one request over HTTP. ok is false for notifications,
// which get no response body (202 Accepted).
func handleHTTPRPC(a *core.App, opts HTTPOptions, req rpc) (response, bool) {
	if strings.HasPrefix(req.Method, "notifications/") {
		handleRPC(a, req, opts.allows)
		return response{}, false
	}
	if req.ID == nil {
		return response{"2.0", nil, nil, map[string]any{"code": -32600, "message": "falta id"}}, true
	}
	return handleRPC(a, req, opts.allows), true
}

// originAllowed reports whether a browser Origin may talk to the server:
// same-host origins always, plus anything listed in
// APUNTES_MCP_ALLOWED_ORIGINS. Requests without an Origin header are not
// browser same-origin-policy traffic (curl, MCP clients) and skip this.
func originAllowed(origin, host string, extra []string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, host) {
		return true
	}
	for _, allowed := range extra {
		if strings.EqualFold(strings.TrimSpace(allowed), u.Host) || strings.EqualFold(strings.TrimSpace(allowed), origin) {
			return true
		}
	}
	return false
}

func writeHTTPJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
