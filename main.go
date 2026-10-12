// Command kroger-mcp is an MCP server that wraps the Kroger Public API so
// Claude can search stores and products and add items to a Kroger-family
// grocery cart. It serves Streamable HTTP for self-hosting.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/philosowaffle/kroger-cart/internal/auth"
	"github.com/philosowaffle/kroger-cart/internal/health"
	"github.com/philosowaffle/kroger-cart/internal/kroger"
	"github.com/philosowaffle/kroger-cart/internal/logging"
	"github.com/philosowaffle/kroger-cart/internal/server"
)

// sessionTimeout closes MCP sessions idle for this long. Clients start a new
// session when an old one is gone, so this only bounds abandoned ones.
const sessionTimeout = time.Hour

func listenAddr() string {
	if addr := os.Getenv("KROGER_MCP_ADDR"); addr != "" {
		return addr
	}
	return ":8080"
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the running server's health endpoint and exit (used by the container HEALTHCHECK)")
	flag.Parse()

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
		slog.Info("listening", "service", server.Name, "version", server.Version, "addr", addr)
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
	}, &mcp.StreamableHTTPOptions{SessionTimeout: sessionTimeout})

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
