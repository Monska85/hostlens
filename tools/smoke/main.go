package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	endpoint := os.Getenv("HOSTLENS_SMOKE_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8080/mcp"
	}
	if len(os.Args) == 2 && os.Args[1] == "--ready" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := ready(ctx, endpoint); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("HTTP listener: ready")
		return
	}
	if len(os.Args) != 4 && (len(os.Args) != 5 || os.Args[4] != "--expect-error") {
		panic("usage: smoke --ready | TOKEN_JSON TOOL EXPECTED_VALUE [--expect-error]")
	}
	b, e := os.ReadFile(os.Args[1])
	if e != nil {
		panic(e)
	}
	var token struct {
		Secret string `json:"secret"`
	}
	if e = json.Unmarshal(b, &token); e != nil {
		panic(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := smoke(ctx, endpoint, token.Secret, os.Args[2], os.Args[3], len(os.Args) == 5); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(os.Args[2] + ": passed")
}

func smoke(ctx context.Context, endpoint, secret, tool, expected string, expectError bool) error {
	args := map[string]any{}
	if tool == "read_config" {
		args["path"] = "/opt/hostlens-fixture.conf"
	}
	if tool == "query_logs" {
		args["unit"] = "hostlens-fixture.service"
	}
	client := http.Client{Timeout: 15 * time.Second}
	offset := 0
	for {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+secret)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		response, err := client.Do(req)
		if err != nil {
			return err
		}
		out, err := io.ReadAll(io.LimitReader(response.Body, 262144))
		response.Body.Close()
		if err != nil {
			return err
		}
		err = checkResponse(out, response.StatusCode, tool, expected, expectError)
		next, ok := err.(nextPackagePage)
		if !ok {
			return err
		}
		if int(next) <= offset {
			return fmt.Errorf("package pagination did not advance")
		}
		offset = int(next)
		args["offset"] = offset
	}
}

type nextPackagePage int

func (nextPackagePage) Error() string { return "expected package absent from current page" }

// Readiness retries only transport failures. A responding handler must enforce authentication.
func ready(ctx context.Context, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("invalid readiness endpoint")
	}
	client := http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for {
		response, err := client.Do(req)
		if err == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				return fmt.Errorf("readiness expected HTTP 401, got %d", response.StatusCode)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("HTTP listener readiness timed out")
		case <-time.After(250 * time.Millisecond):
		}
	}
}

type osInfo struct {
	Family, Architecture, Distribution, Product, Version string
}

func checkResponse(out []byte, status int, tool, expected string, expectError bool) error {
	var response struct {
		Result *struct {
			IsError    bool `json:"isError"`
			Structured *struct {
				Error      bool
				NextOffset *int `json:"next_offset"`
				Issues     []struct{ Code string }
				Data       struct {
					osInfo
					OS      osInfo
					Content string
					Items   []struct{ Name, Version string }
					Entries []struct{ Message string }
				}
			} `json:"structuredContent"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	failure := fmt.Errorf("smoke expectation failed for %s (HTTP %d)", tool, status)
	if json.Unmarshal(out, &response) != nil || status != http.StatusOK || response.Result == nil || len(response.Error) != 0 {
		return failure
	}
	r := response.Result.Structured
	if r == nil || response.Result.IsError != expectError || r.Error != expectError {
		return failure
	}
	if expectError {
		for _, issue := range r.Issues {
			if issue.Code == expected {
				return nil
			}
		}
		return failure
	}
	switch tool {
	case "get_os_info", "get_inventory":
		info := r.Data.osInfo
		if tool == "get_inventory" {
			info = r.Data.OS
		}
		// Arch is rolling and does not advertise VERSION_ID.
		if info.Family == "linux" && info.Architecture == expected && strings.TrimSpace(info.Distribution) != "" && strings.TrimSpace(info.Product) != "" && (info.Distribution == "arch" || strings.TrimSpace(info.Version) != "") {
			return nil
		}
	case "list_packages":
		found := expected == "nonempty"
		for _, item := range r.Data.Items {
			if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Version) == "" {
				return failure
			}
			found = found || item.Name == expected
		}
		if found && len(r.Data.Items) > 0 {
			return nil
		}
		if len(r.Data.Items) > 0 && r.NextOffset != nil && *r.NextOffset > 0 {
			return nextPackagePage(*r.NextOffset)
		}
	case "read_config":
		if r.Data.Content == "fixture: "+expected+"\n" {
			return nil
		}
	case "query_logs":
		for _, entry := range r.Data.Entries {
			if entry.Message == expected {
				return nil
			}
		}
	}
	return failure
}
