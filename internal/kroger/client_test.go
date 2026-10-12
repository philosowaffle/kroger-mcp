package kroger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/philosowaffle/kroger-cart/internal/kroger/krogertest"
)

// newTestClient returns a client wired to a fresh fake Kroger API with its
// own data directory.
func newTestClient(t *testing.T) (*Client, *krogertest.Server) {
	t.Helper()
	fake := krogertest.New(t)
	c, err := New(Config{
		ClientID:     krogertest.ClientID,
		ClientSecret: krogertest.ClientSecret,
		RedirectURI:  "http://localhost:8080/kroger/callback",
		DataDir:      t.TempDir(),
		BaseURL:      fake.URL,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, fake
}

// link runs the authorization-code flow against the fake.
func link(t *testing.T, c *Client) {
	t.Helper()
	u, err := c.AuthorizeURL()
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	pu, _ := url.Parse(u)
	if err := c.CompleteAuthorization(context.Background(), pu.Query().Get("state"), "good-code"); err != nil {
		t.Fatalf("CompleteAuthorization: %v", err)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		for _, k := range []string{"KROGER_CLIENT_ID", "KROGER_CLIENT_SECRET", "KROGER_REDIRECT_URI", "KROGER_LOCATION_ID", "KROGER_MODALITY", "KROGER_MCP_DATA_DIR"} {
			t.Setenv(k, "")
		}
		got := ConfigFromEnv()
		want := Config{
			RedirectURI:     DefaultRedirectURI,
			DefaultModality: ModalityPickup,
			DataDir:         DefaultDataDir,
			BaseURL:         DefaultBaseURL,
		}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
	t.Run("from env", func(t *testing.T) {
		t.Setenv("KROGER_CLIENT_ID", " id ")
		t.Setenv("KROGER_CLIENT_SECRET", "secret")
		t.Setenv("KROGER_REDIRECT_URI", "https://kroger.example.com/cb")
		t.Setenv("KROGER_LOCATION_ID", "123")
		t.Setenv("KROGER_MODALITY", "delivery")
		t.Setenv("KROGER_MCP_DATA_DIR", "/data")
		got := ConfigFromEnv()
		want := Config{
			ClientID:          "id",
			ClientSecret:      "secret",
			RedirectURI:       "https://kroger.example.com/cb",
			DefaultLocationID: "123",
			DefaultModality:   ModalityDelivery,
			DataDir:           "/data",
			BaseURL:           DefaultBaseURL,
		}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

func TestConfigValidate(t *testing.T) {
	good := Config{ClientID: "id", ClientSecret: "s", RedirectURI: DefaultRedirectURI, DataDir: "d", DefaultModality: ModalityPickup}
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"valid", func(*Config) {}, ""},
		{"missing client id", func(c *Config) { c.ClientID = "" }, "KROGER_CLIENT_ID"},
		{"missing secret", func(c *Config) { c.ClientSecret = "" }, "KROGER_CLIENT_SECRET"},
		{"relative redirect", func(c *Config) { c.RedirectURI = "/callback" }, "KROGER_REDIRECT_URI"},
		{"bad modality", func(c *Config) { c.DefaultModality = "SHIP" }, "KROGER_MODALITY"},
		{"empty data dir", func(c *Config) { c.DataDir = "" }, "KROGER_MCP_DATA_DIR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := good
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want mention of %s", err, tt.wantErr)
			}
		})
	}
}

func TestCallbackPath(t *testing.T) {
	tests := map[string]string{
		"http://localhost:8080/kroger/callback": "/kroger/callback",
		"https://example.com/cb":                "/cb",
		"https://example.com":                   "/",
	}
	for uri, want := range tests {
		if got := (Config{RedirectURI: uri}).CallbackPath(); got != want {
			t.Errorf("CallbackPath(%q) = %q, want %q", uri, got, want)
		}
	}
}

func TestClamp(t *testing.T) {
	tests := []struct{ v, want int }{{0, 10}, {-3, 10}, {1, 1}, {25, 25}, {50, 50}, {51, 50}}
	for _, tt := range tests {
		if got := clamp(tt.v, 1, 50, 10); got != tt.want {
			t.Errorf("clamp(%d) = %d, want %d", tt.v, got, tt.want)
		}
	}
}

func TestResolveLocationID(t *testing.T) {
	c, _ := newTestClient(t)
	c.cfg.DefaultLocationID = "env"

	if got := c.ResolveLocationID(""); got != "env" {
		t.Errorf("env fallback: got %q", got)
	}
	if err := c.SetPreferredLocationID("saved"); err != nil {
		t.Fatal(err)
	}
	if got := c.ResolveLocationID(""); got != "saved" {
		t.Errorf("saved preference: got %q", got)
	}
	if got := c.ResolveLocationID(" explicit "); got != "explicit" {
		t.Errorf("explicit: got %q", got)
	}
}

func TestAppTokenCachedAndRenewed(t *testing.T) {
	c, fake := newTestClient(t)
	now := time.Now()
	c.now = func() time.Time { return now }
	ctx := context.Background()

	for range 3 {
		if _, err := c.SearchLocations(ctx, LocationQuery{ZipCode: "80218"}); err != nil {
			t.Fatal(err)
		}
	}
	if n := fake.GrantCount("client_credentials"); n != 1 {
		t.Fatalf("client_credentials grants = %d, want 1 (token should be cached)", n)
	}

	now = now.Add(time.Duration(fake.ExpiresIn) * time.Second)
	if _, err := c.SearchLocations(ctx, LocationQuery{ZipCode: "80218"}); err != nil {
		t.Fatal(err)
	}
	if n := fake.GrantCount("client_credentials"); n != 2 {
		t.Fatalf("client_credentials grants = %d, want 2 after expiry", n)
	}
	if got := fake.Grants()[0].Get("scope"); got != appScope {
		t.Errorf("app token scope = %q, want %q", got, appScope)
	}
}

func TestBadCredentials(t *testing.T) {
	c, _ := newTestClient(t)
	c.cfg.ClientSecret = "wrong"
	_, err := c.SearchLocations(context.Background(), LocationQuery{ZipCode: "80218"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("err = %v, want APIError 401", err)
	}
}

func liteLocations(recs []LocationRecord) []Location {
	out := make([]Location, len(recs))
	for i, r := range recs {
		out[i] = r.Lite
	}
	return out
}

func liteProducts(recs []ProductRecord) []Product {
	out := make([]Product, len(recs))
	for i, r := range recs {
		out[i] = r.Lite
	}
	return out
}

func lastQuery(t *testing.T, fake *krogertest.Server, path string) url.Values {
	t.Helper()
	qs := fake.Queries(path)
	if len(qs) == 0 {
		t.Fatalf("no requests to %s", path)
	}
	return qs[len(qs)-1]
}

func assertQuery(t *testing.T, q url.Values, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if v == "" {
			if q.Has(k) {
				t.Errorf("query %s = %q, want absent", k, q.Get(k))
			}
			continue
		}
		if q.Get(k) != v {
			t.Errorf("query %s = %q, want %q", k, q.Get(k), v)
		}
	}
}

func ptr(f float64) *float64 { return &f }

func TestSearchLocations(t *testing.T) {
	c, fake := newTestClient(t)
	ctx := context.Background()

	t.Run("validation", func(t *testing.T) {
		tests := []struct {
			name string
			q    LocationQuery
			want string
		}{
			{"nothing to search by", LocationQuery{}, "one of zip_code"},
			{"latitude alone", LocationQuery{Latitude: ptr(39.7)}, "together"},
			{"longitude alone", LocationQuery{Longitude: ptr(-104.9)}, "together"},
			{"latitude out of range", LocationQuery{Latitude: ptr(91), Longitude: ptr(0)}, "within"},
			{"longitude out of range", LocationQuery{Latitude: ptr(0), Longitude: ptr(-181)}, "within"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := c.SearchLocations(ctx, tt.q)
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("err = %v, want mention of %q", err, tt.want)
				}
			})
		}
		if n := len(fake.Queries("/locations")); n != 0 {
			t.Errorf("invalid queries reached the API %d times", n)
		}
	})

	t.Run("by zip", func(t *testing.T) {
		got, err := c.SearchLocations(ctx, LocationQuery{ZipCode: "80218", RadiusMiles: 5, Chain: "KINGSOOPERS", Limit: 3})
		if err != nil {
			t.Fatal(err)
		}
		want := []Location{{
			LocationID: krogertest.LocationID,
			Chain:      "KINGSOOPERS",
			Name:       "King Soopers - Speer",
			Address:    "900 E 11th Ave, Denver, CO 80218",
			Phone:      "3035550100",
		}}
		if !reflect.DeepEqual(liteLocations(got), want) {
			t.Errorf("got %+v, want %+v", liteLocations(got), want)
		}
		assertQuery(t, lastQuery(t, fake, "/locations"), map[string]string{
			"filter.zipCode.near":  "80218",
			"filter.radiusInMiles": "5",
			"filter.chain":         "KINGSOOPERS",
			"filter.limit":         "3",
			"filter.latLong.near":  "",
			"filter.department":    "",
			"filter.locationId":    "",
		})
	})

	t.Run("coordinates win over zip", func(t *testing.T) {
		if _, err := c.SearchLocations(ctx, LocationQuery{ZipCode: "80218", Latitude: ptr(39.7334), Longitude: ptr(-104.9767), RadiusMiles: 500}); err != nil {
			t.Fatal(err)
		}
		assertQuery(t, lastQuery(t, fake, "/locations"), map[string]string{
			"filter.latLong.near":  "39.7334,-104.9767",
			"filter.zipCode.near":  "",
			"filter.radiusInMiles": "100",
		})
	})

	t.Run("departments and ids", func(t *testing.T) {
		if _, err := c.SearchLocations(ctx, LocationQuery{ZipCode: "80218", DepartmentIDs: []string{"09", " ", "01"}}); err != nil {
			t.Fatal(err)
		}
		assertQuery(t, lastQuery(t, fake, "/locations"), map[string]string{"filter.department": "09,01"})

		if _, err := c.SearchLocations(ctx, LocationQuery{LocationIDs: []string{krogertest.LocationID, "62000045"}}); err != nil {
			t.Fatal(err)
		}
		assertQuery(t, lastQuery(t, fake, "/locations"), map[string]string{
			"filter.locationId":   krogertest.LocationID + ",62000045",
			"filter.zipCode.near": "",
		})
	})

	t.Run("full record", func(t *testing.T) {
		got, err := c.SearchLocations(ctx, LocationQuery{ZipCode: "80218"})
		if err != nil {
			t.Fatal(err)
		}
		full, err := got[0].Full()
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"hours", "departments", "geolocation", "storeNumber"} {
			if _, ok := full[k]; !ok {
				t.Errorf("full record missing %q: %v", k, full)
			}
		}
	})
}

