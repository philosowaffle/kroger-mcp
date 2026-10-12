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
	want := Config{Issuer: testIssuer, Resource: testResource, Scopes: []string{"mcp", "profile"}, RequireATJWT: true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestConfigFromEnv_RequireATJWT(t *testing.T) {
	//nolint:gocritic // mapKey: " FALSE " checks that surrounding whitespace is trimmed.
	for v, want := range map[string]bool{"": true, "true": true, "yes": true, "false": false, " FALSE ": false, "0": false, "no": false, "off": false} {
		t.Setenv("KROGER_MCP_OIDC_REQUIRE_AT_JWT", v)
		if got := ConfigFromEnv().RequireATJWT; got != want {
			t.Errorf("KROGER_MCP_OIDC_REQUIRE_AT_JWT=%q: RequireATJWT = %v, want %v", v, got, want)
		}
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

// signJWT builds an RS256 access token (typ at+jwt) over claims.
func signJWT(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	return signJWTHeader(t, key, map[string]string{"alg": "RS256", "typ": "at+jwt"}, claims)
}

// signJWTHeader builds an RS256 JWT with an explicit JOSE header.
func signJWTHeader(t *testing.T, key *rsa.PrivateKey, header map[string]string, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	signingInput := enc(header) + "." + enc(claims)
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// newTestMiddleware wraps an "ok" handler in the middleware, trusting key.
func newTestMiddleware(t *testing.T, key *rsa.PrivateKey, cfg Config) http.Handler {
	t.Helper()
	verifier := oidc.NewVerifier(testIssuer,
		&oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}},
		&oidc.Config{ClientID: testResource, SupportedSigningAlgs: []string{oidc.RS256}})
	return newAuthenticator(cfg, verifier).Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
}

func serveToken(h http.Handler, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func testClaims(mod func(map[string]any)) map[string]any {
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

func TestMiddleware(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	h := newTestMiddleware(t, key, Config{Issuer: testIssuer, Resource: testResource, Scopes: []string{"mcp"}, RequireATJWT: true})

	tests := []struct {
		name   string
		token  string
		status int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"valid", signJWT(t, key, testClaims(nil)), http.StatusOK},
		{"audience in list", signJWT(t, key, testClaims(func(c map[string]any) { c["aud"] = []string{"x", testResource} })), http.StatusOK},
		{"wrong audience", signJWT(t, key, testClaims(func(c map[string]any) { c["aud"] = "https://other.example.com" })), http.StatusUnauthorized},
		{"wrong issuer", signJWT(t, key, testClaims(func(c map[string]any) { c["iss"] = "https://evil.example.com" })), http.StatusUnauthorized},
		{"expired", signJWT(t, key, testClaims(func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() })), http.StatusUnauthorized},
		{"wrong key", signJWT(t, otherKey, testClaims(nil)), http.StatusUnauthorized},
		{"opaque token", "not-a-jwt", http.StatusUnauthorized},
		{"missing scope", signJWT(t, key, testClaims(func(c map[string]any) { c["scope"] = "other" })), http.StatusForbidden},
		{"scp string", signJWT(t, key, testClaims(func(c map[string]any) { delete(c, "scope"); c["scp"] = "other mcp" })), http.StatusOK},
		{"scp array", signJWT(t, key, testClaims(func(c map[string]any) { delete(c, "scope"); c["scp"] = []string{"other", "mcp"} })), http.StatusOK},
		{"scp missing scope", signJWT(t, key, testClaims(func(c map[string]any) { delete(c, "scope"); c["scp"] = []string{"other"} })), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveToken(h, tt.token)
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

func TestMiddleware_AccessTokenType(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Issuer: testIssuer, Resource: testResource, RequireATJWT: true}
	strict := newTestMiddleware(t, key, cfg)
	cfg.RequireATJWT = false
	lenient := newTestMiddleware(t, key, cfg)

	sign := func(typ string) string {
		header := map[string]string{"alg": "RS256"}
		if typ != "" {
			header["typ"] = typ
		}
		return signJWTHeader(t, key, header, testClaims(nil))
	}

	tests := []struct {
		name    string
		token   string
		strict  int
		lenient int
	}{
		{"at+jwt", sign("at+jwt"), http.StatusOK, http.StatusOK},
		{"application/at+jwt", sign("application/at+jwt"), http.StatusOK, http.StatusOK},
		{"case-insensitive", sign("AT+JWT"), http.StatusOK, http.StatusOK},
		// An ID token for the same audience: typ JWT.
		{"ID token", sign("JWT"), http.StatusUnauthorized, http.StatusOK},
		{"no typ", sign(""), http.StatusUnauthorized, http.StatusOK},
		{"malformed header", "%%%.e30.sig", http.StatusUnauthorized, http.StatusUnauthorized},
		{"no dots", "not-a-jwt", http.StatusUnauthorized, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serveToken(strict, tt.token).Code; got != tt.strict {
				t.Errorf("RequireATJWT on: status = %d, want %d", got, tt.strict)
			}
			if got := serveToken(lenient, tt.token).Code; got != tt.lenient {
				t.Errorf("RequireATJWT off: status = %d, want %d", got, tt.lenient)
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
