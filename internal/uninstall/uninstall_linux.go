//go:build linux

package uninstall

import (
	"context"
	"debug/buildinfo"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

//go:embed systemd/*.service
var units embed.FS

var names = []string{"hostlens-gateway.service", "hostlens-observer.service", "hostlens-repair.service"}

// PackagedUnit returns the unit shipped with the running binary.
func PackagedUnit(name string) ([]byte, error) {
	for _, known := range names {
		if name == known {
			return units.ReadFile("systemd/" + name)
		}
	}
	return nil, errors.New("unknown HostLens unit")
}

type Report struct {
	Action  string   `json:"action"`
	Remove  []string `json:"remove"`
	Retain  []string `json:"retain"`
	Message string   `json:"message"`
}

type Manager struct {
	UnitDir      string
	Binary       string
	ExpectedMain string
	Run          func(context.Context, string, ...string) error
}

func Default() Manager {
	return Manager{UnitDir: "/etc/systemd/system", Binary: "/usr/local/bin/hostlens"}
}

func (m Manager) defaults() Manager {
	if m.UnitDir == "" {
		m.UnitDir = "/etc/systemd/system"
	}
	if m.Binary == "" {
		m.Binary = "/usr/local/bin/hostlens"
	}
	if m.ExpectedMain == "" {
		m.ExpectedMain = "github.com/Monska85/hostlens/cmd/hostlens"
	}
	if m.Run == nil {
		m.Run = func(ctx context.Context, name string, args ...string) error {
			output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
			}
			return nil
		}
	}
	return m
}

func regular(path string) (bool, error) {
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return false, err
	}
	if !parent.IsDir() || !rootControlled(parent) {
		return false, fmt.Errorf("refusing untrusted directory %s", filepath.Dir(path))
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("refusing unexpected file type at %s", path)
	}
	if !rootControlled(info) {
		return false, fmt.Errorf("refusing untrusted ownership or mode at %s", path)
	}
	return true, nil
}

func rootControlled(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0 && info.Mode().Perm()&022 == 0
}

func (m Manager) restoreUnits(names []string) error {
	var result error
	for _, name := range names {
		content, err := units.ReadFile("systemd/" + name)
		if err == nil {
			path := filepath.Join(m.UnitDir, name)
			var file *os.File
			file, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err == nil {
				_, err = file.Write(content)
				err = errors.Join(err, file.Close())
			}
		}
		result = errors.Join(result, err)
	}
	return result
}

func (m Manager) inspect() ([]string, []string, error) {
	m = m.defaults()
	remove, active := []string{}, []string{}
	for _, name := range names {
		path := filepath.Join(m.UnitDir, name)
		exists, err := regular(path)
		if err != nil {
			return nil, nil, err
		}
		if !exists {
			continue
		}
		wanted, err := units.ReadFile("systemd/" + name)
		if err != nil {
			return nil, nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, nil, err
		}
		if info.Size() != int64(len(wanted)) {
			return nil, nil, fmt.Errorf("refusing modified unit %s", path)
		}
		installed, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		if string(installed) != string(wanted) {
			return nil, nil, fmt.Errorf("refusing modified unit %s", path)
		}
		remove, active = append(remove, path), append(active, name)
	}
	exists, err := regular(m.Binary)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		build, err := buildinfo.ReadFile(m.Binary)
		if err != nil || build.Path != m.ExpectedMain {
			return nil, nil, fmt.Errorf("refusing unrecognized executable %s", m.Binary)
		}
		remove = append(remove, m.Binary)
	}
	return remove, active, nil
}

func (m Manager) Execute(ctx context.Context, apply bool, out, progress io.Writer) error {
	m = m.defaults()
	if apply && os.Geteuid() != 0 {
		return errors.New("uninstall --apply requires root")
	}
	remove, active, err := m.inspect()
	if err != nil {
		return err
	}
	report := Report{
		Action: "preview", Remove: remove,
		Retain:  []string{"/etc/hostlens configuration, profiles, and tokens", "HostLens service users and groups", "shared system journals", "administrator-granted permissions"},
		Message: "Service identities remain because HostLens cannot prove they own no files elsewhere. Review them separately after removal.",
	}
	if !apply {
		return json.NewEncoder(out).Encode(report)
	}
	if len(active) > 0 {
		fmt.Fprintln(progress, "HOSTLENS_STAGE: stopping and disabling HostLens units")
		for _, name := range active {
			step, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := m.Run(step, "systemctl", "disable", "--now", name)
			cancel()
			if err != nil {
				return fmt.Errorf("stopping %s: %w", name, err)
			}
		}
	}
	if len(active) > 0 {
		fmt.Fprintln(progress, "HOSTLENS_STAGE: removing verified HostLens units")
	}
	var removed []string
	for _, name := range active {
		path := filepath.Join(m.UnitDir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.Join(err, m.restoreUnits(removed))
		}
		removed = append(removed, name)
	}
	if len(active) > 0 {
		fmt.Fprintln(progress, "HOSTLENS_STAGE: reloading systemd")
		step, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = m.Run(step, "systemctl", "daemon-reload")
		cancel()
		if err != nil {
			return errors.Join(fmt.Errorf("reloading systemd: %w", err), m.restoreUnits(removed))
		}
	}
	if len(remove) > 0 && remove[len(remove)-1] == m.Binary {
		fmt.Fprintln(progress, "HOSTLENS_STAGE: removing verified HostLens binary")
		if err := os.Remove(m.Binary); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	report.Action = "removed"
	return json.NewEncoder(out).Encode(report)
}
