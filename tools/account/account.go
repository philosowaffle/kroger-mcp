// Package account provides the Kroger account-linking tools: kroger_login and
// kroger_auth_status.
package account

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-cart/internal/kroger"
)

// LoginResult is the output of kroger_login.
type LoginResult struct {
	URL          string `json:"url"`
	Instructions string `json:"instructions"`
	// Warning is set when an account is already linked: completing this
	// login replaces it.
	Warning string `json:"warning,omitempty"`
}

// StatusResult is the output of kroger_auth_status.
type StatusResult struct {
	Linked bool `json:"linked"`
	// ProfileID identifies the linked shopper; set only when Linked.
	ProfileID string `json:"profile_id,omitempty"`
	// PreferredLocationID is the store used when a tool call names none.
	PreferredLocationID string `json:"preferred_location_id,omitempty"`
	Message             string `json:"message"`
}

// AddTools registers the account tools.
func AddTools(s *mcp.Server, c *kroger.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "kroger_login",
		Description: "Start linking the shopper's Kroger account, which add_to_cart needs. Returns a single-use URL for the shopper to open in a browser and sign in with their Kroger (or King Soopers, Ralphs, etc.) account. The link expires after 15 minutes. If an account is already linked, signing in replaces it; the result warns when that's the case. Searching products and stores works without linking.",
		Annotations: &mcp.ToolAnnotations{Title: "Link Kroger account", DestructiveHint: new(false)},
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, LoginResult, error) {
		u, err := c.AuthorizeURL()
		if err != nil {
			return nil, LoginResult{}, err
		}
		res := LoginResult{
			URL:          u,
			Instructions: "Open this URL, sign in to Kroger and approve access. When the page says the account is linked, come back here; kroger_auth_status confirms it.",
		}
		if c.Linked() {
			res.Warning = "A Kroger account is already linked. Signing in with this URL replaces it, and future cart adds go to whichever account signs in. Only continue if the shopper means to switch or re-link their own account."
			if id := c.LinkedProfileID(); id != "" {
				res.Warning += " Currently linked profile: " + id + "."
			}
		}
		return nil, res, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "kroger_auth_status",
		Description: "Check whether a Kroger account is linked and which store is the default.",
		Annotations: &mcp.ToolAnnotations{Title: "Kroger account status", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, StatusResult, error) {
		res := StatusResult{PreferredLocationID: c.ResolveLocationID("")}
		if !c.Linked() {
			res.Message = "No Kroger account is linked. Call kroger_login to link one."
			return nil, res, nil
		}
		p, err := c.GetProfile(ctx)
		if errors.Is(err, kroger.ErrNotLinked) {
			res.Message = "The Kroger link has expired. Call kroger_login to link again."
			return nil, res, nil
		}
		if err != nil {
			return nil, StatusResult{}, err
		}
		res.Linked = true
		res.ProfileID = p.ID
		res.Message = "A Kroger account is linked."
		return nil, res, nil
	})
}