func TestLocationAddressSkipsBlanks(t *testing.T) {
	var a apiLocation
	a.Address.City = "Denver"
	a.Address.State = "CO"
	if got := a.toLocation().Address; got != "Denver, CO" {
		t.Errorf("got %q", got)
	}
}

func TestGetLocation(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	loc, err := c.GetLocation(ctx, krogertest.LocationID)
	if err != nil {
		t.Fatal(err)
	}
	if loc.Lite.Name != "King Soopers - Speer" {
		t.Errorf("name = %q", loc.Lite.Name)
	}
	if full, err := loc.Full(); err != nil || full["hours"] == nil {
		t.Errorf("full = %v, err = %v", full, err)
	}

	_, err = c.GetLocation(ctx, "nope")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("err = %v, want APIError 404", err)
	}
	if _, err := c.GetLocation(ctx, " "); err == nil {
		t.Error("want error for empty id")
	}
}

func TestChains(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	chains, err := c.ListChains(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []Chain{
		{Name: krogertest.Chain, DivisionNumbers: []string{"620"}},
		{Name: "RALPHS", DivisionNumbers: []string{"703"}},
	}
	if !reflect.DeepEqual(chains, want) {
		t.Errorf("chains = %+v, want %+v", chains, want)
	}

	ch, err := c.GetChain(ctx, krogertest.Chain)
	if err != nil || ch.Name != krogertest.Chain {
		t.Errorf("GetChain = %+v, %v", ch, err)
	}
	var apiErr *APIError
	if _, err := c.GetChain(ctx, "NOPE"); !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Errorf("unknown chain err = %v", err)
	}
	if _, err := c.GetChain(ctx, ""); err == nil {
		t.Error("want error for empty name")
	}
}

