package gateway

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/token"
)

func TestBearerSchemeSpellingIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	c := Coordinator{Active: backend.Snapshot{Config: cfg}, Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Tokens: verifyFunc(func(secret string) (token.Record, error) {
		if secret != "test" {
			return token.Record{}, errors.New("invalid credential")
		}
		return token.Record{ID: "test", Roles: []string{"health"}}, nil
	}), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"capabilities":{}}`)), Header: make(http.Header)}, nil
	})}}
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	for _, spelling := range []string{"Bearer", "bearer", "BEARER"} {
		req := httptest.NewRequest("POST", "http://example.com/mcp", strings.NewReader(body))
		req.Header.Set("Authorization", spelling+" test")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		response := httptest.NewRecorder()
		c.Handler().ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("%s scheme rejected: %d %s", spelling, response.Code, response.Body.String())
		}
	}
	for _, tc := range []struct{ auth string }{{""}, {"Basic test"}, {"Bearer"}, {"Bearer  test"}, {"Bearer wrong"}} {
		req := httptest.NewRequest("POST", "http://example.com/mcp", strings.NewReader(body))
		if tc.auth != "" {
			req.Header.Set("Authorization", tc.auth)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		response := httptest.NewRecorder()
		c.Handler().ServeHTTP(response, req)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("malformed credential accepted: %q %d", tc.auth, response.Code)
		}
	}
}

func TestOriginAndRequestBodyLimits(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	cfg.Limits.RequestBytes = 256
	cfg.Limits.Concurrent = 1
	cfg.Server.AllowedOrigins = []string{"https://allowed.example"}
	collectionCalls := 0
	c := Coordinator{Active: backend.Snapshot{Config: cfg}, Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Tokens: verifyFunc(func(string) (token.Record, error) { return token.Record{ID: "test", Roles: []string{"health"}}, nil }), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/call" {
			collectionCalls++
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"capabilities":{}}`)), Header: make(http.Header)}, nil
	})}}
	for _, tc := range []struct {
		name, origin string
		size         int
		unknown      bool
		status       int
	}{
		{"foreign-origin", "https://foreign.example", 0, false, http.StatusForbidden},
		{"declared-oversize", "", 257, false, http.StatusRequestEntityTooLarge},
		{"streamed-oversize", "", 257, true, http.StatusRequestEntityTooLarge},
		{"same-origin", "http://example.com", 0, false, http.StatusOK},
		{"configured-origin", "https://allowed.example", 0, false, http.StatusOK},
		{"absent-origin", "", 0, false, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
			if tc.size > 0 {
				body += strings.Repeat(" ", tc.size-len(body))
			}
			req := httptest.NewRequest("POST", "http://example.com/mcp", strings.NewReader(body))
			if tc.unknown {
				req.ContentLength = -1
			}
			req.Header.Set("Authorization", "Bearer test")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("MCP-Protocol-Version", "2025-06-18")
			req.Header.Set("Origin", tc.origin)
			before := collectionCalls
			response := httptest.NewRecorder()
			c.Handler().ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("got %d: %s", response.Code, response.Body.String())
			}
			if tc.status != http.StatusOK && collectionCalls != before {
				t.Fatal("rejected request invoked collection")
			}
			if c.running != 0 {
				t.Fatal("rejected request leaked admission slot")
			}
		})
	}
}
