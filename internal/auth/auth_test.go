package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

const (
	testIssuer   = "https://auth.example.com"
	testResource = "https://kroger-mcp.example.com"
)

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("KROGER_MCP_OIDC_ISSUER", " https://auth.example.com ")
	t.Setenv("KROGER_MCP_OIDC_RESOURCE", "https://kroger-mcp.example.com")
	t.Setenv("KROGER_MCP_OIDC_SCOPES", "  mcp  profile ")

	got := ConfigFromEnv()
	want := Config{Issuer: testIssuer, Resource: testResource, Scopes: []string{"mcp", "profile"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestEnabled(t *testing.T) {
	t.Setenv("KROGER_MCP_OIDC_ISSUER", "")
	if Enabled() {
		t.Error("enabled with no issuer")
	}
	t.Setenv("KROGER_MCP_OIDC_ISSUER", testIssuer)
	if !Enabled() {
		t.Error("disabled with issuer set")
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (Config{Resource: testResource}).validate(); err == nil || !strings.Contains(err.Error(), "ISSUER") {
		t.Errorf("missing issuer: err = %v", err)
	}
	if err := (Config{Issuer: testIssuer}).validate(); err == nil || !strings.Contains(err.Error(), "RESOURCE") {
		t.Errorf("missing resource: err = %v", err)
	}
	if err := (Config{Issuer: testIssuer, Resource: testResource}).validate(); err != nil {
		t.Errorf("valid config: err = %v", err)
	}
}

func TestMetadataURL(t *testing.T) {
	for _, res := range []string{testResource, testResource + "/"} {
		a := &Authenticator{cfg: Config{Resource: res}}
		if got, want := a.metadataURL(), testResource+MetadataPath; got != want {
			t.Errorf("metadataURL(%q) = %q, want %q", res, got, want)
		}
	}
}

// signJWT builds an RS256 JWT over claims.
func signJWT(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	signingInput := enc(map[string]string{"alg": "RS256", "typ": "at+jwt"}) + "." + enc(claims)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestMiddleware(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	cfg := Config{Issuer: testIssuer, Resource: testResource, Scopes: []string{"mcp"}}
	verifier := oidc.NewVerifier(testIssuer,
		&oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}},
		&oidc.Config{ClientID: testResource, SupportedSigningAlgs: []string{oidc.RS256}})
	a := newAuthenticator(cfg, verifier)

	h := a.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	claims := func(mod func(map[string]any)) map[string]any {
		c := map[string]any{
			"iss":   testIssuer,
			"aud":   testResource,
			"sub":   "bailey",
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"scope": "mcp other",
		}
		if mod != nil {
			mod(c)
		}
		return c
	}

	tests := []struct {
		name   string
		token  string
		status int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"valid", signJWT(t, key, claims(nil)), http.StatusOK},
		{"audience in list", signJWT(t, key, claims(func(c map[string]any) { c["aud"] = []string{"x", testResource} })), http.StatusOK},
		{"wrong audience", signJWT(t, key, claims(func(c map[string]any) { c["aud"] = "https://other.example.com" })), http.StatusUnauthorized},
		{"wrong issuer", signJWT(t, key, claims(func(c map[string]any) { c["iss"] = "https://evil.example.com" })), http.StatusUnauthorized},
		{"expired", signJWT(t, key, claims(func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() })), http.StatusUnauthorized},
		{"wrong key", signJWT(t, otherKey, claims(nil)), http.StatusUnauthorized},
		{"opaque token", "not-a-jwt", http.StatusUnauthorized},
		{"missing scope", signJWT(t, key, claims(func(c map[string]any) { c["scope"] = "other" })), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.status, rec.Body)
			}
			if tt.status == http.StatusUnauthorized {
				if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, testResource+MetadataPath) {
					t.Errorf("WWW-Authenticate = %q, want resource_metadata URL", got)
				}
			}
		})
	}
}

func TestMetadataHandler(t *testing.T) {
	a := newAuthenticator(Config{Issuer: testIssuer, Resource: testResource, Scopes: []string{"mcp"}}, nil)
	rec := httptest.NewRecorder()
	a.MetadataHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, MetadataPath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var md struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &md); err != nil {
		t.Fatal(err)
	}
	if md.Resource != testResource || !reflect.DeepEqual(md.AuthorizationServers, []string{testIssuer}) || !reflect.DeepEqual(md.ScopesSupported, []string{"mcp"}) {
		t.Errorf("metadata = %+v", md)
	}
}
