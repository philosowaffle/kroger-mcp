package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
	"github.com/philosowaffle/kroger-cart/internal/kroger/krogertest"
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
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader("{}")))
	return rec.Code, rec.Header()
}

func TestRoutesWithAuth(t *testing.T) {
	issuer := fakeIssuer(t)
	t.Setenv("KROGER_MCP_OIDC_ISSUER", issuer)
	t.Setenv("KROGER_MCP_OIDC_RESOURCE", "https://kroger-mcp.example.com")
	t.Setenv("KROGER_MCP_OIDC_SCOPES", "")

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
	t.Setenv("KROGER_MCP_OIDC_ISSUER", "")

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
	t.Setenv("KROGER_MCP_OIDC_ISSUER", "http://127.0.0.1:1")
	t.Setenv("KROGER_MCP_OIDC_RESOURCE", "")
	if _, err := newHandler(context.Background(), newKrogerClient(t)); err == nil {
		t.Fatal("want error when resource is missing")
	}
}
