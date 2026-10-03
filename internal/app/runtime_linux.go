//go:build linux

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/containers"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/lifecycle"
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

func repairDrained(config core.Config) error {
	if _, err := os.Lstat("/etc/systemd/system/hostlens-repair.service"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		state, queryErr := exec.CommandContext(ctx, "/usr/bin/systemctl", "show", "--property=ActiveState", "--value", "hostlens-repair.service").Output()
		if queryErr != nil {
			return fmt.Errorf("cannot confirm repair worker shutdown: %w", queryErr)
		}
		if value := strings.TrimSpace(string(state)); value != "inactive" && value != "failed" {
			return fmt.Errorf("repair worker is %s; stop and drain it before read-only activation", value)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot inspect repair worker unit: %w", err)
	}
	if config.RepairSocket == "" {
		return nil
	}
	if _, err := os.Lstat(config.RepairSocket); err == nil {
		return fmt.Errorf("repair worker socket remains at %s; stop and drain the worker before read-only activation", config.RepairSocket)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot confirm repair worker shutdown: %w", err)
	}
	return nil
}

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

func runLifecycle(cmd *cobra.Command, action, source, digest string, apply, start bool) error {
	manager := lifecycle.Manager{Source: source}
	if !apply && digest != "" {
		return errors.New("--plan requires --apply")
	}
	var plan lifecycle.Plan
	var err error
	if apply {
		plan, err = manager.Apply(cmd.Context(), action, digest, start, cmd.ErrOrStderr())
	} else {
		plan, err = manager.Preview(action, start)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(plan)
}

func runStatus(cmd *cobra.Command, _ []string) error {
	return json.NewEncoder(cmd.OutOrStdout()).Encode(status.New().Host(cmd.Context(), func(string) bool { return true }))
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
