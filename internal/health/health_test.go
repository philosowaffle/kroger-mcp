package health

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestURL(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{":8080", "http://127.0.0.1:8080/healthz"},
		{"0.0.0.0:9000", "http://127.0.0.1:9000/healthz"},
		{"[::]:8080", "http://127.0.0.1:8080/healthz"},
		{"localhost:8080", "http://localhost:8080/healthz"},
		{"[::1]:8080", "http://[::1]:8080/healthz"},
		{"example.com", "http://example.com/healthz"},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			if got := URL(tt.addr); got != tt.want {
				t.Errorf("URL(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(Handler))
		defer srv.Close()
		if err := Check(srv.Listener.Addr().String()); err != nil {
			t.Fatalf("Check: %v", err)
		}
	})
	t.Run("unhealthy status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()
		if err := Check(srv.Listener.Addr().String()); err == nil {
			t.Fatal("want error, got nil")
		}
	})
	t.Run("nothing listening", func(t *testing.T) {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := l.Addr().String()
		_ = l.Close()
		if err := Check(addr); err == nil {
			t.Fatal("want error, got nil")
		}
	})
}
