package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStructuredExpectations(t *testing.T) {
	const ubuntu = `{"family":"linux","architecture":"arm64","distribution":"ubuntu","product":"Ubuntu","version":"24.04"}`
	for _, tc := range []struct {
		name, tool, expected, data string
		valid                      bool
	}{
		{"os", "get_os_info", "arm64", ubuntu, true},
		{"wrong architecture", "get_os_info", "amd64", ubuntu, false},
		{"missing distro version", "get_os_info", "arm64", strings.Replace(ubuntu, `"24.04"`, `""`, 1), false},
		{"rolling distro", "get_os_info", "arm64", `{"family":"linux","architecture":"arm64","distribution":"arch","product":"Arch Linux"}`, true},
		{"inventory", "get_inventory", "arm64", `{"os":` + ubuntu + `}`, true},
		{"inventory scope spoof", "get_inventory", "arm64", `{"scope":"architecture arm64 ubuntu"}`, false},
		{"packages", "list_packages", "nonempty", `{"items":[{"name":"libc6","version":"2.41"}]}`, true},
		{"named package", "list_packages", "systemd", `{"items":[{"name":"systemd","version":"255"}]}`, true},
		{"missing named package", "list_packages", "systemd", `{"items":[{"name":"libc6","version":"2.41"}]}`, false},
		{"empty packages", "list_packages", "nonempty", `{"items":[]}`, false},
		{"blank package version", "list_packages", "nonempty", `{"items":[{"name":"systemd","version":" "}]}`, false},
		{"config", "read_config", "visible", `{"content":"fixture: visible\n"}`, true},
		{"wrong config", "read_config", "visible", `{"content":"unrelated visible text"}`, false},
		{"journal", "query_logs", "HOSTLENS_LOG_FIXTURE", `{"entries":[{"message":"HOSTLENS_LOG_FIXTURE"}]}`, true},
		{"journal metadata spoof", "query_logs", "HOSTLENS_LOG_FIXTURE", `{"ordering":"HOSTLENS_LOG_FIXTURE","entries":[]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Appendf(nil, `{"result":{"structuredContent":{"data":%s}}}`, tc.data)
			if err := checkResponse(body, http.StatusOK, tc.tool, tc.expected, false); (err == nil) != tc.valid {
				t.Fatalf("checkResponse success = %t, want %t", err == nil, tc.valid)
			}
		})
	}
}

func TestProtocolAndFailureAssertions(t *testing.T) {
	const failure = `{"result":{"isError":true,"structuredContent":{"error":true,"issues":[{"code":"collection_failed"}]}}}`
	if err := checkResponse([]byte(failure), http.StatusOK, "query_logs", "collection_failed", true); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`private-payload`,
		`{"result":{"content":[{"text":"collection_failed private-payload"}]}}`,
		strings.Replace(failure, `"isError":true`, `"isError":false`, 1),
		strings.Replace(failure, `"error":true`, `"error":false`, 1),
		strings.Replace(failure, `"code":"collection_failed"`, `"message":"collection_failed"`, 1),
		strings.Replace(failure, `"result":`, `"error":{},"result":`, 1),
	} {
		err := checkResponse([]byte(body), http.StatusOK, "query_logs", "collection_failed", true)
		if err == nil || strings.Contains(err.Error(), "private-payload") {
			t.Fatal("invalid response accepted or payload included in diagnostic")
		}
	}
	if checkResponse([]byte(failure), http.StatusUnauthorized, "query_logs", "collection_failed", true) == nil {
		t.Fatal("accepted unsuccessful HTTP status")
	}
}

func TestReadiness(t *testing.T) {
	for _, status := range []int{401, 500, 302, 0} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if status == 0 {
				<-r.Context().Done()
				return
			}
			w.WriteHeader(status)
		}))
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		started := time.Now()
		err := ready(ctx, server.URL)
		cancel()
		server.Close()
		if (err == nil) != (status == 401) || calls != 1 || time.Since(started) > time.Second {
			t.Fatalf("status=%d, calls=%d, error=%v", status, calls, err)
		}
	}
}
