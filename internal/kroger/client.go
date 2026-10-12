// Package kroger is a client for the Kroger Public API
// (https://developer.kroger.com). It holds two kinds of Kroger credentials:
//
//   - an app token from the client-credentials grant, fetched and refreshed
//     in memory, used for product and location lookups; and
//   - a user token from the authorization-code grant, obtained once when the
//     shopper links their Kroger account in a browser, persisted to the data
//     directory and refreshed automatically, used for the cart and profile.
package kroger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrNotLinked means an operation needs the shopper's Kroger account and it
// has not been linked yet, or the link has expired.
var ErrNotLinked = errors.New("kroger account is not linked: call the kroger_login tool and open the returned URL to link it")

// ErrRateLimited matches errors from a 429 response: the app has used up one
// of its Kroger API quotas.
var ErrRateLimited = errors.New("kroger api: rate limited")

// maxRetryWait is the longest Retry-After a GET will wait out before retrying.
// Anything longer means a quota is spent, and waiting inside a tool call won't
// help.
const maxRetryWait = 5 * time.Second

// APIError is a non-2xx response from the Kroger API.
type APIError struct {
	Status int
	Body   string
	// RetryAfter is the wait the response's Retry-After header asked for, or
	// zero when it had none.
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Status == http.StatusTooManyRequests {
		msg := "kroger api: rate limited (status 429): this app's Kroger API quota is used up, so try again later"
		if e.RetryAfter > 0 {
			msg += fmt.Sprintf(" (Kroger asked to wait %s)", e.RetryAfter.Round(time.Second))
		}
		return msg + ". Retrying right away will fail again."
	}
	body := e.Body
	if len(body) > 500 {
		body = body[:500] + "…"
	}
	return fmt.Sprintf("kroger api: status %d: %s", e.Status, body)
}

// Is reports a 429 as ErrRateLimited.
func (e *APIError) Is(target error) bool {
	return target == ErrRateLimited && e.Status == http.StatusTooManyRequests
}

// Client talks to the Kroger API. It is safe for concurrent use.
type Client struct {
	cfg   Config
	http  *http.Client
	state *stateFile
	now   func() time.Time
	// sleep waits before a retry; tests replace it.
	sleep func(context.Context, time.Duration) error

	appMu    sync.Mutex
	appToken *token

	// userMu serializes user-token refreshes so concurrent tool calls don't
	// race to spend the same refresh token.
	userMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]pendingAuth

	products *productCache
}

// New validates cfg and loads any persisted state from cfg.DataDir.
func New(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.DefaultModality == "" {
		cfg.DefaultModality = ModalityPickup
	}
	st, err := loadState(cfg.statePath())
	if err != nil {
		return nil, err
	}
	c := &Client{
		cfg:     cfg,
		http:    &http.Client{Timeout: 30 * time.Second},
		state:   st,
		now:     time.Now,
		sleep:   sleepCtx,
		pending: make(map[string]pendingAuth),
	}
	// Read c.now on each call so tests can move the clock.
	c.products = newProductCache(func() time.Time { return c.now() })
	return c, nil
}

// Config returns the client's configuration.
func (c *Client) Config() Config { return c.cfg }

// Linked reports whether a shopper's Kroger account has been linked. The link
// may still turn out to be expired when next used.
func (c *Client) Linked() bool {
	return c.state.get().UserToken != nil
}

// ResolveLocationID picks the store to scope a request to: an explicit id,
// then the saved preference, then KROGER_LOCATION_ID. It may return "".
func (c *Client) ResolveLocationID(explicit string) string {
	if id := strings.TrimSpace(explicit); id != "" {
		return id
	}
	if id := c.state.get().PreferredLocationID; id != "" {
		return id
	}
	return c.cfg.DefaultLocationID
}

// PreferredLocationID returns the saved default store, if any.
func (c *Client) PreferredLocationID() string {
	return c.state.get().PreferredLocationID
}

// SetPreferredLocationID saves id as the default store.
func (c *Client) SetPreferredLocationID(id string) error {
	return c.state.update(func(s *state) { s.PreferredLocationID = id })
}

