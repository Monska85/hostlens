package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestSnapshotCompareAndUnknownArgumentRejection(t *testing.T) {
	const tool = `{"name":"get_os_info","description":"observes","inputSchema":{"type":"object"},"outputSchema":{"type":"object","properties":{"data":{"type":"object"}}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			Params struct {
				Tool      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if request.Method == "tools/list" {
			fmt.Fprintf(w, `{"result":{"tools":[%s,{"name":"get_inventory"}]}}`, tool)
			return
		}
		fmt.Fprintf(w, `{"result":{"isError":true,"structuredContent":{"error":true,"issues":[{"code":"invalid_arguments","message":"unrelated_argument is not declared"}]}}}`)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot := t.TempDir() + "/tools.json"
	if err := os.WriteFile(snapshot, []byte(`{"tools":[`+tool+`,{"name":"get_inventory"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := compareSnapshot(ctx, server.URL, "secret", snapshot); err != nil {
		t.Fatal(err)
	}
	// Reordered members and whitespace must canonicalize to equality.
	reordered := `{"tools":[{"outputSchema":{"properties":{"data":{"type":"object"}},"type":"object"},"inputSchema":{"type":"object"},"description":"observes","name":"get_os_info"},{"name":"get_inventory"}]}`
	if err := os.WriteFile(snapshot, []byte(reordered), 0600); err != nil {
		t.Fatal(err)
	}
	if err := compareSnapshot(ctx, server.URL, "secret", snapshot); err != nil {
		t.Fatal("canonical comparison rejected reordered members", err)
	}
	if err := os.WriteFile(snapshot, []byte(`{"tools":[{"name":"get_os_info"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := compareSnapshot(ctx, server.URL, "secret", snapshot); err == nil || !strings.Contains(err.Error(), "drifts from the definition") {
		t.Fatal("altered snapshot accepted", err)
	}
	if err := os.WriteFile(snapshot, []byte(`{"tools":[{"name":"get_inventory"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := compareSnapshot(ctx, server.URL, "secret", snapshot); err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Fatal("undeclared advertised tool accepted", err)
	}
	if err := rejectUnknown(ctx, server.URL, "secret", "get_os_info"); err != nil {
		t.Fatal(err)
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

func TestPackageSmokePagination(t *testing.T) {
	for _, scenario := range []string{"found", "missing", "repeated", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var request struct {
					Params struct{ Arguments struct{ Offset int } }
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if request.Params.Arguments.Offset != (calls-1)*200 {
					t.Error("incorrect page offset")
				}
				if calls == 1 || scenario == "repeated" {
					fmt.Fprint(w, `{"result":{"structuredContent":{"next_offset":200,"data":{"items":[{"name":"aaa","version":"1"}]}}}}`)
					return
				}
				if scenario == "cancelled" {
					<-r.Context().Done()
					return
				}
				name := "systemd"
				if scenario == "missing" {
					name = "zzz"
				}
				fmt.Fprintf(w, `{"result":{"structuredContent":{"data":{"items":[{"name":%q,"version":"1"}]}}}}`, name)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			err := smoke(ctx, server.URL, "test", "list_packages", "systemd", false)
			server.Close()
			if (err == nil) != (scenario == "found") || calls != 2 {
				t.Fatalf("scenario=%s calls=%d err=%v", scenario, calls, err)
			}
		})
	}
}