func TestDepartments(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	deps, err := c.ListDepartments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []Department{{DepartmentID: krogertest.DepartmentID, Name: "Pharmacy"}, {DepartmentID: "01", Name: "Bakery"}}
	if !reflect.DeepEqual(deps, want) {
		t.Errorf("departments = %+v", deps)
	}

	d, err := c.GetDepartment(ctx, krogertest.DepartmentID)
	if err != nil || *d != want[0] {
		t.Errorf("GetDepartment = %+v, %v", d, err)
	}
	var apiErr *APIError
	if _, err := c.GetDepartment(ctx, "99"); !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Errorf("unknown department err = %v", err)
	}
	if _, err := c.GetDepartment(ctx, ""); err == nil {
		t.Error("want error for empty id")
	}
}

func TestSearchProducts(t *testing.T) {
	c, fake := newTestClient(t)
	ctx := context.Background()

	if _, err := c.SearchProducts(ctx, ProductQuery{Term: "  "}); err == nil || !strings.Contains(err.Error(), "one of term") {
		t.Fatalf("blank query err = %v", err)
	}

	t.Run("store scoped", func(t *testing.T) {
		got, err := c.SearchProducts(ctx, ProductQuery{Term: "milk", LocationID: krogertest.LocationID, Brand: "Kroger", Limit: 99, Start: 10})
		if err != nil {
			t.Fatal(err)
		}
		want := []Product{{
			ProductID:    krogertest.ProductID,
			UPC:          krogertest.ProductID,
			Brand:        "Kroger",
			Description:  "Kroger® 2% Reduced Fat Milk",
			Size:         "1 gal",
			Categories:   []string{"Dairy"},
			RegularPrice: 4.29,
			PromoPrice:   3.49,
			StockLevel:   "HIGH",
			Aisle:        "Dairy",
			Fulfillment:  []string{"curbside", "delivery", "inStore"},
		}}
		if !reflect.DeepEqual(liteProducts(got), want) {
			t.Errorf("got %+v\nwant %+v", liteProducts(got), want)
		}
		assertQuery(t, lastQuery(t, fake, "/products"), map[string]string{
			"filter.term":        "milk",
			"filter.locationId":  krogertest.LocationID,
			"filter.brand":       "Kroger",
			"filter.limit":       "50",
			"filter.start":       "10",
			"filter.productId":   "",
			"filter.fulfillment": "",
		})
	})

	t.Run("full record has extras", func(t *testing.T) {
		got, err := c.SearchProducts(ctx, ProductQuery{Term: "milk"})
		if err != nil {
			t.Fatal(err)
		}
		full, err := got[0].Full()
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"images", "temperature", "items"} {
			if _, ok := full[k]; !ok {
				t.Errorf("full record missing %q", k)
			}
		}
	})

	t.Run("unscoped has no store data", func(t *testing.T) {
		got, err := c.SearchProducts(ctx, ProductQuery{Term: "milk"})
		if err != nil {
			t.Fatal(err)
		}
		p := got[0].Lite
		if p.RegularPrice != 0 || p.StockLevel != "" || p.Aisle != "" || p.Fulfillment != nil {
			t.Errorf("unexpected store data: %+v", p)
		}
		assertQuery(t, lastQuery(t, fake, "/products"), map[string]string{"filter.locationId": ""})
	})

	t.Run("by product ids and brand alone", func(t *testing.T) {
		got, err := c.SearchProducts(ctx, ProductQuery{ProductIDs: []string{krogertest.ProductID, " ", "0000000000001"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Errorf("got %d products", len(got))
		}
		assertQuery(t, lastQuery(t, fake, "/products"), map[string]string{
			"filter.productId": krogertest.ProductID + ",0000000000001",
			"filter.term":      "",
		})
		if _, err := c.SearchProducts(ctx, ProductQuery{Brand: "Kroger"}); err != nil {
			t.Errorf("brand-only search: %v", err)
		}
	})

	t.Run("fulfillment", func(t *testing.T) {
		for in, code := range map[string]string{
			"in_store": "ais", "curbside": "csp", "pickup": "csp", "Delivery": "dth",
			"ship_to_home": "sth", "sth": "sth",
		} {
			if _, err := c.SearchProducts(ctx, ProductQuery{Term: "milk", Fulfillment: in}); err != nil {
				t.Fatalf("%s: %v", in, err)
			}
			assertQuery(t, lastQuery(t, fake, "/products"), map[string]string{"filter.fulfillment": code})
		}
		if _, err := c.SearchProducts(ctx, ProductQuery{Term: "milk", Fulfillment: "drone"}); err == nil || !strings.Contains(err.Error(), "fulfillment") {
			t.Errorf("bad fulfillment err = %v", err)
		}
	})

	t.Run("no results", func(t *testing.T) {
		got, err := c.SearchProducts(ctx, ProductQuery{Term: "nothing"})
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("got %#v, want empty non-nil slice", got)
		}
	})
}

