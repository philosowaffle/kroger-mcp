// Package auth makes kroger-mcp an OAuth 2.0 protected resource (the MCP
// authorization spec, RFC 9728). It runs no OAuth flows of its own: an
// external authorization server such as Authelia handles login, consent and
// token issuance. This package only
//
//   - serves protected-resource metadata naming that authorization server,
//   - answers unauthenticated requests with 401 and a WWW-Authenticate
//     header pointing at the metadata, and
//   - validates the bearer JWT access token (RFC 9068) on every request:
//     signature against the issuer's JWKS, plus iss, aud and exp.
//
// Auth is opt-in. With no issuer configured the server is unauthenticated,
// which suits LAN or VPN-only deployments.
//
// This guards access to the MCP endpoint itself. Access to the Kroger API is
// a separate concern handled by package kroger.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// MetadataPath is the well-known path of the protected-resource metadata
// document.
const MetadataPath = "/.well-known/oauth-protected-resource"

// Config is the resource-server configuration, read from the environment.
type Config struct {
	// Issuer is the authorization server's issuer URL, used for OIDC discovery.
	Issuer string
	// Resource identifies this server. It is advertised in the metadata and
	// must appear in the access token's aud claim.
	Resource string
	// Scopes, if set, are required on every request and advertised as
	// supported.
	Scopes []string
}

// ConfigFromEnv reads the KROGER_MCP_OIDC_* variables.
func ConfigFromEnv() Config {
	return Config{
		Issuer:   strings.TrimSpace(os.Getenv("KROGER_MCP_OIDC_ISSUER")),
		Resource: strings.TrimSpace(os.Getenv("KROGER_MCP_OIDC_RESOURCE")),
		Scopes:   strings.Fields(os.Getenv("KROGER_MCP_OIDC_SCOPES")),
	}
}

// Enabled reports whether an issuer is configured, which turns auth on.
func Enabled() bool {
	return ConfigFromEnv().Issuer != ""
}

// Authenticator validates bearer tokens and serves the metadata document.
type Authenticator struct {
	cfg      Config
	verifier *oidc.IDTokenVerifier
	metadata *oauthex.ProtectedResourceMetadata
}

// New builds an Authenticator from the environment. It runs OIDC discovery,
// so the issuer must be reachable at startup.
func New(ctx context.Context) (*Authenticator, error) {
	cfg := ConfigFromEnv()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: oidc discovery for %q: %w", cfg.Issuer, err)
	}

	// go-oidc checks that aud contains ClientID, so passing our resource id
	// enforces the audience requirement.
	verifier := provider.Verifier(&oidc.Config{
		ClientID:             cfg.Resource,
		SupportedSigningAlgs: []string{oidc.RS256},
	})
	return newAuthenticator(cfg, verifier), nil
}

func newAuthenticator(cfg Config, verifier *oidc.IDTokenVerifier) *Authenticator {
	return &Authenticator{
		cfg:      cfg,
		verifier: verifier,
		metadata: &oauthex.ProtectedResourceMetadata{
			Resource:               cfg.Resource,
			AuthorizationServers:   []string{cfg.Issuer},
			ScopesSupported:        cfg.Scopes,
			BearerMethodsSupported: []string{"header"},
			ResourceName:           "kroger-mcp",
		},
	}
}

func (c Config) validate() error {
	if c.Issuer == "" {
		return errors.New("auth: KROGER_MCP_OIDC_ISSUER is required")
	}
	if c.Resource == "" {
		return errors.New("auth: KROGER_MCP_OIDC_RESOURCE is required")
	}
	return nil
}

// MetadataHandler serves the protected-resource metadata document.
func (a *Authenticator) MetadataHandler() http.Handler {
	return mcpauth.ProtectedResourceMetadataHandler(a.metadata)
}

// Middleware enforces a valid bearer token on the wrapped handler.
func (a *Authenticator) Middleware() func(http.Handler) http.Handler {
	return mcpauth.RequireBearerToken(a.verify, &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: a.metadataURL(),
		Scopes:              a.cfg.Scopes,
	})
}

// Issuer returns the configured authorization server.
func (a *Authenticator) Issuer() string { return a.cfg.Issuer }

// Metadata returns the advertised protected-resource metadata.
func (a *Authenticator) Metadata() *oauthex.ProtectedResourceMetadata { return a.metadata }

func (a *Authenticator) metadataURL() string {
	return strings.TrimRight(a.cfg.Resource, "/") + MetadataPath
}

func (a *Authenticator) verify(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	idt, err := a.verifier.Verify(ctx, token)
	if err != nil {
		// The client only sees a bare 401, so log the real reason. Typical
		// causes: aud does not contain the resource, an opaque token instead
		// of a JWT ("malformed jwt"), or an issuer mismatch.
		slog.Warn("auth: token rejected", "err", err)
		return nil, fmt.Errorf("%w: %v", mcpauth.ErrInvalidToken, err)
	}

	var claims struct {
		Scope string `json:"scope"`
	}
	_ = idt.Claims(&claims)

	return &mcpauth.TokenInfo{
		Scopes:     strings.Fields(claims.Scope),
		Expiration: idt.Expiry,
		UserID:     idt.Subject,
	}, nil
}
