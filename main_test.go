package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/philosowaffle/kroger-mcp/internal/kroger"
	"github.com/philosowaffle/kroger-mcp/internal/kroger/krogertest"
	"github.com/philosowaffle/kroger-mcp/internal/server"
)

var (
	issuerEnv   = server.EnvPrefix + "_OIDC_ISSUER"
	resourceEnv = server.EnvPrefix + "_OIDC_RESOURCE"
	scopesEnv   = server.EnvPrefix + "_OIDC_SCOPES"
)

func newKrogerClient(t *testing.T) *kroger.Client {
	t.Helper()
	fake := krogertest.New(t)
	kc, err := kroger.New(kroger.Config{
		ClientID:     krogertest.ClientID,
		ClientSecret: krogertest.ClientSecret,
		RedirectURI:  "https://kroger-mcp.example.com/kroger/callback",
		DataDir:      t.TempDir(),
		BaseURL:      fake.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	return kc
}

// fakeIssuer serves just enough OIDC discovery for auth.New to succeed.
func fakeIssuer(t *testing.T) string {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func status(t *testing.T, h http.Handler, method, target string) (int, http.Header) {
	t.Helper()
	return statusWith(t, h, method, target, nil)
}

func statusWith(t *testing.T, h http.Handler, method, target string, header map[string]string) (int, http.Header) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader("{}"))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Header()
}

// TestCrossOriginRejected covers browser requests from another site, which
// must not reach the MCP endpoint whether or not auth is on.
func TestCrossOriginRejected(t *testing.T) {
	for _, authOn := range []bool{false, true} {
		if authOn {
			t.Setenv(issuerEnv, fakeIssuer(t))
			t.Setenv(resourceEnv, "https://kroger-mcp.example.com")
		} else {
			t.Setenv(issuerEnv, "")
		}
		h, err := newHandler(context.Background(), newKrogerClient(t))
		if err != nil {
			t.Fatal(err)
		}

		tests := []struct {
			name    string
			header  map[string]string
			blocked bool
		}{
			{"no browser headers", nil, false},
			{"same-origin fetch", map[string]string{"Sec-Fetch-Site": "same-origin"}, false},
			{"cross-site fetch", map[string]string{"Sec-Fetch-Site": "cross-site"}, true},
			{"foreign Origin", map[string]string{"Origin": "https://evil.example.com"}, true},
		}
		for _, tt := range tests {
			got, _ := statusWith(t, h, http.MethodPost, "/", tt.header)
			if blocked := got == http.StatusForbidden; blocked != tt.blocked {
				t.Errorf("auth=%v %s: status = %d, blocked = %v, want %v", authOn, tt.name, got, blocked, tt.blocked)
			}
		}
	}
}

func TestRoutesWithAuth(t *testing.T) {
	issuer := fakeIssuer(t)
	t.Setenv(issuerEnv, issuer)
	t.Setenv(resourceEnv, "https://kroger-mcp.example.com")
	t.Setenv(scopesEnv, "")

	h, err := newHandler(context.Background(), newKrogerClient(t))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		method string
		target string
		want   int
	}{
		{"health is open", http.MethodGet, "/healthz", http.StatusOK},
		{"metadata is open", http.MethodGet, "/.well-known/oauth-protected-resource", http.StatusOK},
		{"other well-known is 404 not 401", http.MethodGet, "/.well-known/oauth-authorization-server", http.StatusNotFound},
		// The callback must not require a bearer token (a browser redirect
		// can't send one); a forged state is rejected by the handler itself.
		{"kroger callback is open", http.MethodGet, "/kroger/callback?state=forged&code=x", http.StatusBadRequest},
		{"mcp requires token", http.MethodPost, "/", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hdr := status(t, h, tt.method, tt.target)
			if got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
			if tt.want == http.StatusUnauthorized && !strings.Contains(hdr.Get("WWW-Authenticate"), "resource_metadata") {
				t.Errorf("WWW-Authenticate = %q", hdr.Get("WWW-Authenticate"))
			}
		})
	}
}

