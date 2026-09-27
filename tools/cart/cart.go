// Package cart provides the add_to_cart tool.
package cart

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
)

// Item is one line to add.
type Item struct {
	UPC      string `json:"upc" jsonschema:"product upc from search_products"`
	Quantity int    `json:"quantity,omitempty" jsonschema:"how many to add (default 1)"`
	Modality string `json:"modality,omitempty" jsonschema:"PICKUP or DELIVERY; defaults to the server's configured modality"`
}

// AddArgs are the inputs to add_to_cart.
type AddArgs struct {
	Items []Item `json:"items" jsonschema:"items to add"`
}

// AddResult is the output of add_to_cart.
type AddResult struct {
	Added []kroger.CartItem `json:"added"`
	Note  string            `json:"note"`
}

const addNote = "Items were added to the shopper's Kroger cart. The Kroger API cannot read or remove cart items, so review and check out on the store's website or app."

// AddTool registers add_to_cart.
func AddTool(s *mcp.Server, c *kroger.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_to_cart",
		Description: "Add products to the shopper's Kroger cart by UPC. Requires a linked Kroger account (see kroger_login). The cart is add-only through this API: confirm choices with the shopper before adding, because items can only be removed on the store's website or app. Does not check out.",
		Annotations: &mcp.ToolAnnotations{Title: "Add to cart", DestructiveHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args AddArgs) (*mcp.CallToolResult, AddResult, error) {
		items := make([]kroger.CartItem, len(args.Items))
		for i, it := range args.Items {
			if it.Quantity == 0 {
				it.Quantity = 1
			}
			items[i] = kroger.CartItem{UPC: it.UPC, Quantity: it.Quantity, Modality: it.Modality}
		}
		added, err := c.AddToCart(ctx, items)
		if err != nil {
			return nil, AddResult{}, err
		}
		return nil, AddResult{Added: added, Note: addNote}, nil
	})
}
