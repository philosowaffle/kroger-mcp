//go:build integration

package integration

import (
	"testing"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
)

// TestProbeProductSearchByStore is a temporary diagnostic, not a check: it
// logs whether product search fails at other stores and without a store, to
// tell a store-specific Kroger error (PRODUCT-4005) from an API-wide one.
// It never fails. Remove it once the cause is known.
func TestProbeProductSearchByStore(t *testing.T) {
	c := newClient(t)
	try := func(label string, q kroger.ProductQuery) {
		recs, err := c.SearchProducts(ctx(t), q)
		if err != nil {
			t.Logf("PROBE %s: error: %v", label, err)
			return
		}
		t.Logf("PROBE %s: ok, %d products", label, len(recs))
	}
	try("no store", kroger.ProductQuery{Term: "milk", Limit: 3})
	for _, zip := range []string{"45202", "30301", "98101", "80202"} {
		locs, err := c.SearchLocations(ctx(t), kroger.LocationQuery{ZipCode: zip, Limit: 5})
		if err != nil {
			t.Logf("PROBE zip %s: locations error: %v", zip, err)
			continue
		}
		for _, l := range locs {
			try("zip "+zip+" store "+l.Lite.LocationID+" ("+l.Lite.Name+")", kroger.ProductQuery{Term: "milk", LocationID: l.Lite.LocationID, Limit: 3})
		}
	}
}
