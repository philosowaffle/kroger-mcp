// Package locations provides the store tools: search_locations,
// get_location, set_preferred_location, list_chains and list_departments.
package locations

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-mcp/internal/kroger"
)

const detailDoc = `"lite" (default): id, chain, name, address, phone. "full": Kroger's complete record, adding store hours, departments (with their own hours and phone numbers), geolocation, and store and division numbers`

// SearchArgs are the inputs to search_locations.
type SearchArgs struct {
	ZipCode       string   `json:"zip_code,omitempty" jsonschema:"zip code to search near. One of zip_code, latitude+longitude or location_ids is required"`
	Latitude      *float64 `json:"latitude,omitempty" jsonschema:"search near this point instead of a zip code (with longitude)"`
	Longitude     *float64 `json:"longitude,omitempty" jsonschema:"search near this point instead of a zip code (with latitude)"`
	LocationIDs   []string `json:"location_ids,omitempty" jsonschema:"look up these stores by location_id"`
	RadiusMiles   int      `json:"radius_miles,omitempty" jsonschema:"search radius in miles, 1-100 (Kroger's default is 10)"`
	Chain         string   `json:"chain,omitempty" jsonschema:"only this chain; list_chains gives valid names"`
	DepartmentIDs []string `json:"department_ids,omitempty" jsonschema:"only stores with these departments, e.g. a pharmacy; list_departments gives ids"`
	Limit         int      `json:"limit,omitempty" jsonschema:"maximum results, 1-200 (default 10)"`
	Detail        string   `json:"detail,omitempty" jsonschema:"lite or full"`
}

// SearchResult is the output of search_locations.
type SearchResult struct {
	Detail string `json:"detail"`
	// Locations holds lite locations, or full Kroger records when detail is full.
	Locations []any `json:"locations"`
}

// GetArgs are the inputs to get_location.
type GetArgs struct {
	LocationID string `json:"location_id" jsonschema:"store location_id"`
	Detail     string `json:"detail,omitempty" jsonschema:"lite or full"`
}

// GetResult is the output of get_location.
type GetResult struct {
	Detail   string `json:"detail"`
	Location any    `json:"location"`
}

// SetPreferredArgs are the inputs to set_preferred_location.
type SetPreferredArgs struct {
	LocationID string `json:"location_id" jsonschema:"store location_id from search_locations"`
}

// SetPreferredResult is the output of set_preferred_location.
type SetPreferredResult struct {
	Location kroger.Location `json:"location"`
}

// ChainsArgs are the inputs to list_chains.
type ChainsArgs struct {
	Name string `json:"name,omitempty" jsonschema:"look up just this chain"`
}

// ChainsResult is the output of list_chains.
type ChainsResult struct {
	Chains []kroger.Chain `json:"chains"`
}

// DepartmentsArgs are the inputs to list_departments.
type DepartmentsArgs struct {
	DepartmentID string `json:"department_id,omitempty" jsonschema:"look up just this department"`
}

// DepartmentsResult is the output of list_departments.
type DepartmentsResult struct {
	Departments []kroger.Department `json:"departments"`
}

// AddTools registers the location tools.
func AddTools(s *mcp.Server, c *kroger.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "search_locations",
		Description: "Find Kroger-family grocery stores (King Soopers, Kroger, Ralphs, Fred Meyer, etc.) near a zip code or coordinates, or look stores up by id. " +
			"Can filter by chain and by departments (e.g. only stores with a pharmacy). Returns each store's location_id, which scopes product searches to that store's prices and stock. " +
			"detail: " + detailDoc + ".",
		Annotations: &mcp.ToolAnnotations{Title: "Search stores", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, SearchResult, error) {
		full, err := kroger.WantFull(args.Detail)
		if err != nil {
			return nil, SearchResult{}, err
		}
		recs, err := c.SearchLocations(ctx, kroger.LocationQuery{
			ZipCode:       args.ZipCode,
			Latitude:      args.Latitude,
			Longitude:     args.Longitude,
			LocationIDs:   args.LocationIDs,
			RadiusMiles:   args.RadiusMiles,
			Chain:         args.Chain,
			DepartmentIDs: args.DepartmentIDs,
			Limit:         args.Limit,
		})
		if err != nil {
			return nil, SearchResult{}, err
		}
		res := SearchResult{Detail: detailName(full), Locations: make([]any, 0, len(recs))}
		for _, r := range recs {
			l, err := render(r, full)
			if err != nil {
				return nil, SearchResult{}, err
			}
			res.Locations = append(res.Locations, l)
		}
		return nil, res, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_location",
		Description: "Get one store by location_id. detail: " + detailDoc + ".",
		Annotations: &mcp.ToolAnnotations{Title: "Get store", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetArgs) (*mcp.CallToolResult, GetResult, error) {
		full, err := kroger.WantFull(args.Detail)
		if err != nil {
			return nil, GetResult{}, err
		}
		rec, err := c.GetLocation(ctx, args.LocationID)
		if err != nil {
			return nil, GetResult{}, err
		}
		l, err := render(*rec, full)
		if err != nil {
			return nil, GetResult{}, err
		}
		return nil, GetResult{Detail: detailName(full), Location: l}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_preferred_location",
		Description: "Save a store as the default for product searches, so later calls can omit location_id. Persists across restarts.",
		Annotations: &mcp.ToolAnnotations{Title: "Set preferred store", IdempotentHint: true, DestructiveHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SetPreferredArgs) (*mcp.CallToolResult, SetPreferredResult, error) {
		rec, err := c.GetLocation(ctx, args.LocationID)
		if err != nil {
			return nil, SetPreferredResult{}, fmt.Errorf("look up location %q: %w", args.LocationID, err)
		}
		if err := c.SetPreferredLocationID(rec.Lite.LocationID); err != nil {
			return nil, SetPreferredResult{}, err
		}
		return nil, SetPreferredResult{Location: rec.Lite}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_chains",
		Description: "List the Kroger-family chains (the names search_locations' chain filter accepts), or look one up by name.",
		Annotations: &mcp.ToolAnnotations{Title: "List chains", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ChainsArgs) (*mcp.CallToolResult, ChainsResult, error) {
		if args.Name != "" {
			ch, err := c.GetChain(ctx, args.Name)
			if err != nil {
				return nil, ChainsResult{}, err
			}
			return nil, ChainsResult{Chains: []kroger.Chain{*ch}}, nil
		}
		chains, err := c.ListChains(ctx)
		if err != nil {
			return nil, ChainsResult{}, err
		}
		return nil, ChainsResult{Chains: chains}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_departments",
		Description: "List store departments (bakery, deli, pharmacy, ...) with the ids search_locations' department filter takes, or look one up by id.",
		Annotations: &mcp.ToolAnnotations{Title: "List departments", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DepartmentsArgs) (*mcp.CallToolResult, DepartmentsResult, error) {
		if args.DepartmentID != "" {
			d, err := c.GetDepartment(ctx, args.DepartmentID)
			if err != nil {
				return nil, DepartmentsResult{}, err
			}
			return nil, DepartmentsResult{Departments: []kroger.Department{*d}}, nil
		}
		deps, err := c.ListDepartments(ctx)
		if err != nil {
			return nil, DepartmentsResult{}, err
		}
		return nil, DepartmentsResult{Departments: deps}, nil
	})
}

func render(r kroger.LocationRecord, full bool) (any, error) {
	if full {
		return r.Full()
	}
	return r.Lite, nil
}

func detailName(full bool) string {
	if full {
		return kroger.DetailFull
	}
	return kroger.DetailLite
}