func TestProductPromoIgnoredWhenNotLower(t *testing.T) {
	var a apiProduct
	if err := json.Unmarshal([]byte(`{"items":[{"price":{"regular":2.5,"promo":2.5}}]}`), &a); err != nil {
		t.Fatal(err)
	}
	if p := a.toProduct(); p.PromoPrice != 0 || p.RegularPrice != 2.5 {
		t.Errorf("got regular %v promo %v, want 2.5 and no promo", p.RegularPrice, p.PromoPrice)
	}
}

func TestGetProduct(t *testing.T) {
	c, fake := newTestClient(t)
	ctx := context.Background()

	p, err := c.GetProduct(ctx, krogertest.ProductID, krogertest.LocationID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Lite.RegularPrice != 4.29 {
		t.Errorf("price = %v", p.Lite.RegularPrice)
	}
	if got := fake.Queries("/products/" + krogertest.ProductID)[0].Get("filter.locationId"); got != krogertest.LocationID {
		t.Errorf("filter.locationId = %q", got)
	}

	if _, err := c.GetProduct(ctx, "", ""); err == nil {
		t.Error("want error for empty id")
	}
	var apiErr *APIError
	if _, err := c.GetProduct(ctx, "0000000000001", ""); !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Errorf("unknown product err = %v", err)
	}
}

