// Package krogertest provides an in-process fake of the Kroger Public API for
// tests. It implements the token endpoint (client credentials, authorization
// code, refresh), locations, products, cart add and profile, and records the
// requests it receives so tests can assert on them.
package krogertest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// Credentials the fake accepts on the token endpoint.
const (
	ClientID     = "test-client"
	ClientSecret = "test-secret"
)

// Fixture data returned by the fake.
const (
	LocationID   = "62000044"
	ProductID    = "0001111041700"
	ProfileID    = "profile-123"
	Chain        = "KINGSOOPERS"
	DepartmentID = "09"
)

// CartAdd is one recorded PUT /cart/add call.
type CartAdd struct {
	Items []struct {
		UPC      string `json:"upc"`
		Quantity int    `json:"quantity"`
		Modality string `json:"modality"`
	} `json:"items"`
}

// Server is a fake Kroger API.
type Server struct {
	*httptest.Server

	// ExpiresIn is the lifetime, in seconds, of tokens the fake issues.
	ExpiresIn int
	// RejectRefresh makes refresh_token grants fail with 400.
	RejectRefresh bool

	mu           sync.Mutex
	issued       int
	userTokens   map[string]bool
	appTokens    map[string]bool
	refresh      map[string]bool
	grants       []url.Values
	queries      map[string][]url.Values
	cartAdds     []CartAdd
	lastVerifier string
	failures     map[string]failure
}

// failure is a canned error response queued by FailNext.
type failure struct {
	n          int
	status     int
	retryAfter string
}

// New starts a fake Kroger API, closed when the test ends.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{
		ExpiresIn:  1800,
		userTokens: map[string]bool{},
		appTokens:  map[string]bool{},
		refresh:    map[string]bool{},
		queries:    map[string][]url.Values{},
		failures:   map[string]failure{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /connect/oauth2/token", s.token)
	mux.HandleFunc("GET /locations", s.app(s.locations))
	mux.HandleFunc("GET /locations/{id}", s.app(s.location))
	mux.HandleFunc("GET /products", s.app(s.products))
	mux.HandleFunc("GET /products/{id}", s.app(s.product))
	mux.HandleFunc("GET /chains", s.app(s.chains))
	mux.HandleFunc("GET /chains/{name}", s.app(s.chain))
	mux.HandleFunc("GET /departments", s.app(s.departments))
	mux.HandleFunc("GET /departments/{id}", s.app(s.department))
	mux.HandleFunc("PUT /cart/add", s.user(s.cartAdd))
	mux.HandleFunc("GET /identity/profile", s.user(s.profile))
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

// Grants returns the form bodies of every token request, oldest first.
func (s *Server) Grants() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]url.Values(nil), s.grants...)
}

// GrantCount counts token requests with the given grant_type.
func (s *Server) GrantCount(grantType string) int {
	n := 0
	for _, g := range s.Grants() {
		if g.Get("grant_type") == grantType {
			n++
		}
	}
	return n
}

// Queries returns the query strings received on path, oldest first.
func (s *Server) Queries(path string) []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]url.Values(nil), s.queries[path]...)
}

// FailNext makes the next n requests to path fail with status, after they
// are recorded. retryAfter, when non-empty, is sent as the Retry-After header.
// Use it to simulate rate limiting (429) or an overloaded API (503).
func (s *Server) FailNext(path string, n, status int, retryAfter string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[path] = failure{n: n, status: status, retryAfter: retryAfter}
}

// CartAdds returns every recorded cart add, oldest first.
func (s *Server) CartAdds() []CartAdd {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]CartAdd(nil), s.cartAdds...)
}

func (s *Server) newToken(kind string) string {
	s.issued++
	return fmt.Sprintf("%s-%d", kind, s.issued)
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok || id != ClientID || secret != ClientSecret {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.grants = append(s.grants, r.PostForm)

	resp := map[string]any{"token_type": "bearer", "expires_in": s.ExpiresIn}
	switch r.PostForm.Get("grant_type") {
	case "client_credentials":
		tok := s.newToken("app")
		s.appTokens[tok] = true
		resp["access_token"] = tok
	case "authorization_code":
		if r.PostForm.Get("code") == "" || r.PostForm.Get("code") == "bad" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
			return
		}
		s.lastVerifier = r.PostForm.Get("code_verifier")
		resp["access_token"], resp["refresh_token"] = s.issueUser()
	case "refresh_token":
		if s.RejectRefresh || !s.refresh[r.PostForm.Get("refresh_token")] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
			return
		}
		delete(s.refresh, r.PostForm.Get("refresh_token"))
		resp["access_token"], resp["refresh_token"] = s.issueUser()
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_grant_type"})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) issueUser() (access, refresh string) {
	access, refresh = s.newToken("user"), s.newToken("refresh")
	s.userTokens[access] = true
	s.refresh[refresh] = true
	return access, refresh
}

