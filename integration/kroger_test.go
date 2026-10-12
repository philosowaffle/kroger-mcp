//go:build integration

// Package integration runs kroger-mcp against a live Kroger API, by default
// the certification environment. It covers everything an app token can
// reach: locations, chains, departments and products, through the client and
// end to end through the MCP tools. The cart and identity APIs need a linked
// shopper account, which the certification environment doesn't have, so they
// are not exercised here.
//
//	KROGER_CLIENT_ID / KROGER_CLIENT_SECRET   credentials for the target environment (required)
//	KROGER_API_BASE_URL                       defaults to the certification environment
//	KROGER_TEST_ZIP                           zip code to search near (default 45202)
//
// Run with: go test -tags integration -count=1 ./integration/...
package integration

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
	"github.com/philosowaffle/kroger-cart/internal/server"
)

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// newClient builds a client for the live API, skipping the test when no
// credentials are configured.
func newClient(t *testing.T) *kroger.Client {
	t.Helper()
	id, secret := os.Getenv("KROGER_CLIENT_ID"), os.Getenv("KROGER_CLIENT_SECRET")
	if id == "" || secret == "" {
		t.Skip("KROGER_CLIENT_ID and KROGER_CLIENT_SECRET are not set")
	}
	c, err := kroger.New(kroger.Config{
		ClientID:     id,
		ClientSecret: secret,
		RedirectURI:  kroger.DefaultRedirectURI,
		DataDir:      t.TempDir(),
		BaseURL:      env("KROGER_API_BASE_URL", kroger.CertificationBaseURL),
	})
	if err != nil {
		t.Fatalf("kroger.New: %v", err)
	}
	return c
}

func ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return c
}

// nearbyStore returns the first store near KROGER_TEST_ZIP.
func nearbyStore(t *testing.T, c *kroger.Client) kroger.Location {
	t.Helper()
	zip := env("KROGER_TEST_ZIP", "45202")
	locs, err := c.SearchLocations(ctx(t), kroger.LocationQuery{ZipCode: zip, Limit: 5})
	if err != nil {
		t.Fatalf("SearchLocations(%s): %v", zip, err)
	}
	if len(locs) == 0 {
		t.Fatalf("no stores near %s; set KROGER_TEST_ZIP to a zip the environment has data for", zip)
	}
	return locs[0].Lite
}

func TestLocations(t *testing.T) {
	c := newClient(t)
	store := nearbyStore(t, c)
	if store.LocationID == "" || store.Name == "" {
		t.Fatalf("store missing id or name: %+v", store)
	}

	rec, err := c.GetLocation(ctx(t), store.LocationID)
	if err != nil {
		t.Fatalf("GetLocation(%s): %v", store.LocationID, err)
	}
	if rec.Lite.LocationID != store.LocationID {
		t.Errorf("GetLocation returned %q, want %q", rec.Lite.LocationID, store.LocationID)
	}
	full, err := rec.Full()
	if err != nil {
		t.Fatalf("Full: %v", err)
	}
	if full["locationId"] != store.LocationID {
		t.Errorf("full record locationId = %v, want %s", full["locationId"], store.LocationID)
	}
}

func TestChainsAndDepartments(t *testing.T) {
	c := newClient(t)

	chains, err := c.ListChains(ctx(t))
	if err != nil {
		t.Fatalf("ListChains: %v", err)
	}
	if len(chains) == 0 || chains[0].Name == "" {
		t.Fatalf("ListChains returned %+v", chains)
	}
	ch, err := c.GetChain(ctx(t), chains[0].Name)
	if err != nil {
		t.Fatalf("GetChain(%s): %v", chains[0].Name, err)
	}
	if !strings.EqualFold(ch.Name, chains[0].Name) {
		t.Errorf("GetChain returned %q, want %q", ch.Name, chains[0].Name)
	}

	deps, err := c.ListDepartments(ctx(t))
	if err != nil {
		t.Fatalf("ListDepartments: %v", err)
	}
	if len(deps) == 0 || deps[0].DepartmentID == "" {
		t.Fatalf("ListDepartments returned %+v", deps)
	}
	d, err := c.GetDepartment(ctx(t), deps[0].DepartmentID)
	if err != nil {
		t.Fatalf("GetDepartment(%s): %v", deps[0].DepartmentID, err)
	}
	if d.DepartmentID != deps[0].DepartmentID {
		t.Errorf("GetDepartment returned %q, want %q", d.DepartmentID, deps[0].DepartmentID)
	}
}

