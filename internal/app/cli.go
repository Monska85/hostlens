package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/server"
	"github.com/spf13/cobra"
)

var Version = "0.1.0-dev"

type migrationOptions struct {
	From, Output                                            string
	GatewayUID, ObserverUID, RepairUID, SharedGID, TokenGID uint32
}

func parseExpiry(value string) (time.Time, error) {
	if value == "never" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func printSecret(writer io.Writer, record auth.Token, secret string) error {
	return json.NewEncoder(writer).Encode(struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}{record.ID, secret})
}

func Main(args []string) error {
	var configPath string
	root := &cobra.Command{Use: "hostlens", Short: "Host and Docker status through MCP", SilenceUsage: true, SilenceErrors: true}
	root.SetArgs(args)
	root.PersistentFlags().StringVar(&configPath, "config", "/etc/hostlens/config.yaml", "configuration file")
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print the binary version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), Version)
		return nil
	}})
	load := func() (core.Config, core.Policy, error) { return core.Load(configPath, server.ToolEffects()) }
	configCommands := &cobra.Command{Use: "config", Short: "Manage configuration"}
	configCommands.AddCommand(&cobra.Command{Use: "validate", Short: "Validate configuration and active profiles", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		_, _, err := load()
		if err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "configuration valid")
		}
		return err
	}})
	root.AddCommand(configCommands)

	var migration migrationOptions
	migrateCommand := &cobra.Command{Use: "migrate", Short: "Prepare a checked 0.6.0 upgrade candidate from 0.5.0", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if !administrator() {
			return errors.New("migration requires administrator identity")
		}
		return runMigrate(cmd, migration)
	}}
	migrateCommand.Flags().StringVar(&migration.From, "from", "/etc/hostlens/config.yaml", "old configuration path")
	migrateCommand.Flags().StringVar(&migration.Output, "output", "", "new candidate directory")
	migrateCommand.Flags().Uint32Var(&migration.GatewayUID, "gateway-uid", 0, "gateway service UID")
	migrateCommand.Flags().Uint32Var(&migration.ObserverUID, "observer-uid", 0, "observer service UID")
	migrateCommand.Flags().Uint32Var(&migration.RepairUID, "repair-uid", 0, "repair service UID")
	migrateCommand.Flags().Uint32Var(&migration.SharedGID, "shared-gid", 0, "local IPC group GID")
	migrateCommand.Flags().Uint32Var(&migration.TokenGID, "token-gid", 0, "separate token reader group GID")
	root.AddCommand(migrateCommand)
	var applyUninstall bool
	uninstallCommand := &cobra.Command{Use: "uninstall", Short: "Preview or remove the system installation", Long: "Preview or remove verified HostLens systemd units and the installed binary. Configuration, tokens, service identities, journals, and administrator grants remain for separate review.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return runUninstall(cmd, applyUninstall)
	}}
	uninstallCommand.Flags().BoolVar(&applyUninstall, "apply", false, "stop services and remove verified HostLens units and binary")
	root.AddCommand(uninstallCommand)
	root.AddCommand(&cobra.Command{Use: "status", Short: "Collect current host status", Args: cobra.NoArgs, RunE: runStatus})
	root.AddCommand(&cobra.Command{Use: "serve", Short: "Run the MCP gateway", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runServe(cmd, configPath) }})
	root.AddCommand(&cobra.Command{Use: "observer", Short: "Run the read-only Docker observer", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runObserver(cmd, configPath) }})
	root.AddCommand(&cobra.Command{Use: "repair", Short: "Run the separately authorized repair worker", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runRepair(cmd, configPath) }})

	tokens := &cobra.Command{Use: "token", Short: "Manage bearer tokens"}
	var name, roles, expires, id, overlap string
	create := &cobra.Command{Use: "create", Short: "Create a bearer token", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		config, _, err := load()
		if err != nil {
			return err
		}
		if !administrator() {
			return errors.New("token administration requires administrator identity")
		}
		deadline, err := parseExpiry(expires)
		if err != nil {
			return err
		}
		record, secret, err := (auth.Store{Path: config.TokenStore, TokenGID: int(config.TokenGID)}).Create(cmd.Context(), name, strings.Split(roles, ","), deadline)
		if err != nil {
			return err
		}
		return printSecret(cmd.OutOrStdout(), record, secret)
	}}
	create.Flags().StringVar(&name, "name", "", "client name")
	create.Flags().StringVar(&roles, "roles", "observe", "comma-separated roles: observe,repair")
	create.Flags().StringVar(&expires, "expires", "never", "RFC3339 expiry or never")
	list := &cobra.Command{Use: "list", Short: "List token metadata", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		config, _, err := load()
		if err != nil {
			return err
		}
		if !administrator() {
			return errors.New("token administration requires administrator identity")
		}
		records, err := (auth.Store{Path: config.TokenStore}).List()
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(records)
	}}
	revoke := &cobra.Command{Use: "revoke", Short: "Revoke a token by ID", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		config, _, err := load()
		if err != nil {
			return err
		}
		if !administrator() {
			return errors.New("token administration requires administrator identity")
		}
		return (auth.Store{Path: config.TokenStore, TokenGID: int(config.TokenGID)}).Revoke(cmd.Context(), id)
	}}
	revoke.Flags().StringVar(&id, "id", "", "token ID")
	rotate := &cobra.Command{Use: "rotate", Short: "Replace a token with a finite overlap", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		config, _, err := load()
		if err != nil {
			return err
		}
		if !administrator() {
			return errors.New("token administration requires administrator identity")
		}
		deadline, err := parseExpiry(expires)
		if err != nil {
			return err
		}
		retire, err := time.Parse(time.RFC3339, overlap)
		if err != nil {
			return err
		}
		record, secret, err := (auth.Store{Path: config.TokenStore, TokenGID: int(config.TokenGID)}).Rotate(cmd.Context(), id, deadline, retire)
		if err != nil {
			return err
		}
		return printSecret(cmd.OutOrStdout(), record, secret)
	}}
	rotate.Flags().StringVar(&id, "id", "", "token ID")
	rotate.Flags().StringVar(&expires, "expires", "never", "new RFC3339 expiry or never")
	rotate.Flags().StringVar(&overlap, "overlap-until", "", "finite RFC3339 retirement time for old token")
	tokens.AddCommand(create, list, revoke, rotate)
	root.AddCommand(tokens)
	return root.Execute()
}
