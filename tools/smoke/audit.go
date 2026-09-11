package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
)

type auditFixture struct {
	Process string `json:"process"`
	Port    int    `json:"port"`
	Config  string `json:"config"`
	Log     string `json:"log"`
	Unit    string `json:"unit,omitempty"`
	Denied  string `json:"denied,omitempty"`
}

func runAudit(endpoint, tokenFile, fixtureFile string) error {
	var token struct {
		Secret string `json:"secret"`
	}
	b, err := os.ReadFile(tokenFile)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &token); err != nil {
		return err
	}
	b, err = os.ReadFile(fixtureFile)
	if err != nil {
		return err
	}
	if len(b) > 4096 {
		return fmt.Errorf("oversized audit fixture")
	}
	var fixture auditFixture
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&fixture); err != nil {
		return err
	}
	if fixture.Process == "" || fixture.Port < 1 || fixture.Port > 65535 || fixture.Config == "" || fixture.Log == "" {
		return fmt.Errorf("incomplete audit fixture")
	}
	if fixture.Denied == "" {
		fixture.Denied = "/tmp/application-denied.conf"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	coverage := map[string]bool{}
	call := func(tool string, args map[string]any) (contract.Result, error) {
		r, err := auditCall(ctx, endpoint, token.Secret, tool, args)
		for _, code := range auditCoverageCodes(r) {
			coverage[code] = true
		}
		return r, err
	}
	pid := 0
	offset := 0
	for pages := 0; pages < 64; pages++ {
		r, err := call("list_processes", map[string]any{"offset": offset, "limit": 200})
		if err != nil {
			return err
		}
		rows, _ := r.Data["items"].([]any)
		for _, row := range rows {
			m, ok := row.(map[string]any)
			if ok && m["name"] == fixture.Process {
				n, _ := m["pid"].(float64)
				pid = int(n)
				break
			}
		}
		if pid > 0 || r.NextOffset == nil {
			break
		}
		if *r.NextOffset <= offset {
			return fmt.Errorf("audit process pagination did not advance")
		}
		offset = *r.NextOffset
	}
	if pid == 0 {
		return fmt.Errorf("expected application process not observed")
	}
	r, err := call("get_process_info", map[string]any{"pid": pid})
	if err != nil {
		return err
	}
	process, _ := r.Data["process"].(map[string]any)
	if process["name"] != fixture.Process {
		return fmt.Errorf("process identity mismatch")
	}
	fmt.Println("PASS: generic process identity and runtime evidence")
	r, err = call("get_network_info", map[string]any{})
	if err != nil {
		return err
	}
	found := false
	for _, key := range []string{"tcp4_listeners", "tcp6_listeners"} {
		rows, _ := r.Data[key].([]any)
		for _, row := range rows {
			m, ok := row.(map[string]any)
			if ok && m["local_port"] == float64(fixture.Port) {
				found = true
			}
		}
	}
	if !found {
		return fmt.Errorf("application listener not observed")
	}
	fmt.Println("PASS: generic network listener evidence")
	r, err = call("inspect_path", map[string]any{"path": fixture.Config})
	if err != nil {
		return err
	}
	if r.Data["type"] != "file" {
		return fmt.Errorf("configuration metadata missing")
	}
	r, err = call("read_config", map[string]any{"path": fixture.Config})
	if err != nil {
		return err
	}
	content, _ := r.Data["content"].(string)
	if content == "" {
		return fmt.Errorf("approved application configuration empty")
	}
	r, err = call("query_logs", map[string]any{"path": fixture.Log, "format": "raw", "raw_tail": true, "limit": 20})
	if err != nil {
		return err
	}
	lines, _ := r.Data["lines"].([]any)
	if len(lines) == 0 {
		return fmt.Errorf("application log evidence missing")
	}
	fmt.Println("PASS: approved application configuration, permissions and logs")
	if fixture.Unit != "" {
		r, err = call("inspect_service", map[string]any{"unit": fixture.Unit})
		if err != nil {
			return err
		}
		service, _ := r.Data["service"].(map[string]any)
		if service["ActiveState"] != "active" {
			return fmt.Errorf("application service not active")
		}
		fmt.Println("PASS: generic systemd runtime and hardening evidence")
	}
	for _, tool := range []string{"list_accounts", "get_storage_info", "get_security_info", "get_hostlens_info"} {
		r, err = call(tool, map[string]any{})
		if err != nil {
			return err
		}
		if err = checkAuditEvidence(tool, r); err != nil {
			return err
		}
	}
	fmt.Println("PASS: generic account, storage, security and HostLens evidence")
	// A denied path must remain denied through both content and metadata tools.
	for _, tool := range []string{"read_config", "inspect_path"} {
		r, err = auditCall(ctx, endpoint, token.Secret, tool, map[string]any{"path": fixture.Denied})
		expectedCode := "policy_denied"
		if tool == "read_config" {
			expectedCode = "source_denied_or_unavailable"
		}
		if err == nil || !r.Error || len(r.Issues) == 0 || r.Issues[0].Code != expectedCode {
			return fmt.Errorf("denied-source negative control failed for %s", tool)
		}
	}
	fmt.Println("PASS: source denial across content and metadata")
	if len(coverage) > 0 {
		codes := make([]string, 0, len(coverage))
		for code := range coverage {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		fmt.Println("Coverage limitations observed: " + strings.Join(codes, ", "))
	}
	return nil
}

func auditCall(ctx context.Context, endpoint, secret, tool string, args map[string]any) (contract.Result, error) {
	var result contract.Result
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 262145))
	if err != nil {
		return result, err
	}
	if len(b) > 262144 {
		return result, fmt.Errorf("audit response exceeded ceiling")
	}
	var envelope struct {
		Error  json.RawMessage
		Result *struct {
			IsError    bool
			Structured *contract.Result `json:"structuredContent"`
		}
	}
	if json.Unmarshal(b, &envelope) != nil || resp.StatusCode != 200 || len(envelope.Error) > 0 || envelope.Result == nil || envelope.Result.Structured == nil {
		return result, fmt.Errorf("invalid MCP response for %s (HTTP %d)", tool, resp.StatusCode)
	}
	result = *envelope.Result.Structured
	if result.Error || envelope.Result.IsError {
		return result, fmt.Errorf("MCP collection failed for %s", tool)
	}
	if result.Host == "" || result.ObservedAt.IsZero() {
		return result, fmt.Errorf("MCP evidence lacks attribution for %s", tool)
	}
	return result, nil
}

