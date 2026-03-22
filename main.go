package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ── Constants ────────────────────────────────────────────────────────────────

const (
	baseURL      = "https://api.kroger.com/v1"
	authURL      = "https://api.kroger.com/v1/connect/oauth2/authorize"
	tokenURL     = "https://api.kroger.com/v1/connect/oauth2/token"
	redirectURI  = "http://localhost:8080/callback"
	callbackPort = ":8080"

	configFile  = "config.json"
	cacheFile   = "product_cache.json"
	groceryFile = "grocery_list.txt"
)

// ── Data types ────────────────────────────────────────────────────────────────

// Config holds credentials and cached OAuth tokens.
// Stored in config.json next to the binary — keep this file private.
type Config struct {
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	LocationID   string    `json:"location_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenExpiry  time.Time `json:"token_expiry"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type productItem struct {
	ProductID   string `json:"productId"`
	UPC         string `json:"upc"`
	Brand       string `json:"brand"`
	Description string `json:"description"`
}

type productsResponse struct {
	Data []productItem `json:"data"`
}

type cartItem struct {
	UPC      string `json:"upc"`
	Quantity int    `json:"quantity"`
	Modality string `json:"modality"`
}

type cartRequest struct {
	Items []cartItem `json:"items"`
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	config, err := loadConfig()
	if err != nil {
		fatalf("Config error: %v", err)
	}

	if err := ensureAuthenticated(config); err != nil {
		fatalf("Authentication failed: %v", err)
	}

	// Resolve location ID if not yet set
	if config.LocationID == "" {
		if err := resolveLocationID(config); err != nil {
			fatalf("Could not resolve location ID: %v", err)
		}
		saveConfig(config)
	}

	items, err := readGroceryList()
	if err != nil {
		fatalf("Could not read grocery list: %v", err)
	}
	if len(items) == 0 {
		fmt.Println("Grocery list is empty — nothing to do.")
		return
	}

	fmt.Printf("\nProcessing %d items...\n\n", len(items))

	cache := loadCache()
	var added, failed []string

	for _, item := range items {
		fmt.Printf("  %-45s", item)

		upc, cached := cache[strings.ToLower(item)]
		if !cached {
			product, err := searchProduct(config, item)
			if err != nil || product == nil {
				fmt.Println("✗  not found")
				failed = append(failed, item)
				continue
			}
			upc = product.UPC
			cache[strings.ToLower(item)] = upc
			saveCache(cache)
			fmt.Printf("→  %s %s\n", product.Brand, product.Description)
		} else {
			fmt.Print("(cached)  ")
		}

		if err := addToCart(config, upc); err != nil {
			fmt.Println("✗  failed to add")
			failed = append(failed, item)
			// Remove from cache so it gets re-searched next time
			delete(cache, strings.ToLower(item))
			saveCache(cache)
			continue
		}
		fmt.Println("✓  added")
		added = append(added, item)
	}

	saveConfig(config)

	fmt.Printf("\n─────────────────────────────────────────────\n")
	fmt.Printf("✓  %d item(s) added to your cart\n", len(added))
	if len(failed) > 0 {
		fmt.Printf("✗  %d item(s) need manual attention:\n", len(failed))
		for _, f := range failed {
			fmt.Printf("     • %s\n", f)
		}
		fmt.Println("\nAdd these manually at kingsoopers.com/shop/cart")
	}
}

// ── Config ────────────────────────────────────────────────────────────────────

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return firstTimeSetup()
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("malformed %s: %w", configFile, err)
	}
	return &cfg, nil
}

func firstTimeSetup() (*Config, error) {
	fmt.Println("═══════════════════════════════════════════════")
	fmt.Println("  King Soopers Cart Builder — First-time setup")
	fmt.Println("═══════════════════════════════════════════════")
	fmt.Println("You'll need your Kroger Developer API credentials.")
	fmt.Println("Get them at: https://developer.kroger.com\n")

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Client ID:     ")
	clientID, _ := reader.ReadString('\n')

	fmt.Print("Client Secret: ")
	clientSecret, _ := reader.ReadString('\n')

	cfg := &Config{
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
		// LocationID is intentionally left empty here;
		// resolveLocationID will prompt for zip and fill it in after auth.
	}

	if err := saveConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func saveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0600) // owner-only read/write
}

// ── Authentication ────────────────────────────────────────────────────────────

func ensureAuthenticated(cfg *Config) error {
	if cfg.AccessToken != "" && time.Now().Before(cfg.TokenExpiry) {
		return nil // still valid
	}
	if cfg.RefreshToken != "" {
		if err := refreshToken(cfg); err == nil {
			return nil
		}
		fmt.Println("Refresh token expired — re-authorizing...")
	}
	return doOAuthFlow(cfg)
}

