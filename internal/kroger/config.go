package kroger

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultBaseURL is the Kroger Public API root.
	DefaultBaseURL = "https://api.kroger.com/v1"
	// DefaultRedirectURI matches a server running locally on the default port.
	DefaultRedirectURI = "http://localhost:8080/kroger/callback"
	// DefaultDataDir is where state is persisted when KROGER_MCP_DATA_DIR is unset.
	DefaultDataDir = "data"

	// appScope is requested with the client-credentials grant. Product and
	// location lookups need only an app token.
	appScope = "product.compact"
)

// userScopes are requested when the shopper links their Kroger account.
var userScopes = []string{"cart.basic:write", "profile.compact", "product.compact"}

// Config holds the Kroger developer app credentials and server-side settings.
type Config struct {
	ClientID     string
	ClientSecret string
	// RedirectURI must exactly match one registered on the Kroger developer
	// app. Its path is where the OAuth callback handler is mounted.
	RedirectURI string
	// DefaultLocationID is used when neither the tool call nor the saved
	// preference names a store.
	DefaultLocationID string
	// DefaultModality is PICKUP or DELIVERY, used for cart items that don't
	// name one.
	DefaultModality string
	// DataDir holds state.json (linked-account tokens and preferences).
	DataDir string
	// BaseURL overrides the API root; tests point it at a fake server.
	BaseURL string
}

// ConfigFromEnv reads KROGER_* and KROGER_MCP_DATA_DIR, applying defaults.
func ConfigFromEnv() Config {
	return Config{
		ClientID:          strings.TrimSpace(os.Getenv("KROGER_CLIENT_ID")),
		ClientSecret:      strings.TrimSpace(os.Getenv("KROGER_CLIENT_SECRET")),
		RedirectURI:       envOr("KROGER_REDIRECT_URI", DefaultRedirectURI),
		DefaultLocationID: strings.TrimSpace(os.Getenv("KROGER_LOCATION_ID")),
		DefaultModality:   strings.ToUpper(envOr("KROGER_MODALITY", ModalityPickup)),
		DataDir:           envOr("KROGER_MCP_DATA_DIR", DefaultDataDir),
		BaseURL:           DefaultBaseURL,
	}
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// Validate reports missing or malformed settings.
func (c Config) Validate() error {
	var errs []error
	if c.ClientID == "" {
		errs = append(errs, errors.New("KROGER_CLIENT_ID is required"))
	}
	if c.ClientSecret == "" {
		errs = append(errs, errors.New("KROGER_CLIENT_SECRET is required"))
	}
	if u, err := url.Parse(c.RedirectURI); err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, fmt.Errorf("KROGER_REDIRECT_URI %q must be an absolute URL", c.RedirectURI))
	}
	if m := c.DefaultModality; m != "" && m != ModalityPickup && m != ModalityDelivery {
		errs = append(errs, fmt.Errorf("KROGER_MODALITY must be PICKUP or DELIVERY, got %q", m))
	}
	if c.DataDir == "" {
		errs = append(errs, errors.New("KROGER_MCP_DATA_DIR must not be empty"))
	}
	return errors.Join(errs...)
}

// CallbackPath is the HTTP path of the OAuth redirect URI.
func (c Config) CallbackPath() string {
	u, err := url.Parse(c.RedirectURI)
	if err != nil || u.Path == "" {
		return "/"
	}
	return u.Path
}

func (c Config) statePath() string {
	return filepath.Join(c.DataDir, "state.json")
}
