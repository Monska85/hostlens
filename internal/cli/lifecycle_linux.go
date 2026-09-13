package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/lifecycle"
	linux "github.com/Monska85/hostlens/internal/platform/linux"
	"time"
)

func lifecycleCLI(args []string) error {
	f := flags(args[0])
	apply := f.Bool("apply", false, "apply the printed installation/removal plan")
	source := f.String("source", ".", "verified extracted archive directory")
	archive := f.String("archive", "", "upgrade tar.gz archive")
	mode := f.String("privilege", "standard", "standard or restricted")
	start := f.Bool("start", false, "explicitly start newly installed services")
	if e := f.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if f.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	c := config.DefaultsLinux(true)
	c.Privilege = *mode
	if e := config.ValidateLinux(c); e != nil {
		return e
	}
	m := lifecycle.Manager{Config: c}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	switch args[0] {
	case "install":
		plan, e := m.Plan(*source)
		if e != nil {
			return e
		}
		if err := output(plan); err != nil {
			return err
		}
		if *apply {
			if os.Geteuid() != 0 {
				return errors.New("root identity required")
			}
			return m.Install(ctx, *source, *start)
		}
	case "uninstall":
		plan, e := m.Load()
		if e != nil {
			return e
		}
		if err := output(plan); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Shared journals and manually granted ACLs remain administrator-owned.")
		if *apply {
			if os.Geteuid() != 0 {
				return errors.New("root identity required")
			}
			return m.Uninstall(ctx)
		}
	case "upgrade":
		if *archive == "" {
			return errors.New("--archive required")
		}
		fmt.Println("Validate archive and configuration, retain old binaries, preserve profiles and tokens, restart previously active services.")
		if *apply {
			if os.Geteuid() != 0 {
				return errors.New("root identity required")
			}
			installed, _, err := linux.Load("/etc/hostlens/config.yaml", true)
			if err != nil {
				return err
			}
			m.Config = installed
			return m.Upgrade(ctx, *archive)
		}
	}
	return nil
}

// reconcileCLI applies or previews the Docker observer topology plan. It
// always loads the installed configuration so the plan matches the running
// services, and requires the administrator identity for mutation.
func reconcileCLI(args []string) error {
	f := flags(args[0])
	apply := f.Bool("apply", false, "apply the printed reconciliation plan")
	path := f.String("config", "", "configuration path")
	source := f.String("source", "", "extracted release directory supplying the observer binary after older upgrades")
	// Reconciliation always targets the system installation; --system is
	// accepted for command consistency and requires no value.
	system := f.Bool("system", true, "system installation (always enabled)")
	if e := f.Parse(args[1:]); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if f.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	_ = *system
	if !*apply {
		// Reconciliation previews never mutate; root is not required.
	} else if os.Geteuid() != 0 {
		return errors.New("root identity required")
	}
	if *path == "" {
		*path = "/etc/hostlens/config.yaml"
	}
	installed, p, err := linux.Load(*path, true)
	if err != nil {
		return err
	}
	m := lifecycle.Manager{Config: installed, Source: *source}
	_, report, e := m.Reconcile(context.Background(), *apply)
	if e != nil {
		return e
	}
	return output(map[string]any{"policy_fingerprint": p.Fingerprint(), "reconciliation": report})
}
