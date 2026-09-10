package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/diagnosticsapp"
)

func TestExplanationTraversalBoundsAndSymlinks(t *testing.T) {
	root := t.TempDir()
	for i := range 150 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprint(i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, limit := range []int{0, 1, 65, 200} {
		seen := 0
		truncated, issues := explainDescendants(context.Background(), root, limit, func(string) { seen++ })
		if seen != min(limit, 150) || truncated != (limit <= 150) || len(issues) != 0 {
			t.Fatalf("limit=%d seen=%d truncated=%t issues=%v", limit, seen, truncated, issues)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	truncated, _ := explainDescendants(ctx, root, 100, func(string) { t.Fatal("inspected after cancellation") })
	if !truncated {
		t.Fatal("cancellation not reported")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	truncated, issues := explainDescendants(context.Background(), link, 100, func(string) { t.Fatal("followed symlink") })
	if truncated || len(issues) != 0 {
		t.Fatal("symlink root should not be traversed", issues)
	}
}

func TestRejectUnexpectedPositionalArguments(t *testing.T) {
	for _, args := range [][]string{
		{"policy", "explain", "--recursive", "/etc", "/var"},
		{"policy", "explain", "/etc", "/var"},
	} {
		if err := Main(args); err == nil || !strings.Contains(err.Error(), "unexpected positional") {
			t.Fatal("extra policy target not rejected before configuration load", args, err)
		}
	}
	if err := diagnosticsapp.Main([]string{"serve", "extra"}); err == nil || !strings.Contains(err.Error(), "unexpected positional") {
		t.Fatal("diagnostics argument not rejected before configuration load", err)
	}
}