func TestGetProductCache(t *testing.T) {
	c, fake := newTestClient(t)
	now := time.Now()
	c.now = func() time.Time { return now }
	ctx := context.Background()
	detailCalls := func() int { return len(fake.Queries("/products/" + krogertest.ProductID)) }

	// A search fills the cache for the store it was scoped to.
	if _, err := c.SearchProducts(ctx, ProductQuery{Term: "milk", LocationID: krogertest.LocationID}); err != nil {
		t.Fatal(err)
	}
	rec, err := c.GetProduct(ctx, krogertest.ProductID, krogertest.LocationID)
	if err != nil {
		t.Fatal(err)
	}
	if n := detailCalls(); n != 0 {
		t.Fatalf("detail calls = %d, want 0 (served from search cache)", n)
	}
	if full, _ := rec.Full(); full["images"] == nil {
		t.Error("cached record lost full data")
	}

	// A different store is a different entry: prices differ per store.
	if _, err := c.GetProduct(ctx, krogertest.ProductID, ""); err != nil {
		t.Fatal(err)
	}
	if n := detailCalls(); n != 1 {
		t.Fatalf("detail calls = %d, want 1 for a new store", n)
	}
	if _, err := c.GetProduct(ctx, krogertest.ProductID, ""); err != nil {
		t.Fatal(err)
	}
	if n := detailCalls(); n != 1 {
		t.Fatalf("detail calls = %d, want 1 (second lookup cached)", n)
	}

	// Entries expire after the TTL.
	now = now.Add(productCacheTTL)
	if _, err := c.GetProduct(ctx, krogertest.ProductID, krogertest.LocationID); err != nil {
		t.Fatal(err)
	}
	if n := detailCalls(); n != 2 {
		t.Fatalf("detail calls = %d, want 2 after expiry", n)
	}
}

func TestProductCacheEviction(t *testing.T) {
	now := time.Now()
	pc := newProductCache(func() time.Time { return now })
	raw := json.RawMessage(`{}`)

	pc.put("old", "", raw)
	now = now.Add(time.Second)
	for i := range productCacheMax - 1 {
		pc.put(fmt.Sprintf("p%d", i), "", raw)
	}
	if len(pc.entries) != productCacheMax {
		t.Fatalf("entries = %d, want %d", len(pc.entries), productCacheMax)
	}

	// Full cache: adding one more evicts the oldest.
	pc.put("new", "", raw)
	if len(pc.entries) != productCacheMax {
		t.Fatalf("entries = %d after eviction, want %d", len(pc.entries), productCacheMax)
	}
	if _, ok := pc.get("old", ""); ok {
		t.Error("oldest entry survived eviction")
	}
	if _, ok := pc.get("new", ""); !ok {
		t.Error("new entry missing")
	}

	// Expired entries go first, all at once.
	now = now.Add(productCacheTTL)
	pc.put("fresh", "", raw)
	if len(pc.entries) != 1 {
		t.Errorf("entries = %d, want only the fresh one after expired sweep", len(pc.entries))
	}

	// Empty ids are never cached.
	pc.put("", "", raw)
	if _, ok := pc.get("", ""); ok {
		t.Error("cached an empty product id")
	}
}

func TestWantFull(t *testing.T) {
	tests := []struct {
		in      string
		want    bool
		wantErr bool
	}{
		{"", false, false},
		{"lite", false, false},
		{" FULL ", true, false},
		{"everything", false, true},
	}
	for _, tt := range tests {
		got, err := WantFull(tt.in)
		if got != tt.want || (err != nil) != tt.wantErr {
			t.Errorf("WantFull(%q) = %v, %v", tt.in, got, err)
		}
	}
}

