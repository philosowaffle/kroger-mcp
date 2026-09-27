// Package products provides the catalog tools: search_products and
// get_product.
package products

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
)

// SearchArgs are the inputs to search_products.
type SearchArgs struct {
	Term       string `json:"term" jsonschema:"what to search for, e.g. \"2% milk\" or \"sourdough bread\""`
	LocationID string `json:"location_id,omitempty" jsonschema:"store to search; defaults to the preferred store. Prices, stock and aisle are only returned when a store is set"`
	Brand      string `json:"brand,omitempty" jsonschema:"only return this brand, e.g. \"Simple Truth\""`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum results, 1-50 (default 10)"`
	Start      int    `json:"start,omitempty" jsonschema:"number of results to skip, for paging"`
}

// SearchResult is the output of search_products.
type SearchResult struct {
	// LocationID is the store the search was scoped to, if any.
	LocationID string           `json:"location_id,omitempty"`
	Products   []kroger.Product `json:"products"`
}

// GetArgs are the inputs to get_product.
type GetArgs struct {
	ProductID  string `json:"product_id" jsonschema:"product_id or UPC from search_products"`
	LocationID string `json:"location_id,omitempty" jsonschema:"store for price and stock; defaults to the preferred store"`
}

// GetResult is the output of get_product.
type GetResult struct {
	LocationID string         `json:"location_id,omitempty"`
	Product    kroger.Product `json:"product"`
}

// AddTools registers the product tools.
func AddTools(s *mcp.Server, c *kroger.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_products",
		Description: "Search the Kroger catalog. Returns each product's upc (needed for add_to_cart), brand, description and size, plus price, promo price, stock level, aisle and fulfillment options when scoped to a store.",
		Annotations: &mcp.ToolAnnotations{Title: "Search products", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, SearchResult, error) {
		loc := c.ResolveLocationID(args.LocationID)
		prods, err := c.SearchProducts(ctx, kroger.ProductQuery{
			Term:       args.Term,
			LocationID: loc,
			Brand:      args.Brand,
			Limit:      args.Limit,
			Start:      args.Start,
		})
		if err != nil {
			return nil, SearchResult{}, err
		}
		return nil, SearchResult{LocationID: loc, Products: prods}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_product",
		Description: "Get one product's details by product_id or UPC, including store price and stock when a store is set.",
		Annotations: &mcp.ToolAnnotations{Title: "Get product", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetArgs) (*mcp.CallToolResult, GetResult, error) {
		loc := c.ResolveLocationID(args.LocationID)
		p, err := c.GetProduct(ctx, args.ProductID, loc)
		if err != nil {
			return nil, GetResult{}, err
		}
		return nil, GetResult{LocationID: loc, Product: *p}, nil
	})
}
