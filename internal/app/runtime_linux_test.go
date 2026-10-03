//go:build linux

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Monska85/hostlens/internal/core"
)

func TestReadOnlyActivationWaitsForRepairSocketRemoval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repair.sock")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	config := core.Config{RepairSocket: path}
	if err := repairDrained(config); err == nil {
		t.Fatal("read-only activation accepted an existing repair socket")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := repairDrained(config); err != nil {
		t.Fatalf("drained worker blocked read-only activation: %v", err)
	}
}