func doOAuthFlow(cfg *Config) error {
	verifier, err := generateCodeVerifier()
	if err != nil {
		return err
	}
	challenge := generateCodeChallenge(verifier)

	params := url.Values{
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {"product.compact cart.basic:write profile.compact"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	authEndpoint := authURL + "?" + params.Encode()

	// Spin up a temporary local server to catch the OAuth callback
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{Addr: callbackPort, Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("authorization callback missing code parameter")
			http.Error(w, "Authorization failed", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "<html><body><h2>✓ Authorized!</h2><p>You can close this tab and return to the terminal.</p></body></html>")
		codeCh <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Println("\nOpening your browser to authorize with Kroger...")
	fmt.Printf("If the browser doesn't open automatically, visit:\n%s\n\n", authEndpoint)
	openBrowser(authEndpoint)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("timed out waiting for browser authorization (5 min)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	return exchangeCode(cfg, code, verifier)
}

func exchangeCode(cfg *Config, code, verifier string) error {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	return postToken(cfg, data)
}

func refreshToken(cfg *Config) error {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {cfg.RefreshToken},
	}
	return postToken(cfg, data)
}

func postToken(cfg *Config, data url.Values) error {
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token request failed (%d): %s", resp.StatusCode, body)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return err
	}

	cfg.AccessToken = tr.AccessToken
	if tr.RefreshToken != "" {
		cfg.RefreshToken = tr.RefreshToken
	}
	cfg.TokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn-60) * time.Second)
	return nil
}

// ── Location lookup ───────────────────────────────────────────────────────────

// resolveLocationID prompts for a zip code, queries the Locations API for nearby
// King Soopers stores, lets the user pick one, then stores the real location ID.
func resolveLocationID(cfg *Config) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Zip code (to find your King Soopers store): ")
	zip, _ := reader.ReadString('\n')
	zip = strings.TrimSpace(zip)
	if zip == "" {
		return fmt.Errorf("zip code is required to find your store")
	}

	params := url.Values{
		"filter.zipCode.near": {zip},
		"filter.chain":        {"King Soopers"},
		"filter.limit":        {"5"},
	}
	reqURL := fmt.Sprintf("%s/locations?%s", baseURL, params.Encode())

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("location lookup failed (%d): %s", resp.StatusCode, body)
	}

	// Parse just enough to list stores for the user to pick
	var result struct {
		Data []struct {
			LocationID string `json:"locationId"`
			Name       string `json:"name"`
			Address    struct {
				AddressLine1 string `json:"addressLine1"`
				City         string `json:"city"`
				State        string `json:"state"`
			} `json:"address"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if len(result.Data) == 0 {
		return fmt.Errorf("no King Soopers locations found near zip %s", zip)
	}

	fmt.Printf("\nKing Soopers stores near %s:\n", zip)
	for i, loc := range result.Data {
		fmt.Printf("  [%d] %s — %s, %s %s\n",
			i+1, loc.Name, loc.Address.AddressLine1, loc.Address.City, loc.Address.State)
	}

	fmt.Print("\nEnter the number of your store: ")
	input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	input = strings.TrimSpace(input)

	idx := 0
	fmt.Sscanf(input, "%d", &idx)
	if idx < 1 || idx > len(result.Data) {
		idx = 1
		fmt.Printf("Invalid selection, using store #1\n")
	}

	chosen := result.Data[idx-1]
	cfg.LocationID = chosen.LocationID
	fmt.Printf("Store set to: %s (ID: %s)\n\n", chosen.Name, chosen.LocationID)
	return nil
}

// ── Product search ────────────────────────────────────────────────────────────

func searchProduct(cfg *Config, term string) (*productItem, error) {
	params := url.Values{
		"filter.term":       {term},
		"filter.locationId": {cfg.LocationID},
		"filter.limit":      {"5"},
	}
	reqURL := fmt.Sprintf("%s/products?%s", baseURL, params.Encode())

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed (%d): %s", resp.StatusCode, body)
	}

	var pr productsResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}
	if len(pr.Data) == 0 {
		return nil, nil
	}
	return &pr.Data[0], nil
}

// ── Cart ──────────────────────────────────────────────────────────────────────

func addToCart(cfg *Config, upc string) error {
	payload := cartRequest{
		Items: []cartItem{
			{UPC: upc, Quantity: 1, Modality: "PICKUP"},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", baseURL+"/cart/add", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cart add failed (%d): %s", resp.StatusCode, respBody)
	}
	return nil
}

// ── Grocery list ──────────────────────────────────────────────────────────────

func readGroceryList() ([]string, error) {
	f, err := os.Open(groceryFile)
	if err != nil {
		if os.IsNotExist(err) {
			createSampleGroceryList()
			return nil, fmt.Errorf("%s not found — a sample file has been created, edit it and re-run", groceryFile)
		}
		return nil, err
	}
	defer f.Close()

	var items []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		items = append(items, line)
	}
	return items, scanner.Err()
}

func createSampleGroceryList() {
	sample := `# King Soopers grocery list
# One item per line — be as specific as you like for better matching.
# Lines starting with # are ignored.
# Examples:
#   Bananas
#   Kroger 2% Reduced Fat Milk gallon
#   Dave's Killer Bread 21 Whole Grains
#   Simple Truth Organic Baby Spinach 5oz

Bananas
Kroger 2% Reduced Fat Milk gallon
`
	os.WriteFile(groceryFile, []byte(sample), 0644)
}

// ── Product cache ─────────────────────────────────────────────────────────────

// The cache maps lowercased item names → UPC codes so repeat runs are instant.
// Edit product_cache.json to correct any bad matches from the first run.

func loadCache() map[string]string {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return make(map[string]string)
	}
	var cache map[string]string
	if err := json.Unmarshal(data, &cache); err != nil {
		return make(map[string]string)
	}
	return cache
}

func saveCache(cache map[string]string) {
	data, _ := json.MarshalIndent(cache, "", "  ")
	os.WriteFile(cacheFile, data, 0644)
}

// ── PKCE helpers ──────────────────────────────────────────────────────────────

func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	cmd.Start()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}