// tokenResponse is the body of a successful token-endpoint call.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// postToken calls the token endpoint with the app's client credentials.
func (c *Client) postToken(ctx context.Context, form url.Values) (*token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/connect/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	var tr tokenResponse
	if err := c.do(req, &tr); err != nil {
		return nil, err
	}
	if tr.AccessToken == "" {
		return nil, errors.New("kroger api: token response had no access_token")
	}
	return &token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		Expiry:       c.now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}, nil
}

// appAccessToken returns a client-credentials token, fetching a new one when
// the cached token is missing or near expiry.
func (c *Client) appAccessToken(ctx context.Context) (string, error) {
	c.appMu.Lock()
	defer c.appMu.Unlock()
	if c.appToken.valid(c.now()) {
		return c.appToken.AccessToken, nil
	}
	t, err := c.postToken(ctx, url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {appScope},
	})
	if err != nil {
		return "", fmt.Errorf("get app token: %w", err)
	}
	c.appToken = t
	return t.AccessToken, nil
}

// userAccessToken returns the linked shopper's access token, refreshing and
// persisting it when near expiry. A rejected refresh unlinks the account.
func (c *Client) userAccessToken(ctx context.Context) (string, error) {
	c.userMu.Lock()
	defer c.userMu.Unlock()

	cur := c.state.get().UserToken
	if cur == nil {
		return "", ErrNotLinked
	}
	if cur.valid(c.now()) {
		return cur.AccessToken, nil
	}
	if cur.RefreshToken == "" {
		return "", ErrNotLinked
	}

	t, err := c.postToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {cur.RefreshToken},
	})
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && (apiErr.Status == http.StatusBadRequest || apiErr.Status == http.StatusUnauthorized) {
			// Refresh tokens last about six months; after that the shopper
			// has to link again.
			_ = c.state.update(func(s *state) { s.UserToken = nil })
			return "", fmt.Errorf("%w (refresh rejected: %v)", ErrNotLinked, err)
		}
		return "", fmt.Errorf("refresh user token: %w", err)
	}
	if t.RefreshToken == "" {
		t.RefreshToken = cur.RefreshToken
	}
	if err := c.state.update(func(s *state) { s.UserToken = t }); err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

// get performs an authenticated GET of path (relative to BaseURL) and
// decodes the JSON response into out.
func (c *Client) get(ctx context.Context, bearer, path string, query url.Values, out any) error {
	u := c.cfg.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/json")
	return c.do(req, out)
}

// sendJSON performs an authenticated request with a JSON body.
func (c *Client) sendJSON(ctx context.Context, method, bearer, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.do(req, out)
}

// do sends req, turns non-2xx responses into *APIError, and decodes a JSON
// body into out when out is non-nil and the body is non-empty. A GET that is
// throttled (429 or 503) with a short Retry-After is retried once. Nothing
// else is retried: a repeated cart add could add items twice.
func (c *Client) do(req *http.Request, out any) error {
	err := c.send(req, out)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !retryable(req, apiErr) {
		return err
	}
	if c.sleep(req.Context(), apiErr.RetryAfter) != nil {
		return err
	}
	return c.send(req.Clone(req.Context()), out)
}

// retryable reports whether a failed request may be sent again.
func retryable(req *http.Request, e *APIError) bool {
	if req.Method != http.MethodGet {
		return false
	}
	if e.Status != http.StatusTooManyRequests && e.Status != http.StatusServiceUnavailable {
		return false
	}
	return e.RetryAfter > 0 && e.RetryAfter <= maxRetryWait
}

// send makes one attempt at req.
func (c *Client) send(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("kroger api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("kroger api: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		apiErr := &APIError{
			Status:     resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), c.now()),
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			slog.Warn("kroger: rate limited", "method", req.Method, "path", req.URL.Path, "retry_after", apiErr.RetryAfter)
		}
		return apiErr
	}
	if out == nil || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("kroger api: decode response: %w", err)
	}
	return nil
}

// parseRetryAfter reads a Retry-After header, either delay-seconds or an
// HTTP date. It returns zero when the header is missing or unparseable.
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}

// sleepCtx waits for d or until ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
