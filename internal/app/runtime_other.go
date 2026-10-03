//go:build !linux

package app

import (
	"errors"
	"os"

	"github.com/Monska85/hostlens/internal/containers"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/server"
	"github.com/Monska85/hostlens/internal/status"
	"github.com/spf13/cobra"
)

var unsupported = errors.New("native HostLens status and repair adapters are not available on this platform")

func administrator() bool              { return false }
func gatewayIdentity(core.Config) bool { return false }
func platformSignals() []os.Signal     { return []os.Signal{os.Interrupt} }
func platformServices(core.Config) (status.Reader, containers.Reader, server.RepairService, error) {
	return nil, nil, nil, unsupported
}
func runMigrate(*cobra.Command, migrationOptions) error { return unsupported }
func runUninstall(*cobra.Command, bool) error           { return unsupported }
func runStatus(*cobra.Command, []string) error          { return unsupported }
func runObserver(*cobra.Command, string) error          { return unsupported }
func runRepair(*cobra.Command, string) error            { return unsupported }