func TestAuthorizeURL(t *testing.T) {
	c, _ := newTestClient(t)
	raw, err := c.AuthorizeURL()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/connect/oauth2/authorize" {
		t.Errorf("path = %q", u.Path)
	}
	q := u.Query()
	for k, v := range map[string]string{
		"response_type":         "code",
		"client_id":             krogertest.ClientID,
		"redirect_uri":          "http://localhost:8080/kroger/callback",
		"scope":                 "cart.basic:write profile.compact product.compact",
		"code_challenge_method": "S256",
	} {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	if q.Get("state") == "" || q.Get("code_challenge") == "" {
		t.Error("state and code_challenge must be set")
	}

	raw2, _ := c.AuthorizeURL()
	u2, _ := url.Parse(raw2)
	if u2.Query().Get("state") == q.Get("state") {
		t.Error("each URL must carry a fresh state")
	}
}

func TestAuthorizeURL_CapsPendingLogins(t *testing.T) {
	c, _ := newTestClient(t)
	now := time.Now()
	c.now = func() time.Time { return now }

	stateOf := func() string {
		raw, err := c.AuthorizeURL()
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(raw)
		return u.Query().Get("state")
	}
	states := make([]string, 0, maxPendingAuths+1)
	for range maxPendingAuths + 1 {
		states = append(states, stateOf())
		now = now.Add(time.Second)
	}

	c.pendingMu.Lock()
	n := len(c.pending)
	c.pendingMu.Unlock()
	if n != maxPendingAuths {
		t.Fatalf("pending = %d, want %d", n, maxPendingAuths)
	}

	ctx := context.Background()
	if err := c.CompleteAuthorization(ctx, states[0], "good-code"); !errors.Is(err, ErrUnknownState) {
		t.Errorf("oldest state: err = %v, want ErrUnknownState (evicted)", err)
	}
	if err := c.CompleteAuthorization(ctx, states[len(states)-1], "good-code"); err != nil {
		t.Errorf("newest state: err = %v", err)
	}
}

func TestCompleteAuthorization(t *testing.T) {
	ctx := context.Background()

	t.Run("links and persists", func(t *testing.T) {
		c, fake := newTestClient(t)
		raw, _ := c.AuthorizeURL()
		u, _ := url.Parse(raw)
		st := u.Query().Get("state")

		if c.Linked() {
			t.Fatal("linked before authorization")
		}
		if err := c.CompleteAuthorization(ctx, st, "good-code"); err != nil {
			t.Fatal(err)
		}
		if !c.Linked() {
			t.Fatal("not linked after authorization")
		}

		// PKCE: the verifier sent at exchange must hash to the challenge.
		if got := codeChallenge(fake.LastVerifier()); got != u.Query().Get("code_challenge") {
			t.Error("code_verifier does not match code_challenge")
		}
		g := fake.Grants()[0]
		if g.Get("redirect_uri") != "http://localhost:8080/kroger/callback" || g.Get("code") != "good-code" {
			t.Errorf("unexpected exchange form: %v", g)
		}

		path := c.cfg.statePath()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("state file perm = %o, want 600", perm)
		}

		// A fresh client on the same data dir picks the link back up.
		c2, err := New(c.cfg)
		if err != nil {
			t.Fatal(err)
		}
		if !c2.Linked() {
			t.Error("link not restored from disk")
		}
	})

	t.Run("state is single use", func(t *testing.T) {
		c, _ := newTestClient(t)
		raw, _ := c.AuthorizeURL()
		u, _ := url.Parse(raw)
		st := u.Query().Get("state")
		if err := c.CompleteAuthorization(ctx, st, "bad"); err == nil {
			t.Fatal("want exchange error")
		}
		if err := c.CompleteAuthorization(ctx, st, "good-code"); !errors.Is(err, ErrUnknownState) {
			t.Fatalf("reuse err = %v, want ErrUnknownState", err)
		}
	})

	t.Run("unknown state", func(t *testing.T) {
		c, _ := newTestClient(t)
		if err := c.CompleteAuthorization(ctx, "made-up", "good-code"); !errors.Is(err, ErrUnknownState) {
			t.Fatalf("err = %v, want ErrUnknownState", err)
		}
	})

	t.Run("expired state", func(t *testing.T) {
		c, _ := newTestClient(t)
		now := time.Now()
		c.now = func() time.Time { return now }
		raw, _ := c.AuthorizeURL()
		u, _ := url.Parse(raw)
		now = now.Add(pendingAuthTTL + time.Second)
		if err := c.CompleteAuthorization(ctx, u.Query().Get("state"), "good-code"); !errors.Is(err, ErrUnknownState) {
			t.Fatalf("err = %v, want ErrUnknownState", err)
		}
	})

	t.Run("missing code", func(t *testing.T) {
		c, _ := newTestClient(t)
		raw, _ := c.AuthorizeURL()
		u, _ := url.Parse(raw)
		if err := c.CompleteAuthorization(ctx, u.Query().Get("state"), ""); err == nil {
			t.Fatal("want error for missing code")
		}
	})
}

