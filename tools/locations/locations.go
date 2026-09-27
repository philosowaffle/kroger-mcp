// Package locations provides the store lookup tools: search_locations and
// set_preferred_location.
package locations

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
)

// SearchArgs are the inputs to search_locations.
type SearchArgs struct {
	ZipCode     string `json:"zip_code" jsonschema:"zip code to search near"`
	RadiusMiles int    `json:"radius_miles,omitempty" jsonschema:"search radius in miles (Kroger's default is 10)"`
	Chain       string `json:"chain,omitempty" jsonschema:"only return stores of this chain; use a chain value from earlier results"`
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum results, 1-200 (default 10)"`
}

// SearchResult is the output of search_locations.
type SearchResult struct {
	Locations []kroger.Location `json:"locations"`
}

// SetPreferredArgs are the inputs to set_preferred_location.
type SetPreferredArgs struct {
	LocationID string `json:"location_id" jsonschema:"store location_id from search_locations"`
}

// SetPreferredResult is the output of set_preferred_location.
type SetPreferredResult struct {
	Location kroger.Location `json:"location"`
}

// AddTools registers the location tools.
func AddTools(s *mcp.Server, c *kroger.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_locations",
		Description: "Find Kroger-family grocery stores (King Soopers, Kroger, Ralphs, Fred Meyer, etc.) near a zip code. Returns each store's location_id, which scopes product searches to that store's prices and stock.",
		Annotations: &mcp.ToolAnnotations{Title: "Search stores", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, SearchResult, error) {
		locs, err := c.SearchLocations(ctx, kroger.LocationQuery{
			ZipCode:     args.ZipCode,
			RadiusMiles: args.RadiusMiles,
			Chain:       args.Chain,
			Limit:       args.Limit,
		})
		if err != nil {
			return nil, SearchResult{}, err
		}
		return nil, SearchResult{Locations: locs}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_preferred_location",
		Description: "Save a store as the default for product searches, so later calls can omit location_id. Persists across restarts.",
		Annotations: &mcp.ToolAnnotations{Title: "Set preferred store", IdempotentHint: true, DestructiveHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SetPreferredArgs) (*mcp.CallToolResult, SetPreferredResult, error) {
		loc, err := c.GetLocation(ctx, args.LocationID)
		if err != nil {
			return nil, SetPreferredResult{}, fmt.Errorf("look up location %q: %w", args.LocationID, err)
		}
		if err := c.SetPreferredLocationID(loc.LocationID); err != nil {
			return nil, SetPreferredResult{}, err
		}
		return nil, SetPreferredResult{Location: *loc}, nil
	})
}
