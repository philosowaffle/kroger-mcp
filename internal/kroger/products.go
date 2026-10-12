package kroger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Product is the lite view of a catalog item: what's useful when shopping,
// kept small for Claude's context. Price, stock level and aisle are only
// present when the lookup is scoped to a store. For everything else Kroger
// returns (images, shelf position, dimensions, nutrition where available),
// use the record's full form.
type Product struct {
	ProductID    string   `json:"product_id"`
	UPC          string   `json:"upc"`
	Brand        string   `json:"brand,omitempty"`
	Description  string   `json:"description"`
	Size         string   `json:"size,omitempty"`
	Categories   []string `json:"categories,omitempty"`
	RegularPrice float64  `json:"regular_price,omitempty"`
	// PromoPrice is the sale price; omitted when there is no promotion.
	PromoPrice float64 `json:"promo_price,omitempty"`
	// StockLevel is HIGH, LOW or TEMPORARILY_OUT_OF_STOCK.
	StockLevel string `json:"stock_level,omitempty"`
	Aisle      string `json:"aisle,omitempty"`
	// Fulfillment lists the ways the item can be bought at the store, as
	// Kroger names them (e.g. curbside, delivery, instore, shiptohome).
	Fulfillment []string `json:"fulfillment,omitempty"`
}

// ProductRecord is one product as Kroger returned it, plus its lite view.
type ProductRecord struct {
	Lite Product
	Raw  json.RawMessage
}

// Full decodes the complete Kroger record.
func (r ProductRecord) Full() (map[string]any, error) {
	return decodeObject(r.Raw)
}

// Fulfillment filter values accepted by ProductQuery.Fulfillment, mapped to
// Kroger's codes.
var fulfillmentCodes = map[string]string{
	"in_store": "ais", "ais": "ais",
	"curbside": "csp", "pickup": "csp", "csp": "csp",
	"delivery": "dth", "dth": "dth",
	"ship_to_home": "sth", "ship": "sth", "sth": "sth",
}

// ProductQuery filters a product search. At least one of Term, ProductIDs or
// Brand is required.
type ProductQuery struct {
	Term       string
	ProductIDs []string
	LocationID string
	Brand      string
	// Fulfillment is one of in_store, curbside, delivery or ship_to_home
	// (Kroger's ais/csp/dth/sth codes are accepted too).
	Fulfillment string
	Limit       int
	Start       int
}

type apiProduct struct {
	ProductID      string   `json:"productId"`
	UPC            string   `json:"upc"`
	Brand          string   `json:"brand"`
	Description    string   `json:"description"`
	Categories     []string `json:"categories"`
	AisleLocations []struct {
		Description string `json:"description"`
	} `json:"aisleLocations"`
	Items []struct {
		Size  string `json:"size"`
		Price *struct {
			Regular float64 `json:"regular"`
			Promo   float64 `json:"promo"`
		} `json:"price"`
		Fulfillment map[string]bool `json:"fulfillment"`
		Inventory   *struct {
			StockLevel string `json:"stockLevel"`
		} `json:"inventory"`
	} `json:"items"`
}

func (a apiProduct) toProduct() Product {
	p := Product{
		ProductID:   a.ProductID,
		UPC:         a.UPC,
		Brand:       a.Brand,
		Description: a.Description,
		Categories:  a.Categories,
	}
	if len(a.AisleLocations) > 0 {
		p.Aisle = a.AisleLocations[0].Description
	}
	// A product carries one item per UPC in practice; use the first.
	if len(a.Items) > 0 {
		it := a.Items[0]
		p.Size = it.Size
		if it.Price != nil {
			p.RegularPrice = it.Price.Regular
			if it.Price.Promo > 0 && it.Price.Promo < it.Price.Regular {
				p.PromoPrice = it.Price.Promo
			}
		}
		if it.Inventory != nil {
			p.StockLevel = it.Inventory.StockLevel
		}
		for mode, ok := range it.Fulfillment {
			if ok {
				p.Fulfillment = append(p.Fulfillment, mode)
			}
		}
		sort.Strings(p.Fulfillment)
	}
	return p
}

