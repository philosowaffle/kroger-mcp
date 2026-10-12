// Package products provides the catalog tools: search_products and
// get_product.
package products

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
)

const detailDoc = `"lite" (default): upc, brand, description, size, price, promo, stock, aisle, fulfillment. "full": Kroger's complete record, adding images, all item and price fields, shelf position, dimensions, temperature and nutrition where Kroger provides it. Use full only when those extras are needed; it is much larger`

// SearchArgs are the inputs to search_products.
type SearchArgs struct {
	Term        string   `json:"term,omitempty" jsonschema:"what to search for, e.g. \"2% milk\" or \"sourdough bread\". One of term, product_ids or brand is required"`
	ProductIDs  []string `json:"product_ids,omitempty" jsonschema:"look up these product ids or UPCs in one call"`
	Brand       string   `json:"brand,omitempty" jsonschema:"only return this brand, e.g. \"Simple Truth\". On its own it searches for the brand's products"`
	LocationID  string   `json:"location_id,omitempty" jsonschema:"store to search; defaults to the preferred store. Prices, stock and aisle are only returned when a store is set"`
	Fulfillment string   `json:"fulfillment,omitempty" jsonschema:"only items available this way at the store: in_store, curbside, delivery or ship_to_home"`
	Limit       int      `json:"limit,omitempty" jsonschema:"maximum results, 1-50 (default 10)"`
	Start       int      `json:"start,omitempty" jsonschema:"number of results to skip, for paging"`
	Detail      string   `json:"detail,omitempty" jsonschema:"lite or full"`
}

// SearchResult is the output of search_products.
type SearchResult struct {
	// LocationID is the store the search was scoped to, if any.
	LocationID string `json:"location_id,omitempty"`
	Detail     string `json:"detail"`
	// Products holds lite products, or full Kroger records when detail is full.
	Products []any `json:"products"`
}

// GetArgs are the inputs to get_product.
type GetArgs struct {
	ProductID  string `json:"product_id" jsonschema:"product_id or UPC from search_products"`
	LocationID string `json:"location_id,omitempty" jsonschema:"store for price and stock; defaults to the preferred store"`
	Detail     string `json:"detail,omitempty" jsonschema:"lite or full"`
}

// GetResult is the output of get_product.
type GetResult struct {
	LocationID string `json:"location_id,omitempty"`
	Detail     string `json:"detail"`
	Product    any    `json:"product"`
}

// AddTools registers the product tools.
func AddTools(s *mcp.Server, c *kroger.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "search_products",
		Description: "Search the Kroger catalog by term, brand or product ids, optionally only items available for a given fulfillment. " +
			"Returns each product's upc (needed for add_to_cart). Scoped to a store it adds price, promo price, stock level, aisle and fulfillment options. " +
			"detail: " + detailDoc + ".",
		Annotations: &mcp.ToolAnnotations{Title: "Search products", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, SearchResult, error) {
		full, err := kroger.WantFull(args.Detail)
		if err != nil {
			return nil, SearchResult{}, err
		}
		loc := c.ResolveLocationID(args.LocationID)
		recs, err := c.SearchProducts(ctx, kroger.ProductQuery{
			Term:        args.Term,
			ProductIDs:  args.ProductIDs,
			Brand:       args.Brand,
			LocationID:  loc,
			Fulfillment: args.Fulfillment,
			Limit:       args.Limit,
			Start:       args.Start,
		})
		if err != nil {
			return nil, SearchResult{}, err
		}
		res := SearchResult{LocationID: loc, Detail: detailName(full), Products: make([]any, 0, len(recs))}
		for _, r := range recs {
			p, err := render(r, full)
			if err != nil {
				return nil, SearchResult{}, err
			}
			res.Products = append(res.Products, p)
		}
		return nil, res, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_product",
		Description: "Get one product by product_id or UPC, with store price and stock when a store is set. " +
			"Products seen in the last 5 minutes are served from cache, so asking for detail=full on a search result is cheap. " +
			"detail: " + detailDoc + ".",
		Annotations: &mcp.ToolAnnotations{Title: "Get product", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetArgs) (*mcp.CallToolResult, GetResult, error) {
		full, err := kroger.WantFull(args.Detail)
		if err != nil {
			return nil, GetResult{}, err
		}
		loc := c.ResolveLocationID(args.LocationID)
		rec, err := c.GetProduct(ctx, args.ProductID, loc)
		if err != nil {
			return nil, GetResult{}, err
		}
		p, err := render(*rec, full)
		if err != nil {
			return nil, GetResult{}, err
		}
		return nil, GetResult{LocationID: loc, Detail: detailName(full), Product: p}, nil
	})
}

func render(r kroger.ProductRecord, full bool) (any, error) {
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