func TestUserTokenRefresh(t *testing.T) {
	ctx := context.Background()

	t.Run("refreshes and persists", func(t *testing.T) {
		c, fake := newTestClient(t)
		now := time.Now()
		c.now = func() time.Time { return now }
		link(t, c)
		before := c.state.get().UserToken

		if _, err := c.GetProfile(ctx); err != nil {
			t.Fatal(err)
		}
		if n := fake.GrantCount("refresh_token"); n != 0 {
			t.Fatalf("refreshed a valid token (%d grants)", n)
		}

		now = now.Add(time.Hour)
		if _, err := c.GetProfile(ctx); err != nil {
			t.Fatal(err)
		}
		if n := fake.GrantCount("refresh_token"); n != 1 {
			t.Fatalf("refresh grants = %d, want 1", n)
		}
		after := c.state.get().UserToken
		if after.AccessToken == before.AccessToken || after.RefreshToken == before.RefreshToken {
			t.Error("tokens not rotated")
		}
		reloaded, err := loadState(c.cfg.statePath())
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.get().UserToken.RefreshToken != after.RefreshToken {
			t.Error("refreshed token not persisted")
		}
	})

	t.Run("rejected refresh unlinks", func(t *testing.T) {
		c, fake := newTestClient(t)
		now := time.Now()
		c.now = func() time.Time { return now }
		link(t, c)
		fake.RejectRefresh = true
		now = now.Add(time.Hour)

		if _, err := c.GetProfile(ctx); !errors.Is(err, ErrNotLinked) {
			t.Fatalf("err = %v, want ErrNotLinked", err)
		}
		if c.Linked() {
			t.Error("still linked after refresh was rejected")
		}
	})

	t.Run("not linked", func(t *testing.T) {
		c, _ := newTestClient(t)
		if _, err := c.GetProfile(ctx); !errors.Is(err, ErrNotLinked) {
			t.Fatalf("err = %v, want ErrNotLinked", err)
		}
	})
}

func TestAddToCart(t *testing.T) {
	ctx := context.Background()

	t.Run("validation", func(t *testing.T) {
		c, fake := newTestClient(t)
		link(t, c)
		tests := []struct {
			name  string
			items []CartItem
			want  string
		}{
			{"empty", nil, "at least one item"},
			{"missing upc", []CartItem{{Quantity: 1}}, "upc is required"},
			{"zero quantity", []CartItem{{UPC: "1", Quantity: 0}}, "quantity"},
			{"bad modality", []CartItem{{UPC: "1", Quantity: 1, Modality: "SHIP"}}, "modality"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := c.AddToCart(ctx, tt.items)
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("err = %v, want mention of %q", err, tt.want)
				}
			})
		}
		if n := len(fake.CartAdds()); n != 0 {
			t.Errorf("invalid input reached the API %d times", n)
		}
	})

	t.Run("adds with default modality", func(t *testing.T) {
		c, fake := newTestClient(t)
		c.cfg.DefaultModality = ModalityDelivery
		link(t, c)
		added, err := c.AddToCart(ctx, []CartItem{
			{UPC: " 111 ", Quantity: 2},
			{UPC: "222", Quantity: 1, Modality: "pickup"},
		})
		if err != nil {
			t.Fatal(err)
		}
		want := []CartItem{
			{UPC: "111", Quantity: 2, Modality: ModalityDelivery},
			{UPC: "222", Quantity: 1, Modality: ModalityPickup},
		}
		if !reflect.DeepEqual(added, want) {
			t.Errorf("added = %+v, want %+v", added, want)
		}
		adds := fake.CartAdds()
		if len(adds) != 1 || len(adds[0].Items) != 2 || adds[0].Items[0].Modality != ModalityDelivery {
			t.Errorf("unexpected API calls: %+v", adds)
		}
	})

	t.Run("not linked", func(t *testing.T) {
		c, _ := newTestClient(t)
		if _, err := c.AddToCart(ctx, []CartItem{{UPC: "1", Quantity: 1}}); !errors.Is(err, ErrNotLinked) {
			t.Fatalf("err = %v, want ErrNotLinked", err)
		}
	})
}

