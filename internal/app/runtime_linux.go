//go:build linux

package app

import (
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/containers"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/migrate"
	"github.com/Monska85/hostlens/internal/observer"
	"github.com/Monska85/hostlens/internal/repair"
	"github.com/Monska85/hostlens/internal/server"
	"github.com/Monska85/hostlens/internal/status"
	"github.com/Monska85/hostlens/internal/uninstall"
	"github.com/spf13/cobra"
)

func administrator() bool                     { return os.Geteuid() == 0 }
func gatewayIdentity(config core.Config) bool { return uint32(os.Geteuid()) == config.GatewayUID }
func platformSignals() []os.Signal            { return []os.Signal{os.Interrupt, syscall.SIGTERM} }

func platformServices(config core.Config) (status.Reader, containers.Reader, server.RepairService, error) {
	var docker containers.Reader
	if config.DockerSocket != "" && config.ObserverSocket != "" {
		docker = observer.NewClient(config.ObserverSocket, config.ObserverUID)
	}
	var repairs server.RepairService
	if !config.ReadOnly && config.RepairSocket != "" {
		repairs = repair.NewClient(config.RepairSocket, config.RepairUID)
	}
	return status.New(), docker, repairs, nil
}

func runMigrate(cmd *cobra.Command, options migrationOptions) error {
	report, err := migrate.Prepare(migrate.Options{
		From: options.From, Output: options.Output, GatewayUID: options.GatewayUID,
		ObserverUID: options.ObserverUID, RepairUID: options.RepairUID,
		SharedGID: options.SharedGID, TokenGID: options.TokenGID,
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
}

func runUninstall(cmd *cobra.Command, apply bool) error {
	return uninstall.Default().Execute(cmd.Context(), apply, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func runStatus(cmd *cobra.Command, _ []string) error {
	return json.NewEncoder(cmd.OutOrStdout()).Encode(status.New().Host(cmd.Context()))
}

func runObserver(cmd *cobra.Command, configPath string) error {
	config, _, err := core.Load(configPath, server.ToolEffects())
	if err != nil {
		return err
	}
	if config.ObserverSocket == "" || config.DockerSocket == "" {
		return errors.New("Docker observer is not configured")
	}
	if uint32(os.Geteuid()) != config.ObserverUID {
		return errors.New("observer identity mismatch")
	}
	reader, err := containers.NewReader(config.DockerSocket)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return (observer.Server{Socket: config.ObserverSocket, SharedGID: int(config.SharedGID), GatewayUID: config.GatewayUID, Docker: reader}).Run(ctx)
}

func runRepair(cmd *cobra.Command, configPath string) error {
	config, _, err := core.Load(configPath, server.ToolEffects())
	if err != nil {
		return err
	}
	if config.ReadOnly || config.RepairSocket == "" {
		return errors.New("repair is disabled")
	}
	if uint32(os.Geteuid()) != config.RepairUID {
		return errors.New("repair identity mismatch")
	}
	var docker containers.Restarter
	if config.DockerSocket != "" {
		docker, err = containers.NewRestarter(config.DockerSocket)
		if err != nil {
			return err
		}
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	worker := repair.Server{Socket: config.RepairSocket, ConfigPath: configPath, GatewayUID: config.GatewayUID, SharedGID: int(config.SharedGID), Tokens: auth.Store{Path: config.TokenStore}, Services: status.NewRestarter(), Containers: docker}
	return worker.Run(ctx)
}
