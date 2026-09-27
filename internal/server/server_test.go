package server

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
	"github.com/philosowaffle/kroger-cart/internal/kroger/krogertest"
)

type harness struct {
	fake   *krogertest.Server
	client *kroger.Client
	cs     *mcp.ClientSession
}

// newHarness connects an MCP client to New() over the SDK's in-memory
// transport, backed by a fake Kroger API.
func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := context.Background()
	fake := krogertest.New(t)
	kc, err := kroger.New(kroger.Config{
		ClientID:     krogertest.ClientID,
		ClientSecret: krogertest.ClientSecret,
		RedirectURI:  "http://localhost:8080/kroger/callback",
		DataDir:      t.TempDir(),
		BaseURL:      fake.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	clientT, serverT := mcp.NewInMemoryTransports()
	ss, err := New(kc).Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return &harness{fake: fake, client: kc, cs: cs}
}

// call invokes a tool and decodes its structured output into out. It
// returns the tool's error text, or "" on success.
func (h *harness) call(t *testing.T, name string, args map[string]any, out any) string {
	t.Helper()
	res, err := h.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		return res.Content[0].(*mcp.TextContent).Text
	}
	if out != nil {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s: decode output: %v", name, err)
		}
	}
	return ""
}

func (h *harness) mustCall(t *testing.T, name string, args map[string]any, out any) {
	t.Helper()
	if msg := h.call(t, name, args, out); msg != "" {
		t.Fatalf("%s returned tool error: %s", name, msg)
	}
}

// link completes the Kroger login the way the shopper would: kroger_login
// hands out a URL, and Kroger redirects back with its state.
func (h *harness) link(t *testing.T) {
	t.Helper()
	var login struct{ URL string }
	h.mustCall(t, "kroger_login", nil, &login)
	u, err := url.Parse(login.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.client.CompleteAuthorization(context.Background(), u.Query().Get("state"), "good-code"); err != nil {
		t.Fatal(err)
	}
}

func TestListTools(t *testing.T) {
	h := newHarness(t)
	res, err := h.cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
		if tool.Description == "" {
			t.Errorf("%s has no description", tool.Name)
		}
	}
	sort.Strings(names)
	want := []string{"add_to_cart", "get_product", "kroger_auth_status", "kroger_login", "search_locations", "search_products", "set_preferred_location"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("tools = %v, want %v", names, want)
	}
}

func TestServerInstructions(t *testing.T) {
	h := newHarness(t)
	if got := h.cs.InitializeResult().Instructions; !strings.Contains(got, "add_to_cart") {
		t.Errorf("instructions missing workflow guidance: %q", got)
	}
}

func TestSearchLocationsAndSetPreferred(t *testing.T) {
	h := newHarness(t)

	var found struct{ Locations []kroger.Location }
	h.mustCall(t, "search_locations", map[string]any{"zip_code": "80218"}, &found)
	if len(found.Locations) != 1 || found.Locations[0].LocationID != krogertest.LocationID {
		t.Fatalf("locations = %+v", found.Locations)
	}

	if msg := h.call(t, "set_preferred_location", map[string]any{"location_id": "bogus"}, nil); !strings.Contains(msg, "404") {
		t.Errorf("unknown store: error = %q, want 404", msg)
	}
	if got := h.client.PreferredLocationID(); got != "" {
		t.Errorf("unknown store was saved: %q", got)
	}

	var set struct{ Location kroger.Location }
	h.mustCall(t, "set_preferred_location", map[string]any{"location_id": krogertest.LocationID}, &set)
	if set.Location.Name != "King Soopers - Speer" {
		t.Errorf("location = %+v", set.Location)
	}
	if got := h.client.PreferredLocationID(); got != krogertest.LocationID {
		t.Errorf("preferred = %q", got)
	}
}

func TestSearchLocationsRequiresZip(t *testing.T) {
	h := newHarness(t)
	if msg := h.call(t, "search_locations", map[string]any{}, nil); msg == "" {
		t.Error("want tool error without zip_code")
	}
}

