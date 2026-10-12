// Package health serves the /healthz liveness probe and provides the client
// the container HEALTHCHECK uses to probe it (the distroless image has no
// shell or curl, so the binary checks itself).
package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Path is the route the probe is served on.
const Path = "/healthz"

// Handler always reports healthy once the HTTP server is accepting requests.
func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok"))
}

// URL builds the probe URL for a listen address such as ":8080". Wildcard or
// missing hosts are probed on loopback.
func URL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host, port = addr, ""
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	if port == "" {
		return "http://" + host + Path
	}
	return "http://" + net.JoinHostPort(host, port) + Path
}

// Check probes the server listening on addr once and returns an error unless
// it answers 200.
func Check(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, URL(addr), http.NoBody)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}
