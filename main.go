// Command kroger-mcp is an MCP server that wraps the Kroger Public API so
// Claude can search stores and products and add items to a Kroger-family
// grocery cart. It serves Streamable HTTP for self-hosting.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-mcp/internal/auth"
	"github.com/philosowaffle/kroger-mcp/internal/health"
	"github.com/philosowaffle/kroger-mcp/internal/kroger"
	"github.com/philosowaffle/kroger-mcp/internal/logging"
	"github.com/philosowaffle/kroger-mcp/internal/server"
)

// listenAddr defaults to loopback so a bare local run without auth is
// private. The container image sets KROGER_MCP_ADDR=:8080.
func listenAddr() string {
	if addr := os.Getenv("KROGER_MCP_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:8080"
}

// allowUnauthEnv opts in to serving the MCP endpoint without auth on a
// non-loopback address.
const allowUnauthEnv = "KROGER_MCP_ALLOW_UNAUTHENTICATED"

// isLoopback reports whether addr (host:port) listens only on loopback. An
// empty host binds every interface, so it doesn't count.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// checkExposure refuses to serve an unauthenticated MCP endpoint beyond
// loopback unless the operator opted in. Anyone who can reach it can add to
// the shopper's cart, or run kroger_login and link their own account in its
// place.
func checkExposure(addr string, authOn bool) error {
	if authOn || isLoopback(addr) {
		return nil
	}
	if allow, _ := strconv.ParseBool(os.Getenv(allowUnauthEnv)); allow {
		slog.Warn("auth disabled on a non-loopback address; anyone who can reach it can use the linked Kroger account",
			"addr", addr, allowUnauthEnv, true)
		return nil
	}
	return fmt.Errorf("refusing to listen on %q without auth: set KROGER_MCP_OIDC_ISSUER to require OAuth, "+
		"listen on loopback (e.g. 127.0.0.1:8080), or set %s=true if the network is trusted", addr, allowUnauthEnv)
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the running server's health endpoint and exit (used by the container HEALTHCHECK)")
	version := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *version {
		fmt.Println(server.Name, server.Version, server.Commit)
		return
	}

	logging.Setup()
	addr := listenAddr()

	if *healthcheck {
		if err := health.Check(addr); err != nil {
			slog.Error("healthcheck failed", "err", err)
			os.Exit(1)
		}
		return
	}

	if err := run(addr); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(addr string) error {
	if err := checkExposure(addr, auth.Enabled()); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	kc, err := kroger.New(kroger.ConfigFromEnv())
	if err != nil {
		return err
	}
	if !kc.Linked() {
		slog.Warn("no Kroger account linked yet; ask Claude to run kroger_login", "redirect_uri", kc.Config().RedirectURI)
	}

	handler, err := newHandler(ctx, kc)
	if err != nil {
		return err
	}

	// No ReadTimeout or WriteTimeout: MCP responses can be long-lived SSE
	// streams, and either deadline would cut them off. The SDK caps request
	// bodies (4 MiB) itself.
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           requestLog(handler),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "service", server.Name, "version", server.Version, "commit", server.Commit, "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(sctx)
}

// newHandler assembles the HTTP routes:
//
//	/healthz                      liveness probe (no auth)
//	/.well-known/...              protected-resource metadata when auth is on,
//	                              404 otherwise (no auth, per RFC 9728)
//	<KROGER_REDIRECT_URI path>    Kroger OAuth callback (no auth; guarded by
//	                              single-use state from kroger_login)
//	/                             MCP Streamable HTTP (bearer auth when on;
//	                              cross-origin browser requests rejected)
func newHandler(ctx context.Context, kc *kroger.Client) (http.Handler, error) {
	srv := server.New(kc)
	var mcpHandler http.Handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{
		// Stateless: the tools keep no per-session state and never call back
		// to the client. go-sdk v1.8.0 rejects, on stateful servers, any call
		// whose _meta carries io.modelcontextprotocol/protocolVersion (SEP-2575),
		// even at 2025-11-25, which Claude's client now sends on every call.
		Stateless: true,
	})

	mux := http.NewServeMux()
	mux.HandleFunc(health.Path, health.Handler)

	// Discovery paths must be reachable without auth. Anything under
	// /.well-known/ other than the protected-resource document gets a plain
	// 404, so a client probing this host for authorization-server metadata
	// moves on to the advertised issuer instead of hitting a 401 from the MCP
	// handler. The exact metadata path registered below takes precedence.
	mux.HandleFunc("/.well-known/", http.NotFound)

	mux.Handle(kc.Config().CallbackPath(), kc.CallbackHandler())

	if auth.Enabled() {
		authn, err := auth.New(ctx)
		if err != nil {
			return nil, err
		}
		mux.Handle(auth.MetadataPath, authn.MetadataHandler())
		mcpHandler = authn.Middleware()(mcpHandler)

		md := authn.Metadata()
		slog.Info("auth enabled",
			"resource", md.Resource,
			"authorization_servers", md.AuthorizationServers,
			"scopes_supported", md.ScopesSupported,
		)
	} else {
		slog.Warn("auth disabled: the MCP endpoint is open to anyone who can reach it; set KROGER_MCP_OIDC_ISSUER to require OAuth")
	}

	// The SDK's DNS-rebinding check only covers loopback. This rejects
	// cross-origin browser requests on any address, so a web page can't drive
	// the tools from a shopper's browser. Clients that send neither Origin nor
	// Sec-Fetch-Site (Claude Code, claude.ai's servers) are unaffected.
	mux.Handle("/", http.NewCrossOriginProtection().Handler(mcpHandler))
	return mux, nil
}

// requestLog logs each request at TRACE, excluding health probes. Only the
// method, path, status, size and duration are logged: query strings are left
// out because the OAuth callback carries the authorization code there.
// httpsnoop keeps http.Flusher working for the MCP SSE stream.
func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == health.Path {
			next.ServeHTTP(w, r)
			return
		}
		m := httpsnoop.CaptureMetrics(next, w, r)
		logging.Trace("request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"status", m.Code,
			"bytes", m.Written,
			"dur", m.Duration,
		)
	})
}