func TestSearchProductsUsesPreferredStore(t *testing.T) {
	h := newHarness(t)

	var unscoped struct {
		LocationID string `json:"location_id"`
		Products   []kroger.Product
	}
	h.mustCall(t, "search_products", map[string]any{"term": "milk"}, &unscoped)
	if unscoped.LocationID != "" || unscoped.Products[0].RegularPrice != 0 {
		t.Errorf("unscoped search = %+v", unscoped)
	}

	if err := h.client.SetPreferredLocationID(krogertest.LocationID); err != nil {
		t.Fatal(err)
	}
	var scoped struct {
		LocationID string `json:"location_id"`
		Products   []kroger.Product
	}
	h.mustCall(t, "search_products", map[string]any{"term": "milk"}, &scoped)
	if scoped.LocationID != krogertest.LocationID {
		t.Errorf("location_id = %q", scoped.LocationID)
	}
	if p := scoped.Products[0]; p.UPC != krogertest.ProductID || p.RegularPrice != 4.29 || p.StockLevel != "HIGH" {
		t.Errorf("product = %+v", p)
	}

	var empty struct{ Products []kroger.Product }
	h.mustCall(t, "search_products", map[string]any{"term": "nothing"}, &empty)
	if len(empty.Products) != 0 {
		t.Errorf("products = %+v", empty.Products)
	}
}

func TestGetProduct(t *testing.T) {
	h := newHarness(t)
	var got struct{ Product kroger.Product }
	h.mustCall(t, "get_product", map[string]any{"product_id": krogertest.ProductID, "location_id": krogertest.LocationID}, &got)
	if got.Product.PromoPrice != 3.49 {
		t.Errorf("product = %+v", got.Product)
	}
}

func TestAccountAndCartFlow(t *testing.T) {
	h := newHarness(t)

	var status struct {
		Linked    bool
		ProfileID string `json:"profile_id"`
		Message   string
	}
	h.mustCall(t, "kroger_auth_status", nil, &status)
	if status.Linked || !strings.Contains(status.Message, "kroger_login") {
		t.Errorf("status before link = %+v", status)
	}

	items := map[string]any{"items": []any{map[string]any{"upc": krogertest.ProductID}}}
	if msg := h.call(t, "add_to_cart", items, nil); !strings.Contains(msg, "kroger_login") {
		t.Errorf("add_to_cart before link: error = %q, want pointer to kroger_login", msg)
	}

	h.link(t)

	h.mustCall(t, "kroger_auth_status", nil, &status)
	if !status.Linked || status.ProfileID != krogertest.ProfileID {
		t.Errorf("status after link = %+v", status)
	}

	var added struct{ Added []kroger.CartItem }
	h.mustCall(t, "add_to_cart", items, &added)
	want := kroger.CartItem{UPC: krogertest.ProductID, Quantity: 1, Modality: kroger.ModalityPickup}
	if len(added.Added) != 1 || added.Added[0] != want {
		t.Errorf("added = %+v, want [%+v]", added.Added, want)
	}
	if adds := h.fake.CartAdds(); len(adds) != 1 || adds[0].Items[0].Quantity != 1 {
		t.Errorf("cart API calls = %+v", adds)
	}

	bad := map[string]any{"items": []any{map[string]any{"upc": "1", "modality": "SHIP"}}}
	if msg := h.call(t, "add_to_cart", bad, nil); !strings.Contains(msg, "modality") {
		t.Errorf("bad modality: error = %q", msg)
	}
}

func TestExpiredLinkReportedByStatus(t *testing.T) {
	h := newHarness(t)
	h.link(t)
	h.fake.RejectRefresh = true
	h.fake.ExpiresIn = 0
	// Force a refresh by linking again with a token that is already expired.
	h.link(t)

	var status struct {
		Linked  bool
		Message string
	}
	h.mustCall(t, "kroger_auth_status", nil, &status)
	if status.Linked || !strings.Contains(status.Message, "expired") {
		t.Errorf("status = %+v", status)
	}
}
