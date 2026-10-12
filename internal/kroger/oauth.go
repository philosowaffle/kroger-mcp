package kroger

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// pendingAuthTTL bounds how long a login URL from AuthorizeURL stays usable.
	pendingAuthTTL = 15 * time.Minute
	// maxPendingAuths caps outstanding login URLs. A shopper needs one; the
	// cap stops a caller looping kroger_login from growing the map.
	maxPendingAuths = 32
	// maxCallbackParam bounds query values from the callback before they're
	// logged or rendered.
	maxCallbackParam = 200
)

// pendingAuth is an authorization request waiting for Kroger to redirect back.
type pendingAuth struct {
	verifier string
	expires  time.Time
}

// ErrUnknownState means the callback's state did not match an outstanding
// login URL: it expired, was already used, or was never issued here.
var ErrUnknownState = errors.New("unknown or expired login state; request a new link with the kroger_login tool")

// AuthorizeURL starts linking a shopper's Kroger account. It returns a Kroger
// login URL carrying a fresh single-use state and PKCE challenge. Only states
// issued here are accepted by the callback, so the callback can't be used to
// link an account unless someone first obtained a URL through this method.
func (c *Client) AuthorizeURL() (string, error) {
	st, err := randomString(32)
	if err != nil {
		return "", err
	}
	verifier, err := randomString(64)
	if err != nil {
		return "", err
	}

	now := c.now()
	c.pendingMu.Lock()
	for k, p := range c.pending {
		if now.After(p.expires) {
			delete(c.pending, k)
		}
	}
	if len(c.pending) >= maxPendingAuths {
		// All live: drop the one closest to expiry, i.e. the oldest.
		var oldest string
		for k, p := range c.pending {
			if oldest == "" || p.expires.Before(c.pending[oldest].expires) {
				oldest = k
			}
		}
		delete(c.pending, oldest)
	}
	c.pending[st] = pendingAuth{verifier: verifier, expires: now.Add(pendingAuthTTL)}
	c.pendingMu.Unlock()

	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.cfg.ClientID},
		"redirect_uri":          {c.cfg.RedirectURI},
		"scope":                 {strings.Join(userScopes, " ")},
		"state":                 {st},
		"code_challenge":        {codeChallenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	return c.cfg.BaseURL + "/connect/oauth2/authorize?" + q.Encode(), nil
}

// CompleteAuthorization exchanges the code Kroger redirected back with for
// tokens and persists them. The state is consumed whether or not the
// exchange succeeds.
func (c *Client) CompleteAuthorization(ctx context.Context, st, code string) error {
	c.pendingMu.Lock()
	p, ok := c.pending[st]
	delete(c.pending, st)
	c.pendingMu.Unlock()
	if !ok || c.now().After(p.expires) {
		return ErrUnknownState
	}
	if code == "" {
		return errors.New("callback is missing the authorization code")
	}

	t, err := c.postToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.cfg.RedirectURI},
		"code_verifier": {p.verifier},
	})
	if err != nil {
		return fmt.Errorf("exchange authorization code: %w", err)
	}
	if err := c.state.update(func(s *state) { s.UserToken = t }); err != nil {
		return err
	}
	c.recordProfile(ctx, t.AccessToken)
	return nil
}

// recordProfile stores the newly linked shopper's profile ID and logs when
// the link replaced a different account, which is how a login by someone
// else who can reach the MCP endpoint would show up. Best effort: the link
// itself already succeeded, so this uses the new token as-is rather than
// refreshing (and possibly unlinking) on failure.
func (c *Client) recordProfile(ctx context.Context, tok string) {
	prev := c.state.get().ProfileID
	p, err := c.profileWith(ctx, tok)
	if err != nil {
		slog.Warn("kroger: linked, but couldn't read the profile", "err", err)
		return
	}
	if err := c.state.update(func(s *state) { s.ProfileID = p.ID }); err != nil {
		slog.Warn("kroger: save profile id", "err", err)
		return
	}
	if prev != "" && prev != p.ID {
		slog.Info("kroger: linked account replaced", "previous_profile_id", prev, "profile_id", p.ID)
	}
}

var callbackPage = template.Must(template.New("callback").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>kroger-mcp</title>
<style>body{font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem;line-height:1.5}footer{margin-top:3rem;font-size:.8rem;opacity:.7}</style>
</head><body><h1>{{.Title}}</h1><p>{{.Message}}</p>
<footer>kroger-mcp is an unofficial, self-hosted project. It is not affiliated with or endorsed by The Kroger Co.</footer>
</body></html>
`))

// CallbackHandler handles Kroger's OAuth redirect. It must be reachable by
// the shopper's browser without MCP bearer auth; the single-use state is
// what ties the request back to a login URL issued by AuthorizeURL.
func (c *Client) CallbackHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The page loads nothing, must not be framed, and its URL carries the
		// authorization code, so it must not leak through Referer.
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")

		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()

		render := func(status int, title, msg string) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(status)
			_ = callbackPage.Execute(w, struct{ Title, Message string }{title, msg})
		}

		if e := truncate(q.Get("error"), maxCallbackParam); e != "" {
			// Consume the state so the URL can't be retried.
			c.pendingMu.Lock()
			delete(c.pending, q.Get("state"))
			c.pendingMu.Unlock()
			slog.Warn("kroger: authorization denied", "error", e, "description", truncate(q.Get("error_description"), maxCallbackParam))
			render(http.StatusBadRequest, "Kroger account not linked", "Kroger returned: "+e+". Ask Claude for a new link to try again.")
			return
		}

		err := c.CompleteAuthorization(r.Context(), q.Get("state"), q.Get("code"))
		switch {
		case errors.Is(err, ErrUnknownState):
			render(http.StatusBadRequest, "Link expired", "This login link has expired or was already used. Ask Claude for a new one.")
		case err != nil:
			slog.Error("kroger: complete authorization", "err", err)
			render(http.StatusBadGateway, "Kroger account not linked", "Something went wrong exchanging the code with Kroger. Check the server logs, then ask Claude for a new link.")
		default:
			slog.Info("kroger: account linked")
			render(http.StatusOK, "Kroger account linked", "You can close this tab and go back to Claude.")
		}
	})
}

// truncate shortens s to at most n runes, marking the cut with an ellipsis.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// randomString returns n random bytes, base64url-encoded without padding.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// codeChallenge derives the PKCE S256 challenge for verifier.
func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
