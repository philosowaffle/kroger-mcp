package kroger

import (
	"encoding/json"
	"sync"
	"time"
)

const (
	// productCacheTTL is how long a fetched product record is reused. Prices
	// and stock move slowly enough that a few minutes is safe, and it lets
	// Claude ask for full details on something it just searched without a
	// second API call.
	productCacheTTL = 5 * time.Minute
	// productCacheMax bounds memory. Beyond it, expired entries are dropped
	// first, then the oldest.
	productCacheMax = 2000
)

type cacheEntry struct {
	raw     json.RawMessage
	expires time.Time
}

// productCache holds raw Kroger product records keyed by product id and the
// store they were fetched for (price and stock differ per store).
type productCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]cacheEntry
}

func newProductCache(now func() time.Time) *productCache {
	return &productCache{now: now, entries: make(map[string]cacheEntry)}
}

func productCacheKey(productID, locationID string) string {
	return productID + "@" + locationID
}

func (pc *productCache) get(productID, locationID string) (json.RawMessage, bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	key := productCacheKey(productID, locationID)
	e, ok := pc.entries[key]
	if !ok {
		return nil, false
	}
	if !pc.now().Before(e.expires) {
		delete(pc.entries, key)
		return nil, false
	}
	return e.raw, true
}

func (pc *productCache) put(productID, locationID string, raw json.RawMessage) {
	if productID == "" {
		return
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	now := pc.now()
	if len(pc.entries) >= productCacheMax {
		pc.evictLocked(now)
	}
	pc.entries[productCacheKey(productID, locationID)] = cacheEntry{raw: raw, expires: now.Add(productCacheTTL)}
}

// evictLocked drops expired entries, then the soonest-to-expire ones, until
// there is room for one more.
func (pc *productCache) evictLocked(now time.Time) {
	for k, e := range pc.entries {
		if !now.Before(e.expires) {
			delete(pc.entries, k)
		}
	}
	for len(pc.entries) >= productCacheMax {
		var oldestKey string
		var oldest time.Time
		for k, e := range pc.entries {
			if oldestKey == "" || e.expires.Before(oldest) {
				oldestKey, oldest = k, e.expires
			}
		}
		delete(pc.entries, oldestKey)
	}
}
