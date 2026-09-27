package kroger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Location is the lite view of a Kroger-family store. The full record adds
// hours, departments, geolocation, store and division numbers.
type Location struct {
	LocationID string `json:"location_id"`
	Chain      string `json:"chain,omitempty"`
	Name       string `json:"name"`
	Address    string `json:"address,omitempty"`
	Phone      string `json:"phone,omitempty"`
}

// LocationRecord is one store as Kroger returned it, plus its lite view.
type LocationRecord struct {
	Lite Location
	Raw  json.RawMessage
}

// Full decodes the complete Kroger record.
func (r LocationRecord) Full() (map[string]any, error) {
	return decodeObject(r.Raw)
}

// LocationQuery filters a store search. One of ZipCode, a Latitude/Longitude
// pair, or LocationIDs is required.
type LocationQuery struct {
	ZipCode   string
	Latitude  *float64
	Longitude *float64
	// LocationIDs looks up specific stores.
	LocationIDs []string
	RadiusMiles int
	Chain       string
	// DepartmentIDs only returns stores that have these departments (see
	// ListDepartments).
	DepartmentIDs []string
	Limit         int
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

func newLocationRecord(raw json.RawMessage) (LocationRecord, error) {
	var a apiLocation
	if err := json.Unmarshal(raw, &a); err != nil {
		return LocationRecord{}, fmt.Errorf("kroger api: decode location: %w", err)
	}
	return LocationRecord{Lite: a.toLocation(), Raw: raw}, nil
}

// SearchLocations finds stores near a zip code or coordinates, or by id.
// Coordinates take precedence over a zip code when both are given.
func (c *Client) SearchLocations(ctx context.Context, q LocationQuery) ([]LocationRecord, error) {
	params := url.Values{}
	zip := strings.TrimSpace(q.ZipCode)
	ids := cleanList(q.LocationIDs)
	switch {
	case (q.Latitude == nil) != (q.Longitude == nil):
		return nil, errors.New("latitude and longitude must be given together")
	case q.Latitude != nil:
		if *q.Latitude < -90 || *q.Latitude > 90 || *q.Longitude < -180 || *q.Longitude > 180 {
			return nil, errors.New("latitude must be within ±90 and longitude within ±180")
		}
		params.Set("filter.latLong.near", strconv.FormatFloat(*q.Latitude, 'f', -1, 64)+","+strconv.FormatFloat(*q.Longitude, 'f', -1, 64))
	case zip != "":
		params.Set("filter.zipCode.near", zip)
	case len(ids) == 0:
		return nil, errors.New("one of zip_code, latitude/longitude or location_ids is required")
	}
	if len(ids) > 0 {
		params.Set("filter.locationId", strings.Join(ids, ","))
	}
	if q.RadiusMiles > 0 {
		params.Set("filter.radiusInMiles", strconv.Itoa(clamp(q.RadiusMiles, 1, 100, 10)))
	}
	if chain := strings.TrimSpace(q.Chain); chain != "" {
		params.Set("filter.chain", chain)
	}
	if deps := cleanList(q.DepartmentIDs); len(deps) > 0 {
		params.Set("filter.department", strings.Join(deps, ","))
	}
	params.Set("filter.limit", strconv.Itoa(clamp(q.Limit, 1, 200, 10)))

	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := c.get(ctx, tok, "/locations", params, &resp); err != nil {
		return nil, err
	}
	out := make([]LocationRecord, 0, len(resp.Data))
	for _, raw := range resp.Data {
		rec, err := newLocationRecord(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

// GetLocation looks up one store by id.
func (c *Client) GetLocation(ctx context.Context, id string) (*LocationRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("location_id is required")
	}
	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.get(ctx, tok, "/locations/"+url.PathEscape(id), nil, &resp); err != nil {
		return nil, err
	}
	rec, err := newLocationRecord(resp.Data)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// Chain is a Kroger-family banner (e.g. the value to pass as a location
// search's chain filter).
type Chain struct {
	Name            string   `json:"name"`
	DivisionNumbers []string `json:"division_numbers,omitempty"`
}

type apiChain struct {
	Name            string   `json:"name"`
	DivisionNumbers []string `json:"divisionNumbers"`
}

// ListChains returns every chain.
func (c *Client) ListChains(ctx context.Context) ([]Chain, error) {
	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []apiChain `json:"data"`
	}
	if err := c.get(ctx, tok, "/chains", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Chain, 0, len(resp.Data))
	for _, ch := range resp.Data {
		out = append(out, Chain(ch))
	}
	return out, nil
}

// GetChain looks up one chain by name.
func (c *Client) GetChain(ctx context.Context, name string) (*Chain, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("chain name is required")
	}
	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data apiChain `json:"data"`
	}
	if err := c.get(ctx, tok, "/chains/"+url.PathEscape(name), nil, &resp); err != nil {
		return nil, err
	}
	ch := Chain(resp.Data)
	return &ch, nil
}

// Department is a store department (bakery, pharmacy, ...). Its id is what
// a location search's department filter takes.
type Department struct {
	DepartmentID string `json:"department_id"`
	Name         string `json:"name"`
}

type apiDepartment struct {
	DepartmentID string `json:"departmentId"`
	Name         string `json:"name"`
}

// ListDepartments returns every department.
func (c *Client) ListDepartments(ctx context.Context) ([]Department, error) {
	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []apiDepartment `json:"data"`
	}
	if err := c.get(ctx, tok, "/departments", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Department, 0, len(resp.Data))
	for _, d := range resp.Data {
		out = append(out, Department(d))
	}
	return out, nil
}

// GetDepartment looks up one department by id.
func (c *Client) GetDepartment(ctx context.Context, id string) (*Department, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("department_id is required")
	}
	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data apiDepartment `json:"data"`
	}
	if err := c.get(ctx, tok, "/departments/"+url.PathEscape(id), nil, &resp); err != nil {
		return nil, err
	}
	d := Department(resp.Data)
	return &d, nil
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
