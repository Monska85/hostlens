package lifecycle

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestUnsafeServiceGroupsRejectBeforeMutation(t *testing.T) {
	for _, tc := range []struct{ name, record string }{
		{"root group", "hostlens-gateway:x:0:"},
		{"invalid number", "hostlens-gateway:x:invalid:"},
		{"wrong name", "other:x:200:"},
		{"unrelated member", "hostlens-gateway:x:200:other"},
		{"malformed", "hostlens-gateway:x:200"},
		{"colliding groups", "hostlens-gateway:x:201:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, source, sys := setup(t)
			sys.groups["hostlens-gateway"] = 200
			sys.groups["hostlens-diagnostics"] = 201
			m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if name == "getent" && args[0] == "group" && args[1] == "hostlens-gateway" {
					return []byte(tc.record), nil
				}
				return sys.run(ctx, name, args...)
			}
			if err := m.Install(context.Background(), source, false); err == nil {
				t.Fatal("unsafe group accepted")
			}
			if _, err := os.Stat(m.path(ManifestPath)); !os.IsNotExist(err) {
				t.Fatal("installation mutated state", err)
			}
			for _, call := range sys.calls {
				if !strings.HasPrefix(call, "getent ") {
					t.Fatal("mutating command before rejection", call)
				}
			}
		})
	}
}

func TestServiceGroupsRecheckedAfterCreation(t *testing.T) {
	m, source, sys := setup(t)
	m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		b, err := sys.run(ctx, name, args...)
		if name == "useradd" && args[len(args)-1] == "hostlens-diagnostics" {
			sys.groups["hostlens-gateway"] = 0
		}
		return b, err
	}
	if err := m.Install(context.Background(), source, false); err == nil {
		t.Fatal("changed group accepted")
	}
	if _, err := os.Stat(m.path("/usr/local/bin/hostlens")); !os.IsNotExist(err) {
		t.Fatal("binary installed before group recheck", err)
	}
}