func newProductRecord(raw json.RawMessage) (ProductRecord, error) {
	var a apiProduct
	if err := json.Unmarshal(raw, &a); err != nil {
		return ProductRecord{}, fmt.Errorf("kroger api: decode product: %w", err)
	}
	return ProductRecord{Lite: a.toProduct(), Raw: raw}, nil
}

// SearchProducts searches the catalog. Scoping to a store adds price, stock
// and aisle data and limits results to what that store carries. Every result
// is cached for GetProduct.
func (c *Client) SearchProducts(ctx context.Context, q ProductQuery) ([]ProductRecord, error) {
	term := strings.TrimSpace(q.Term)
	ids := cleanList(q.ProductIDs)
	brand := strings.TrimSpace(q.Brand)
	if term == "" && len(ids) == 0 && brand == "" {
		return nil, errors.New("one of term, product_ids or brand is required")
	}

	params := url.Values{"filter.limit": {strconv.Itoa(clamp(q.Limit, 1, 50, 10))}}
	if term != "" {
		params.Set("filter.term", term)
	}
	if len(ids) > 0 {
		params.Set("filter.productId", strings.Join(ids, ","))
	}
	if brand != "" {
		params.Set("filter.brand", brand)
	}
	loc := strings.TrimSpace(q.LocationID)
	if loc != "" {
		params.Set("filter.locationId", loc)
	}
	if f := strings.ToLower(strings.TrimSpace(q.Fulfillment)); f != "" {
		code, ok := fulfillmentCodes[f]
		if !ok {
			return nil, fmt.Errorf("fulfillment must be one of in_store, curbside, delivery or ship_to_home, got %q", q.Fulfillment)
		}
		params.Set("filter.fulfillment", code)
	}
	if q.Start > 0 {
		params.Set("filter.start", strconv.Itoa(q.Start))
	}

	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := c.get(ctx, tok, "/products", params, &resp); err != nil {
		return nil, err
	}
	out := make([]ProductRecord, 0, len(resp.Data))
	for _, raw := range resp.Data {
		rec, err := newProductRecord(raw)
		if err != nil {
			return nil, err
		}
		c.products.put(rec.Lite.ProductID, loc, raw)
		out = append(out, rec)
	}
	return out, nil
}

// GetProduct fetches one product by product id or UPC, optionally scoped to a
// store. Records seen in the last few minutes (by this call or a search for
// the same store) are served from cache.
func (c *Client) GetProduct(ctx context.Context, id, locationID string) (*ProductRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("product_id is required")
	}
	loc := strings.TrimSpace(locationID)
	if raw, ok := c.products.get(id, loc); ok {
		rec, err := newProductRecord(raw)
		if err != nil {
			return nil, err
		}
		return &rec, nil
	}

	var params url.Values
	if loc != "" {
		params = url.Values{"filter.locationId": {loc}}
	}
	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.get(ctx, tok, "/products/"+url.PathEscape(id), params, &resp); err != nil {
		return nil, err
	}
	rec, err := newProductRecord(resp.Data)
	if err != nil {
		return nil, err
	}
	// Cache under the id asked for and the canonical product id, so a lookup
	// by either hits next time.
	c.products.put(id, loc, resp.Data)
	if rec.Lite.ProductID != id {
		c.products.put(rec.Lite.ProductID, loc, resp.Data)
	}
	return &rec, nil
}

// cleanList trims entries and drops empty ones.
func cleanList(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// decodeObject decodes a raw JSON object for passing through as tool output.
func decodeObject(raw json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("kroger api: decode record: %w", err)
	}
	return m, nil
}

// Detail levels for product and location tools.
const (
	DetailLite = "lite"
	DetailFull = "full"
)

// WantFull parses a detail level. Empty means lite.
func WantFull(detail string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(detail)) {
	case "", DetailLite:
		return false, nil
	case DetailFull:
		return true, nil
	default:
		return false, fmt.Errorf("detail must be %q or %q, got %q", DetailLite, DetailFull, detail)
	}
}
