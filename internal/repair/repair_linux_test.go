//go:build linux

package repair

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/status"
	"go.yaml.in/yaml/v3"
)

type fakeRestarter struct {
	calls   atomic.Int32
	outcome *status.RestartOutcome
}

func (f *fakeRestarter) RestartService(_ context.Context, name string) (status.RestartOutcome, error) {
	f.calls.Add(1)
	if f.outcome != nil {
		return *f.outcome, nil
	}
	service := status.Service{Name: name, State: "active"}
	done := true
	return status.RestartOutcome{Service: &service, Invoked: &done, Completed: &done}, nil
}

func TestPostRestartObservationFailureRemainsVisible(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("identity fixture requires disposable root container")
	}
	directory := t.TempDir()
	profiles := filepath.Join(directory, "profiles")
	if err := os.Mkdir(profiles, 0700); err != nil {
		t.Fatal(err)
	}
	profile := core.Profile{Allow: []core.Rule{{Effect: core.EffectRepair, Tool: "restart_service", Resource: "example.service"}}}
	data, err := yaml.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profiles, "repair.yaml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	store := auth.Store{Path: filepath.Join(directory, "tokens.json")}
	configPath := filepath.Join(directory, "config.yaml")
	config := core.Config{Version: 2, Listen: "127.0.0.1:8080", TokenStore: store.Path, ProfileDir: profiles, ActiveProfiles: []string{"repair"}, RepairSocket: filepath.Join(directory, "repair.sock"), GatewayUID: 1001, ObserverUID: 1002, RepairUID: 0, SharedGID: 1001, TokenGID: 1003}
	data, err = yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.Create(context.Background(), "repair", []string{"repair"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	done := true
	backend := &fakeRestarter{outcome: &status.RestartOutcome{Invoked: &done, Completed: &done, Issue: "post_observation_unavailable"}}
	var audit bytes.Buffer
	worker := Server{Socket: config.RepairSocket, ConfigPath: configPath, GatewayUID: config.GatewayUID, Tokens: store, Services: backend, Audit: &audit}
	body, err := json.Marshal(request{Secret: secret, Tool: "restart_service", Target: "example.service"})
	if err != nil {
		t.Fatal(err)
	}
	httpRequest := httptest.NewRequest(http.MethodPost, "/restart", bytes.NewReader(body))
	httpRequest = httpRequest.WithContext(context.WithValue(httpRequest.Context(), peerKey{}, config.GatewayUID))
	writer := httptest.NewRecorder()
	worker.handle(writer, httpRequest)
	if writer.Code != http.StatusOK {
		t.Fatalf("repair response: %d", writer.Code)
	}
	var result response
	if err := json.Unmarshal(writer.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Invoked == nil || !*result.Invoked || result.Completed == nil || !*result.Completed || result.Service != nil || result.Issue != "post_observation_unavailable" {
		t.Fatalf("lost partial outcome: %+v", result)
	}
	backend.outcome = &status.RestartOutcome{Issue: "restart_outcome_unknown"}
	httpRequest = httptest.NewRequest(http.MethodPost, "/restart", bytes.NewReader(body))
	httpRequest = httpRequest.WithContext(context.WithValue(httpRequest.Context(), peerKey{}, config.GatewayUID))
	writer = httptest.NewRecorder()
	worker.handle(writer, httpRequest)
	if writer.Code != http.StatusOK {
		t.Fatalf("unknown restart response: %d", writer.Code)
	}
	result = response{}
	if err := json.Unmarshal(writer.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Invoked != nil || result.Completed != nil || result.Service != nil || result.Issue != "restart_outcome_unknown" {
		t.Fatalf("uncertain outcome was reported as known: %+v", result)
	}
	if strings.Contains(audit.String(), secret) || strings.Contains(audit.String(), "\"service\"") {
		t.Fatal("audit exposed credential or diagnostic payload")
	}
	var events []auditEvent
	for _, line := range strings.Split(strings.TrimSpace(audit.String()), "\n") {
		var event auditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if len(events) != 4 || events[0].Outcome != "started" || events[1].Outcome != "post_observation_unavailable" || events[2].Outcome != "started" || events[3].Outcome != "restart_outcome_unknown" {
		t.Fatalf("unexpected audit sequence: %+v", events)
	}
}

func TestRepairRechecksRoleProfileAndReadOnlyState(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("identity fixture requires disposable root container")
	}
	directory := t.TempDir()
	profiles := filepath.Join(directory, "profiles")
	if err := os.Mkdir(profiles, 0700); err != nil {
		t.Fatal(err)
	}
	profile := core.Profile{Allow: []core.Rule{{Effect: core.EffectRepair, Tool: "restart_service", Resource: "example.service"}}}
	data, err := yaml.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profiles, "repair.yaml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "config.yaml")
	store := auth.Store{Path: filepath.Join(directory, "tokens.json")}
	config := core.Config{Version: 2, Listen: "127.0.0.1:8080", TokenStore: store.Path, ProfileDir: profiles, ActiveProfiles: []string{"repair"}, RepairSocket: filepath.Join(directory, "repair.sock"), GatewayUID: 1001, ObserverUID: 1002, RepairUID: 0, SharedGID: 1001, TokenGID: 1003}
	writeConfig := func() {
		t.Helper()
		data, err := yaml.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configPath, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig()
	repairToken, repairSecret, err := store.Create(context.Background(), "repair", []string{"repair"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	_, observeSecret, err := store.Create(context.Background(), "observe", []string{"observe"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	backend := &fakeRestarter{}
	var audit bytes.Buffer
	worker := Server{Socket: config.RepairSocket, ConfigPath: configPath, GatewayUID: config.GatewayUID, Tokens: store, Services: backend, Audit: &audit}
	call := func(secret, target string, method string) int {
		t.Helper()
		body, err := json.Marshal(request{Secret: secret, Tool: "restart_service", Target: target})
		if err != nil {
			t.Fatal(err)
		}
		httpRequest := httptest.NewRequest(method, "/restart", bytes.NewReader(body))
		httpRequest = httpRequest.WithContext(context.WithValue(httpRequest.Context(), peerKey{}, config.GatewayUID))
		writer := httptest.NewRecorder()
		worker.handle(writer, httpRequest)
		return writer.Code
	}
	if got := call(repairToken.ID+"0000000000000000000", "example.service", http.MethodPost); got != http.StatusForbidden {
		t.Fatalf("token ID without bearer proof returned %d", got)
	}
	if got := call(repairSecret, "example.service", http.MethodPost); got != http.StatusOK {
		t.Fatalf("approved repair returned %d", got)
	}
	if got := call(observeSecret, "example.service", http.MethodPost); got != http.StatusForbidden {
		t.Fatalf("observe role returned %d", got)
	}
	if got := call(repairSecret, "other.service", http.MethodPost); got != http.StatusForbidden {
		t.Fatalf("ungranted target returned %d", got)
	}
	if got := call(repairSecret, "example.service", http.MethodGet); got != http.StatusForbidden {
		t.Fatalf("GET returned %d", got)
	}
	config.ReadOnly = true
	writeConfig()
	if got := call(repairSecret, "example.service", http.MethodPost); got != http.StatusForbidden {
		t.Fatalf("read-only returned %d", got)
	}
	config.ReadOnly = false
	writeConfig()
	if err := store.Revoke(context.Background(), repairToken.ID); err != nil {
		t.Fatal(err)
	}
	if got := call(repairSecret, "example.service", http.MethodPost); got != http.StatusForbidden {
		t.Fatalf("revoked token returned %d", got)
	}
	if backend.calls.Load() != 1 {
		t.Fatalf("unexpected repair calls: %d", backend.calls.Load())
	}
	if strings.Contains(audit.String(), repairSecret) || strings.Contains(audit.String(), observeSecret) {
		t.Fatal("audit included bearer secret")
	}
	if strings.Contains(audit.String(), `"tool":"restart_service","target":"other.service","decision":"denied"`) {
		t.Fatal("denied repair audit included unapproved operation and target")
	}
}
