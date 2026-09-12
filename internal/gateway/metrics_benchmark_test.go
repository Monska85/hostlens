package gateway

import (
	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/token"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func BenchmarkServiceRequest(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		b.Run(name, func(b *testing.B) {
			cfg := config.DefaultsLinux(false)
			cfg.Metrics.Enabled = enabled
			c := &Coordinator{Active: backend.Snapshot{Config: cfg}, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Tokens: verifyFunc(func(string) (token.Record, error) { return token.Record{ID: "test", Roles: []string{"health"}}, nil }), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"capabilities":{"get_os_info":true}}`)), Header: make(http.Header)}, nil
			})}}
			handler := c.Handler()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
				r.Header.Set("Authorization", "Bearer valid")
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("Accept", "application/json, text/event-stream")
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if w.Code != 200 {
					b.Fatal(w.Code)
				}
			}
		})
	}
}