func TestGetProfile(t *testing.T) {
	c, _ := newTestClient(t)
	link(t, c)
	p, err := c.GetProfile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != krogertest.ProfileID {
		t.Errorf("ID = %q", p.ID)
	}
}

func TestCallbackHandler(t *testing.T) {
	c, _ := newTestClient(t)
	h := c.CallbackHandler()

	stateFor := func() string {
		raw, _ := c.AuthorizeURL()
		u, _ := url.Parse(raw)
		return u.Query().Get("state")
	}
	call := func(method, query string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/kroger/callback?"+query, nil))
		return rec
	}

	t.Run("wrong method", func(t *testing.T) {
		if rec := call(http.MethodPost, ""); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d", rec.Code)
		}
	})
	t.Run("security headers on every response", func(t *testing.T) {
		// Its own client: the success case links an account, which the
		// unknown-state case below must not see.
		c2, _ := newTestClient(t)
		h2 := c2.CallbackHandler()
		call2 := func(method, query string) *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			h2.ServeHTTP(rec, httptest.NewRequest(method, "/kroger/callback?"+query, nil))
			return rec
		}
		raw, _ := c2.AuthorizeURL()
		u, _ := url.Parse(raw)
		for _, rec := range []*httptest.ResponseRecorder{
			call2(http.MethodPost, ""),
			call2(http.MethodGet, "error=access_denied"),
			call2(http.MethodGet, "code=good-code&state=forged"),
			call2(http.MethodGet, "code=good-code&state="+u.Query().Get("state")),
		} {
			for k, want := range map[string]string{
				"Referrer-Policy":        "no-referrer",
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
			} {
				if got := rec.Header().Get(k); got != want {
					t.Errorf("status %d: %s = %q, want %q", rec.Code, k, got, want)
				}
			}
			if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
				t.Errorf("status %d: Content-Security-Policy = %q", rec.Code, csp)
			}
		}
	})
	t.Run("long error is truncated", func(t *testing.T) {
		long := strings.Repeat("x", 5000)
		rec := call(http.MethodGet, "error="+long)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d", rec.Code)
		}
		if strings.Contains(rec.Body.String(), long[:maxCallbackParam+1]) {
			t.Error("error value rendered untruncated")
		}
	})
	t.Run("user denied consumes state", func(t *testing.T) {
		st := stateFor()
		if rec := call(http.MethodGet, "error=access_denied&state="+st); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d", rec.Code)
		}
		if rec := call(http.MethodGet, "code=good-code&state="+st); rec.Code != http.StatusBadRequest {
			t.Errorf("state reusable after denial: status = %d", rec.Code)
		}
	})
	t.Run("unknown state", func(t *testing.T) {
		rec := call(http.MethodGet, "code=good-code&state=forged")
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "expired") {
			t.Errorf("status = %d body = %s", rec.Code, rec.Body)
		}
		if c.Linked() {
			t.Error("forged state linked an account")
		}
	})
	t.Run("exchange failure", func(t *testing.T) {
		if rec := call(http.MethodGet, "code=bad&state="+stateFor()); rec.Code != http.StatusBadGateway {
			t.Errorf("status = %d", rec.Code)
		}
	})
	t.Run("success", func(t *testing.T) {
		rec := call(http.MethodGet, "code=good-code&state="+stateFor())
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "linked") {
			t.Errorf("status = %d body = %s", rec.Code, rec.Body)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q", got)
		}
		if !c.Linked() {
			t.Error("not linked")
		}
	})
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		want string
	}{
		{"", 3, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc…"},
		{"héllo", 2, "hé…"},
	}
	for _, tt := range tests {
		if got := truncate(tt.in, tt.n); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
		}
	}
}

func TestLoadStateCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := New(Config{ClientID: "id", ClientSecret: "s", RedirectURI: DefaultRedirectURI, DataDir: dir})
	if err == nil {
		t.Fatal("want error for corrupt state file")
	}
}

func TestStateCreatesDataDir(t *testing.T) {
	c, _ := newTestClient(t)
	c.state.path = filepath.Join(t.TempDir(), "nested", "dir", "state.json")
	if err := c.SetPreferredLocationID("1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.state.path); err != nil {
		t.Fatal(err)
	}
}
