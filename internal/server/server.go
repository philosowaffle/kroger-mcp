// Package server builds the MCP server and is the one place tools are
// registered. To add a tool, create a package under tools/ and call its
// registration function from New.
package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-mcp/internal/kroger"
	"github.com/philosowaffle/kroger-mcp/tools/account"
	"github.com/philosowaffle/kroger-mcp/tools/cart"
	"github.com/philosowaffle/kroger-mcp/tools/locations"
	"github.com/philosowaffle/kroger-mcp/tools/products"
)

// Name identifies this server to MCP clients.
const Name = "kroger-mcp"

// Version and Commit are set at build time with
//
//	-ldflags "-X github.com/philosowaffle/kroger-mcp/internal/server.Version=v1.2.3
//	          -X github.com/philosowaffle/kroger-mcp/internal/server.Commit=abc1234"
//
// by the Taskfile, Dockerfile and release workflow. Version is reported to MCP
// clients as serverInfo.version.
var (
	Version = "dev"
	Commit  = "unknown"
)

const instructions = `Tools for grocery shopping at Kroger-family stores (King Soopers, Kroger, Ralphs, Fred Meyer, and others).

Typical flow: find the shopper's store with search_locations and save it with set_preferred_location; look items up with search_products; confirm choices with the shopper; then add_to_cart by UPC. add_to_cart needs a linked Kroger account: if it reports the account isn't linked, call kroger_login and give the shopper the URL.

Product and store tools return a compact "lite" view by default. Pass detail="full" only when you need more (images, nutrition where Kroger has it, shelf position, store hours, departments); full records are much larger. get_product serves products seen in the last 5 minutes from cache, so fetching full details for a search result is cheap.

Use list_chains for valid chain names and list_departments for department ids before filtering search_locations by them.

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