// Acceptance requires actual fixture evidence even when collection is partial.
func checkAuditEvidence(tool string, r contract.Result) error {
	switch tool {
	case "list_accounts":
		rows, _ := r.Data["items"].([]any)
		for _, row := range rows {
			account, _ := row.(map[string]any)
			if account["kind"] == "account" && account["name"] == "root" && account["uid"] == float64(0) {
				return nil
			}
		}
	case "get_storage_info":
		rows, _ := r.Data["mounts"].([]any)
		for _, row := range rows {
			mount, _ := row.(map[string]any)
			if mount["mount"] == "/" {
				return nil
			}
		}
	case "get_security_info":
		controls, _ := r.Data["kernel_controls"].(map[string]any)
		value, ok := controls["/proc/sys/kernel/randomize_va_space"].(float64)
		if ok && (value == 0 || value == 1 || value == 2) {
			return nil
		}
	case "get_hostlens_info":
		mode, privilege := r.Data["mode"], r.Data["privilege"]
		if (mode != "user" && mode != "system") || (privilege != "restricted" && privilege != "standard") {
			break
		}
		domains, _ := r.Data["audit_domains"].(map[string]any)
		for _, domain := range []string{"processes", "network", "accounts", "storage", "security", "hostlens", "paths"} {
			if domains[domain] != true {
				return fmt.Errorf("expected audit domain grant missing")
			}
		}
		return nil
	}
	return fmt.Errorf("required fixture evidence missing for %s", tool)
}

func auditCoverageCodes(r contract.Result) []string {
	seen := map[string]bool{}
	for _, issue := range r.Issues {
		code := issue.Code
		switch code {
		case "evidence_unavailable", "unavailable_interface", "partial_observation", "source_unavailable", "permission_denied", "policy_denied", "executable_unavailable", "insufficient_visibility", "malformed_source", "inspection_limit", "collection_failed":
		default:
			code = "other"
		}
		seen[code] = true
	}
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}
