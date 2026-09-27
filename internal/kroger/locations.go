package kroger

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// Location is a Kroger-family store.
type Location struct {
	LocationID string `json:"location_id"`
	Chain      string `json:"chain,omitempty"`
	Name       string `json:"name"`
	Address    string `json:"address,omitempty"`
	Phone      string `json:"phone,omitempty"`
}

// LocationQuery filters a store search. ZipCode is required.
type LocationQuery struct {
	ZipCode     string
	RadiusMiles int
	Chain       string
	Limit       int
}

type apiLocation struct {
	LocationID string `json:"locationId"`
	Chain      string `json:"chain"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Address    struct {
		AddressLine1 string `json:"addressLine1"`
		City         string `json:"city"`
		State        string `json:"state"`
		ZipCode      string `json:"zipCode"`
	} `json:"address"`
}

func (a apiLocation) toLocation() Location {
	// "123 Main St, Denver, CO 80202", skipping any parts Kroger left blank.
	var parts []string
	for _, p := range []string{a.Address.AddressLine1, a.Address.City, strings.TrimSpace(a.Address.State + " " + a.Address.ZipCode)} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return Location{
		LocationID: a.LocationID,
		Chain:      a.Chain,
		Name:       a.Name,
		Address:    strings.Join(parts, ", "),
		Phone:      a.Phone,
	}
}

// SearchLocations finds stores near a zip code.
func (c *Client) SearchLocations(ctx context.Context, q LocationQuery) ([]Location, error) {
	zip := strings.TrimSpace(q.ZipCode)
	if zip == "" {
		return nil, errors.New("zip_code is required")
	}
	params := url.Values{"filter.zipCode.near": {zip}}
	if q.RadiusMiles > 0 {
		params.Set("filter.radiusInMiles", strconv.Itoa(q.RadiusMiles))
	}
	if chain := strings.TrimSpace(q.Chain); chain != "" {
		params.Set("filter.chain", chain)
	}
	params.Set("filter.limit", strconv.Itoa(clamp(q.Limit, 1, 200, 10)))

	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []apiLocation `json:"data"`
	}
	if err := c.get(ctx, tok, "/locations", params, &resp); err != nil {
		return nil, err
	}
	out := make([]Location, 0, len(resp.Data))
	for _, l := range resp.Data {
		out = append(out, l.toLocation())
	}
	return out, nil
}

// GetLocation looks up one store by id.
func (c *Client) GetLocation(ctx context.Context, id string) (*Location, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("location_id is required")
	}
	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data apiLocation `json:"data"`
	}
	if err := c.get(ctx, tok, "/locations/"+url.PathEscape(id), nil, &resp); err != nil {
		return nil, err
	}
	l := resp.Data.toLocation()
	return &l, nil
}

// clamp returns def when v is unset (<= 0), otherwise v bounded to [lo, hi].
func clamp(v, lo, hi, def int) int {
	switch {
	case v <= 0:
		return def
	case v < lo:
		return lo
	case v > hi:
		return hi
	default:
		return v
	}
}
