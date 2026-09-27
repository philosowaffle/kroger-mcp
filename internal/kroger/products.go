package kroger

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Product is a catalog item, flattened to what's useful when shopping. Price,
// stock level and aisle are only present when the lookup is scoped to a store.
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
	// Fulfillment lists the ways the item can be bought at the store, e.g.
	// curbside, delivery, inStore, shipToHome.
	Fulfillment []string `json:"fulfillment,omitempty"`
}

// ProductQuery filters a product search. Term is required.
type ProductQuery struct {
	Term       string
	LocationID string
	Brand      string
	Limit      int
	Start      int
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

// SearchProducts searches the catalog. Scoping to a store adds price, stock
// and aisle data and limits results to what that store carries.
func (c *Client) SearchProducts(ctx context.Context, q ProductQuery) ([]Product, error) {
	term := strings.TrimSpace(q.Term)
	if term == "" {
		return nil, errors.New("term is required")
	}
	params := url.Values{
		"filter.term":  {term},
		"filter.limit": {strconv.Itoa(clamp(q.Limit, 1, 50, 10))},
	}
	if loc := strings.TrimSpace(q.LocationID); loc != "" {
		params.Set("filter.locationId", loc)
	}
	if brand := strings.TrimSpace(q.Brand); brand != "" {
		params.Set("filter.brand", brand)
	}
	if q.Start > 0 {
		params.Set("filter.start", strconv.Itoa(q.Start))
	}

	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []apiProduct `json:"data"`
	}
	if err := c.get(ctx, tok, "/products", params, &resp); err != nil {
		return nil, err
	}
	out := make([]Product, 0, len(resp.Data))
	for _, p := range resp.Data {
		out = append(out, p.toProduct())
	}
	return out, nil
}

// GetProduct fetches one product by product id or UPC, optionally scoped to
// a store.
func (c *Client) GetProduct(ctx context.Context, id, locationID string) (*Product, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("product_id is required")
	}
	var params url.Values
	if loc := strings.TrimSpace(locationID); loc != "" {
		params = url.Values{"filter.locationId": {loc}}
	}

	tok, err := c.appAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data apiProduct `json:"data"`
	}
	if err := c.get(ctx, tok, "/products/"+url.PathEscape(id), params, &resp); err != nil {
		return nil, err
	}
	p := resp.Data.toProduct()
	return &p, nil
}