func TestProducts(t *testing.T) {
	c := newClient(t)
	store := nearbyStore(t, c)

	recs, err := c.SearchProducts(ctx(t), kroger.ProductQuery{Term: "milk", LocationID: store.LocationID, Limit: 5})
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(recs) == 0 {
		t.Fatalf("no products for \"milk\" at store %s", store.LocationID)
	}
	for _, r := range recs {
		if r.Lite.ProductID == "" || r.Lite.Description == "" {
			t.Errorf("product missing id or description: %+v", r.Lite)
		}
	}
	first := recs[0].Lite

	rec, err := c.GetProduct(ctx(t), first.ProductID, store.LocationID)
	if err != nil {
		t.Fatalf("GetProduct(%s): %v", first.ProductID, err)
	}
	if rec.Lite.ProductID != first.ProductID {
		t.Errorf("GetProduct returned %q, want %q", rec.Lite.ProductID, first.ProductID)
	}
	if full, err := rec.Full(); err != nil || full["productId"] != first.ProductID {
		t.Errorf("full record productId = %v (err %v), want %s", full["productId"], err, first.ProductID)
	}

	byID, err := c.SearchProducts(ctx(t), kroger.ProductQuery{ProductIDs: []string{first.ProductID}, LocationID: store.LocationID})
	if err != nil {
		t.Fatalf("SearchProducts by id: %v", err)
	}
	if !slices.ContainsFunc(byID, func(r kroger.ProductRecord) bool { return r.Lite.ProductID == first.ProductID }) {
		t.Errorf("product_ids search did not return %s", first.ProductID)
	}

	// The filter only has to be accepted; whether anything matches depends on the data.
	if _, err := c.SearchProducts(ctx(t), kroger.ProductQuery{Term: "milk", LocationID: store.LocationID, Fulfillment: "curbside", Limit: 1}); err != nil {
		t.Errorf("SearchProducts with fulfillment filter: %v", err)
	}
}

// TestMCPTools drives the read-only tools through a real MCP session, so
// the live responses are decoded and validated against each tool's schema.
func TestMCPTools(t *testing.T) {
	c := newClient(t)
	store := nearbyStore(t, c)

	clientT, serverT := mcp.NewInMemoryTransports()
	ss, err := server.New(c).Connect(ctx(t), serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "integration", Version: "0"}, nil).Connect(ctx(t), clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	call := func(t *testing.T, name string, args map[string]any) map[string]any {
		t.Helper()
		res, err := cs.CallTool(ctx(t), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError {
			t.Fatalf("%s returned a tool error: %s", name, res.Content[0].(*mcp.TextContent).Text)
		}
		var out map[string]any
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("%s: decode output: %v", name, err)
		}
		return out
	}
	nonEmpty := func(t *testing.T, out map[string]any, key string) {
		t.Helper()
		if list, _ := out[key].([]any); len(list) == 0 {
			t.Errorf("%s is empty: %v", key, out)
		}
	}

	t.Run("search_locations", func(t *testing.T) {
		nonEmpty(t, call(t, "search_locations", map[string]any{"zip_code": env("KROGER_TEST_ZIP", "45202")}), "locations")
	})
	t.Run("get_location full", func(t *testing.T) {
		out := call(t, "get_location", map[string]any{"location_id": store.LocationID, "detail": "full"})
		if out["detail"] != "full" || out["location"] == nil {
			t.Errorf("get_location = %v", out)
		}
	})
	t.Run("list_chains", func(t *testing.T) { nonEmpty(t, call(t, "list_chains", nil), "chains") })
	t.Run("list_departments", func(t *testing.T) { nonEmpty(t, call(t, "list_departments", nil), "departments") })

	var productID string
	t.Run("search_products", func(t *testing.T) {
		out := call(t, "search_products", map[string]any{"term": "eggs", "location_id": store.LocationID, "limit": 3})
		nonEmpty(t, out, "products")
		if list, _ := out["products"].([]any); len(list) > 0 {
			productID, _ = list[0].(map[string]any)["product_id"].(string)
		}
	})
	t.Run("search_products full", func(t *testing.T) {
		nonEmpty(t, call(t, "search_products", map[string]any{"term": "bread", "location_id": store.LocationID, "limit": 2, "detail": "full"}), "products")
	})
	t.Run("get_product", func(t *testing.T) {
		if productID == "" {
			t.Skip("no product from search_products")
		}
		out := call(t, "get_product", map[string]any{"product_id": productID, "location_id": store.LocationID, "detail": "full"})
		if p, _ := out["product"].(map[string]any); p["productId"] != productID {
			t.Errorf("get_product = %v", out)
		}
	})
	t.Run("kroger_auth_status unlinked", func(t *testing.T) {
		if out := call(t, "kroger_auth_status", nil); out["linked"] != false {
			t.Errorf("kroger_auth_status = %v, want not linked", out)
		}
	})
}