// app requires a client-credentials or user bearer token; user requires a
// user token. Both record the query string.
func (s *Server) app(h http.HandlerFunc) http.HandlerFunc {
	return s.guard(func(tok string) bool { return s.appTokens[tok] || s.userTokens[tok] }, h)
}

func (s *Server) user(h http.HandlerFunc) http.HandlerFunc {
	return s.guard(func(tok string) bool { return s.userTokens[tok] }, h)
}

func (s *Server) guard(ok func(string) bool, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		s.mu.Lock()
		allowed := ok(tok)
		s.queries[r.URL.Path] = append(s.queries[r.URL.Path], r.URL.Query())
		f := s.failures[r.URL.Path]
		fail := f.n > 0
		if fail {
			f.n--
			s.failures[r.URL.Path] = f
		}
		s.mu.Unlock()
		if fail {
			if f.retryAfter != "" {
				w.Header().Set("Retry-After", f.retryAfter)
			}
			writeJSON(w, f.status, map[string]any{"errors": map[string]string{"reason": http.StatusText(f.status)}})
			return
		}
		if !allowed {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
			return
		}
		h(w, r)
	}
}

var location = map[string]any{
	"locationId":     LocationID,
	"storeNumber":    "00044",
	"divisionNumber": "620",
	"chain":          Chain,
	"name":           "King Soopers - Speer",
	"phone":          "3035550100",
	"address": map[string]any{
		"addressLine1": "900 E 11th Ave",
		"city":         "Denver",
		"state":        "CO",
		"zipCode":      "80218",
	},
	"geolocation": map[string]any{"latitude": 39.7334, "longitude": -104.9767},
	"hours": map[string]any{
		"timezone": "America/Denver",
		"monday":   map[string]any{"open": "06:00", "close": "22:00", "open24": false},
	},
	"departments": []any{map[string]any{"departmentId": DepartmentID, "name": "Pharmacy"}},
}

func (s *Server) locations(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": []any{location}})
}

func (s *Server) location(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != LocationID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": location})
}

// product builds the fixture; store-scoped data only appears with a location.
func product(scoped bool) map[string]any {
	item := map[string]any{"itemId": ProductID, "size": "1 gal"}
	if scoped {
		item["price"] = map[string]any{"regular": 4.29, "promo": 3.49}
		item["inventory"] = map[string]any{"stockLevel": "HIGH"}
		item["fulfillment"] = map[string]any{"curbside": true, "delivery": true, "inStore": true, "shipToHome": false}
	}
	p := map[string]any{
		"productId":   ProductID,
		"upc":         ProductID,
		"brand":       "Kroger",
		"description": "Kroger® 2% Reduced Fat Milk",
		"categories":  []string{"Dairy"},
		"items":       []any{item},
		"images": []any{map[string]any{
			"perspective": "front",
			"featured":    true,
			"sizes":       []any{map[string]any{"size": "medium", "url": "https://example.com/milk.jpg"}},
		}},
		"temperature": map[string]any{"indicator": "Refrigerated", "heatSensitive": false},
	}
	if scoped {
		p["aisleLocations"] = []any{map[string]any{"description": "Dairy", "number": "30"}}
	}
	return p
}

func (s *Server) products(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("filter.term") == "" && q.Get("filter.productId") == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"errors": map[string]string{
			"reason": "Field 'term' or 'productId' must be used", "code": "PRODUCT-2016",
		}})
		return
	}
	if q.Get("filter.term") == "nothing" || (q.Has("filter.productId") && !strings.Contains(q.Get("filter.productId"), ProductID)) {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	scoped := r.URL.Query().Get("filter.locationId") != ""
	writeJSON(w, http.StatusOK, map[string]any{"data": []any{product(scoped)}})
}

func (s *Server) product(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != ProductID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	scoped := r.URL.Query().Get("filter.locationId") != ""
	writeJSON(w, http.StatusOK, map[string]any{"data": product(scoped)})
}

func (s *Server) chains(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": []any{
		map[string]any{"name": Chain, "divisionNumbers": []string{"620"}},
		map[string]any{"name": "RALPHS", "divisionNumbers": []string{"703"}},
	}})
}

func (s *Server) chain(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("name") != Chain {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"name": Chain, "divisionNumbers": []string{"620"}}})
}

func (s *Server) departments(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": []any{
		map[string]any{"departmentId": DepartmentID, "name": "Pharmacy"},
		map[string]any{"departmentId": "01", "name": "Bakery"},
	}})
}

func (s *Server) department(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != DepartmentID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"departmentId": DepartmentID, "name": "Pharmacy"}})
}

func (s *Server) cartAdd(w http.ResponseWriter, r *http.Request) {
	var add CartAdd
	if err := json.NewDecoder(r.Body).Decode(&add); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.mu.Lock()
	s.cartAdds = append(s.cartAdds, add)
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) profile(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"id": ProfileID}})
}

// LastVerifier is the PKCE code_verifier sent with the latest code exchange.
func (s *Server) LastVerifier() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastVerifier
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
