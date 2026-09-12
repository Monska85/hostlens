package token

import (
	"bytes"
	"encoding/json"
	"github.com/Monska85/hostlens/internal/contract"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLifecycle(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "tokens.json"), AdminUID: os.Geteuid()}
	r, secret, e := s.Create("client", []string{"diagnostics"}, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	b, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), secret) {
		t.Fatal("secret persisted")
	}
	if _, e = s.Verify(secret); e != nil {
		t.Fatal(e)
	}
	if e = s.Update(r.ID, []string{"health"}, false); e != nil {
		t.Fatal(e)
	}
	current, err := s.Verify(secret)
	if err != nil {
		t.Fatal(err)
	}
	if Allows(current.Roles, "query_logs") {
		t.Fatal("stale roles")
	}
	overlap := time.Now().Add(time.Minute)
	replacement, newSecret, e := s.Rotate(r.ID, time.Now().Add(2*time.Hour), overlap)
	if e != nil {
		t.Fatal(e)
	}
	for _, credential := range []string{secret, newSecret} {
		if _, err := s.Verify(credential); err != nil {
			t.Fatal("rotation overlap rejected an active credential", err)
		}
	}
	old, err := s.Verify(secret)
	if err != nil || !old.Expires.Equal(overlap) {
		t.Fatal("rotation did not apply overlap expiry", err)
	}
	if err := s.change(func(records *[]Record) error {
		for i := range *records {
			if (*records)[i].ID == r.ID {
				(*records)[i].Expires = time.Now().Add(-time.Second)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, e = s.Verify(secret); e == nil {
		t.Fatal("overlap expired")
	}
	if _, e = s.Verify(newSecret); e != nil {
		t.Fatal(e)
	}
	if e = s.Update(replacement.ID, nil, true); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Verify(newSecret); e == nil {
		t.Fatal("revocation bypass")
	}
	all, e := s.List(true)
	if e != nil || len(all) != 2 {
		t.Fatal(e)
	}
	for _, row := range all {
		if row.Hash != "" {
			t.Fatal("hash listed")
		}
	}
}
func TestEveryToolRole(t *testing.T) {
	for i, role := range []string{"health", "inspect", "diagnostics"} {
		for j, tool := range contract.ToolNames() {
			want := j < 3 || i >= 1 && j < 6 || i == 2
			if Allows([]string{role}, tool) != want {
				t.Errorf("%s %s", role, tool)
			}
		}
	}
	if Allows([]string{"diagnostics"}, "shell") {
		t.Fatal("shell authorized")
	}
}
func TestConcurrentWritersAndLocalAuthority(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "tokens.json"), AdminUID: os.Geteuid()}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, e := s.Create("client", []string{"health"}, time.Now().Add(time.Hour)); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	rs, e := s.List(true)
	if e != nil || len(rs) != 12 {
		t.Fatalf("lost updates %d %v", len(rs), e)
	}
	s.AdminUID = os.Geteuid() + 1
	if _, _, e = s.Create("intruder", []string{"health"}, time.Now().Add(time.Hour)); e == nil {
		t.Fatal("local admin bypass")
	}
}

func TestStoreSizeFailurePreservesActiveCredentials(t *testing.T) {
	s := Store{Path: filepath.Join(t.TempDir(), "tokens.json"), AdminUID: os.Geteuid()}
	_, secret, err := s.Create("client", []string{"health"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	records, err := s.read()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// A name fills the existing valid JSON store exactly to its read ceiling.
	records[0].Name += strings.Repeat("x", maxStoreBytes-len(data))
	data, err = json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != maxStoreBytes {
		t.Fatalf("invalid boundary fixture: %d", len(data))
	}
	if err = Atomic(s.Path, data, 0640); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Verify(secret); err != nil {
		t.Fatal("boundary store unreadable", err)
	}
	if _, _, err = s.Create("overflow", []string{"health"}, time.Now().Add(time.Hour)); err == nil {
		t.Fatal("oversized candidate accepted")
	}
	after, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, after) {
		t.Fatal("failed mutation replaced active token state")
	}
	if _, err = s.Verify(secret); err != nil {
		t.Fatal("failed mutation disabled credential", err)
	}
}

func TestTokenStoreRejectsNonRegularAndTrailingData(t *testing.T) {
	for _, suffix := range []string{" {}", " []", " garbage"} {
		t.Run(suffix, func(t *testing.T) {
			s := Store{Path: filepath.Join(t.TempDir(), "tokens.json"), AdminUID: os.Geteuid()}
			_, secret, err := s.Create("client", []string{"health"}, time.Now().Add(time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(s.Path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(s.Path, append(data, suffix...), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Verify(secret); err == nil {
				t.Fatal("trailing token data accepted")
			}
		})
	}
	s := Store{Path: filepath.Join(t.TempDir(), "tokens.json")}
	if err := unix.Mkfifo(s.Path, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := s.Verify("invalid"); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO token store accepted")
		}
	case <-time.After(time.Second):
		// Unblock a regressed blocking open before failing the test.
		f, err := os.OpenFile(s.Path, os.O_WRONLY|unix.O_NONBLOCK, 0)
		if err == nil {
			f.Close()
		}
		t.Fatal("FIFO token store blocked authentication")
	}
}

func TestMetricsRoleIsIndependent(t *testing.T) {
	if !RolesOK([]string{"metrics"}) || RolesOK([]string{"admin"}) {
		t.Fatal("role validation")
	}
	for _, tool := range contract.ToolNames() {
		if Allows([]string{"metrics"}, tool) {
			t.Fatal("metrics grants tool", tool)
		}
	}
	for _, role := range []string{"health", "inspect", "diagnostics"} {
		for _, tool := range contract.ToolNames() {
			if Allows([]string{role, "metrics"}, tool) != Allows([]string{role}, tool) {
				t.Fatal("union changed", role, tool)
			}
		}
	}
	s := Store{Path: filepath.Join(t.TempDir(), "tokens.json"), AdminUID: os.Geteuid()}
	row, secret, err := s.Create("existing", []string{"health"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Update(row.ID, []string{"health", "metrics"}, false); err != nil {
		t.Fatal(err)
	}
	current, err := s.Verify(secret)
	if err != nil || len(current.Roles) != 2 {
		t.Fatal("secret changed or roles lost", err)
	}
	if err = s.Update(row.ID, nil, true); err != nil {
		t.Fatal(err)
	}
	if err = s.Update(row.ID, []string{"health"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Verify(secret); err == nil {
		t.Fatal("metadata update reactivated revoked token")
	}
	s.AdminUID++
	if s.Update(row.ID, []string{"metrics"}, false) == nil {
		t.Fatal("metrics bypassed local admin identity")
	}
}
