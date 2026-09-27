// Package server builds the MCP server and is the one place tools are
// registered. To add a tool, create a package under tools/ and call its
// registration function from New.
package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
	"github.com/philosowaffle/kroger-cart/tools/account"
	"github.com/philosowaffle/kroger-cart/tools/cart"
	"github.com/philosowaffle/kroger-cart/tools/locations"
	"github.com/philosowaffle/kroger-cart/tools/products"
)

const (
	// Name and Version identify this server to MCP clients.
	Name    = "kroger-mcp"
	Version = "0.1.0"
)

const instructions = `Tools for grocery shopping at Kroger-family stores (King Soopers, Kroger, Ralphs, Fred Meyer, and others).

Typical flow: find the shopper's store with search_locations and save it with set_preferred_location; look items up with search_products; confirm choices with the shopper; then add_to_cart by UPC. add_to_cart needs a linked Kroger account: if it reports the account isn't linked, call kroger_login and give the shopper the URL.

The Kroger API can add to the cart but cannot read, change or remove cart items, and cannot check out. Shoppers finish on the store's website or app.`

// New builds the MCP server with every tool registered against c.
func New(c *kroger.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: Name, Version: Version}, &mcp.ServerOptions{
		Instructions: instructions,
	})

	account.AddTools(s, c)
	locations.AddTools(s, c)
	products.AddTools(s, c)
	cart.AddTool(s, c)

	return s
}
