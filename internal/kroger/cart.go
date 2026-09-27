package kroger

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Modalities are the fulfillment types the cart accepts.
const (
	ModalityPickup   = "PICKUP"
	ModalityDelivery = "DELIVERY"
)

// CartItem is one line to add to the cart.
type CartItem struct {
	UPC      string `json:"upc"`
	Quantity int    `json:"quantity"`
	Modality string `json:"modality,omitempty"`
}

// AddToCart adds items to the linked shopper's cart. The public API is
// add-only: items can't be read back or removed. Items without a modality use
// the configured default. It returns the items as sent to Kroger.
func (c *Client) AddToCart(ctx context.Context, items []CartItem) ([]CartItem, error) {
	if len(items) == 0 {
		return nil, errors.New("at least one item is required")
	}
	norm := make([]CartItem, len(items))
	for i, it := range items {
		it.UPC = strings.TrimSpace(it.UPC)
		if it.UPC == "" {
			return nil, fmt.Errorf("item %d: upc is required", i+1)
		}
		if it.Quantity < 1 {
			return nil, fmt.Errorf("item %d (%s): quantity must be at least 1", i+1, it.UPC)
		}
		it.Modality = strings.ToUpper(strings.TrimSpace(it.Modality))
		switch it.Modality {
		case "":
			it.Modality = c.cfg.DefaultModality
		case ModalityPickup, ModalityDelivery:
		default:
			return nil, fmt.Errorf("item %d (%s): modality must be PICKUP or DELIVERY, got %q", i+1, it.UPC, it.Modality)
		}
		norm[i] = it
	}

	tok, err := c.userAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	body := struct {
		Items []CartItem `json:"items"`
	}{norm}
	if err := c.sendJSON(ctx, http.MethodPut, tok, "/cart/add", body, nil); err != nil {
		return nil, err
	}
	return norm, nil
}

// Profile identifies the linked shopper.
type Profile struct {
	ID string `json:"id"`
}

// GetProfile returns the linked shopper's profile.
func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	tok, err := c.userAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Profile `json:"data"`
	}
	if err := c.get(ctx, tok, "/identity/profile", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