func TestRoutesWithoutAuth(t *testing.T) {
	t.Setenv(issuerEnv, "")

	h, err := newHandler(context.Background(), newKrogerClient(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := status(t, h, http.MethodGet, "/.well-known/oauth-protected-resource"); got != http.StatusNotFound {
		t.Errorf("metadata without auth: status = %d, want 404", got)
	}
	if got, _ := status(t, h, http.MethodPost, "/"); got == http.StatusUnauthorized {
		t.Error("MCP endpoint demanded auth with auth disabled")
	}
}

func TestAuthMisconfigured(t *testing.T) {
	t.Setenv(issuerEnv, "http://127.0.0.1:1")
	t.Setenv(resourceEnv, "")
	if _, err := newHandler(context.Background(), newKrogerClient(t)); err == nil {
		t.Fatal("want error when resource is missing")
	}
}

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8080", true},
		{"127.1.2.3:8080", true},
		{"[::1]:8080", true},
		{"localhost:8080", true},
		{"LOCALHOST:8080", true},
		{":8080", false},
		{"0.0.0.0:8080", false},
		{"[::]:8080", false},
		{"192.168.1.10:8080", false},
		{"kroger.example.com:8080", false},
		{"not-an-addr", false},
	}
	for _, tt := range tests {
		if got := isLoopback(tt.addr); got != tt.want {
			t.Errorf("isLoopback(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

// TestCheckExposure_AuthOffNonLoopback covers #25: with auth off, the server
// only starts beyond loopback when the operator opts in.
func TestCheckExposure_AuthOffNonLoopback(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		authOn  bool
		allow   string
		wantErr bool
	}{
		{"auth off, all interfaces", ":8080", false, "", true},
		{"auth off, LAN address", "192.168.1.10:8080", false, "", true},
		{"auth off, opt-in false", ":8080", false, "false", true},
		{"auth off, opted in", ":8080", false, "true", false},
		{"auth off, loopback", "127.0.0.1:8080", false, "", false},
		{"auth on, all interfaces", ":8080", true, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(allowUnauthEnv, tt.allow)
			err := checkExposure(tt.addr, tt.authOn)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkExposure(%q, %v) = %v, wantErr %v", tt.addr, tt.authOn, err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), allowUnauthEnv) {
				t.Errorf("error %q doesn't name the opt-in", err)
			}
		})
	}
}

// TestRun_RefusesUnauthenticatedExposure checks the guard runs at start-up,
// before anything listens.
func TestRun_RefusesUnauthenticatedExposure(t *testing.T) {
	t.Setenv(issuerEnv, "")
	t.Setenv(allowUnauthEnv, "")
	err := run(":0")
	if err == nil || !strings.Contains(err.Error(), "refusing to listen") {
		t.Fatalf("run(:0) = %v, want exposure refusal", err)
	}
}

// TestToolCallWithProtocolVersionMeta covers #48: Claude's client sends
// _meta["io.modelcontextprotocol/protocolVersion"] on calls under the
// 2025-11-25 protocol. go-sdk rejects such calls on a stateful handler with
// "protocol version \"2025-11-25\" is not supported by this server".
func TestToolCallWithProtocolVersionMeta(t *testing.T) {
	t.Setenv(issuerEnv, "")
	h, err := newHandler(context.Background(), newKrogerClient(t))
	if err != nil {
		t.Fatal(err)
	}
	post := func(body, sessionID string) (*httptest.ResponseRecorder, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Protocol-Version", "2025-11-25")
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec, rec.Body.String()
	}
	rec, _ := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, "")
	sid := rec.Header().Get("Mcp-Session-Id")
	post(`{"jsonrpc":"2.0","method":"notifications/initialized"}`, sid)
	rec, body := post(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2025-11-25"}}}`, sid)
	if rec.Code != http.StatusOK || strings.Contains(body, `"error"`) || !strings.Contains(body, `"tools"`) {
		t.Fatalf("tools/list with protocolVersion _meta: status %d, body %s", rec.Code, body)
	}
}
