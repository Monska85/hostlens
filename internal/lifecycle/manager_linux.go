//go:build linux

package lifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/server"
	"github.com/Monska85/hostlens/internal/uninstall"
	"go.yaml.in/yaml/v3"
)

var unitNames = []string{"hostlens-gateway.service", "hostlens-observer.service", "hostlens-repair.service"}
var groupNames = []string{"hostlens-shared", "hostlens-tokens"}
var userNames = []string{"hostlens-gateway", "hostlens-observer"}
var profileNames = []string{"host.yaml", "docker-readonly.yaml", "docker-evidence.yaml", "services-readonly.yaml", "service-logs.yaml", "repair-example.yaml"}

const provenancePath = "/etc/hostlens/unit-provenance.json"

type Manager struct {
	Source string
	Root   string // set only by isolated tests
	Run    func(context.Context, string, ...string) error
}

type Plan struct {
	Action       string   `json:"action"`
	Version      string   `json:"version"`
	Digest       string   `json:"plan_digest"`
	Create       []string `json:"create,omitempty"`
	Replace      []string `json:"replace,omitempty"`
	Retain       []string `json:"retain,omitempty"`
	Start        []string `json:"start,omitempty"`
	Source       string   `json:"source"`
	sourceDigest string
}

type manifest struct {
	Version      string            `json:"version"`
	Schema       int               `json:"schema"`
	Architecture string            `json:"architecture"`
	Checksums    map[string]string `json:"checksums"`
}

type unitProvenance struct {
	Checksums map[string]string `json:"checksums"`
}

func packagedUnitProvenance(data map[string][]byte) unitProvenance {
	record := unitProvenance{Checksums: make(map[string]string, len(unitNames))}
	for _, name := range unitNames {
		digest := sha256.Sum256(data["systemd/"+name])
		record.Checksums[name] = hex.EncodeToString(digest[:])
	}
	return record
}

func releaseProfiles(release manifest) ([]string, error) {
	names := []string{}
	for member := range release.Checksums {
		if !strings.HasPrefix(member, "profiles/") {
			continue
		}
		name := strings.TrimPrefix(member, "profiles/")
		if !strings.HasSuffix(name, ".yaml") || strings.Contains(name, "/") || !core.ValidProfileName(strings.TrimSuffix(name, ".yaml")) {
			return nil, fmt.Errorf("invalid release profile %s", member)
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

func unitActive(ctx context.Context, name string) (bool, error) {
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(checkCtx, "systemctl", "show", "--property=ActiveState", "--value", name).Output()
	if err != nil {
		return false, fmt.Errorf("cannot determine activation state of %s: %w", name, err)
	}
	switch strings.TrimSpace(string(output)) {
	case "active", "activating", "reloading", "deactivating":
		return true, nil
	case "inactive", "failed":
		return false, nil
	default:
		return false, fmt.Errorf("unrecognized activation state for %s", name)
	}
}

func gatewayReady(ctx context.Context, config core.Config) error {
	scheme := "http"
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 300 * time.Millisecond}).DialContext(ctx, "tcp", config.Listen)
	}}
	if config.TLSCert != "" {
		file, err := core.OpenTrusted(config.TLSCert)
		if err != nil {
			return err
		}
		certificate, readErr := io.ReadAll(io.LimitReader(file, (1<<20)+1))
		if err := errors.Join(readErr, file.Close()); err != nil || len(certificate) > 1<<20 {
			return errors.New("gateway TLS certificate unavailable")
		}
		block, _ := pem.Decode(certificate)
		if block == nil || block.Type != "CERTIFICATE" {
			return errors.New("invalid gateway TLS certificate")
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return err
		}
		want := sha256.Sum256(block.Bytes)
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true, VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 || sha256.Sum256(state.PeerCertificates[0].Raw) != want {
				return errors.New("gateway TLS certificate differs from configured certificate")
			}
			return nil
		}}
		scheme = "https"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+config.Listen+"/mcp", nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Transport: transport, Timeout: 500 * time.Millisecond}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || response.Header.Get("Cache-Control") != "no-store" {
		return errors.New("gateway health response unavailable")
	}
	return nil
}

func waitReady(ctx context.Context, name string, config core.Config) error {
	readyCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var stableSince time.Time
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		active, stateErr := unitActive(readyCtx, name)
		if stateErr == nil && active {
			address, network := config.Listen, "tcp"
			switch name {
			case "hostlens-observer.service":
				address, network = config.ObserverSocket, "unix"
			case "hostlens-repair.service":
				address, network = config.RepairSocket, "unix"
			}
			probeCtx, stopProbe := context.WithTimeout(readyCtx, 300*time.Millisecond)
			conn, dialErr := (&net.Dialer{}).DialContext(probeCtx, network, address)
			stopProbe()
			if dialErr == nil {
				conn.Close()
				if name == "hostlens-gateway.service" {
					probeCtx, cancel := context.WithTimeout(readyCtx, 500*time.Millisecond)
					dialErr = gatewayReady(probeCtx, config)
					cancel()
				}
			}
			if dialErr == nil {
				if stableSince.IsZero() {
					stableSince = time.Now()
				}
				if time.Since(stableSince) >= time.Second {
					return nil
				}
			} else {
				stableSince = time.Time{}
			}
		} else {
			stableSince = time.Time{}
		}
		select {
		case <-readyCtx.Done():
			return fmt.Errorf("%s did not become ready: %w", name, readyCtx.Err())
		case <-ticker.C:
		}
	}
}

func (record unitProvenance) matches(name string, content []byte) bool {
	digest := sha256.Sum256(content)
	return record.Checksums[name] == hex.EncodeToString(digest[:])
}

func readUnitProvenance(path string) (unitProvenance, bool, error) {
	info, err := trusted(path)
	if err != nil || info == nil {
		return unitProvenance{}, false, err
	}
	if info.Size() > 4096 {
		return unitProvenance{}, false, errors.New("unit provenance exceeds byte ceiling")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return unitProvenance{}, false, err
	}
	var record unitProvenance
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return unitProvenance{}, false, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF || len(record.Checksums) != len(unitNames) {
		return unitProvenance{}, false, errors.New("invalid unit provenance")
	}
	return record, true, nil
}

type snapshot struct {
	path   string
	data   []byte
	mode   os.FileMode
	uid    int
	gid    int
	exists bool
}

func (m Manager) path(path string) string {
	if m.Root == "" {
		return path
	}
	return filepath.Join(m.Root, strings.TrimPrefix(path, "/"))
}

func (m Manager) runner() func(context.Context, string, ...string) error {
	if m.Run != nil {
		return m.Run
	}
	return func(ctx context.Context, name string, args ...string) error {
		output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
		}
		return nil
	}
}

func trusted(path string) (os.FileInfo, error) {
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		if _, err := os.Lstat(parent); err == nil {
			if err := core.TrustedDirectory(parent); err != nil {
				return nil, fmt.Errorf("unsafe target directory %s: %w", parent, err)
			}
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if parent == "/" {
			return nil, errors.New("target root unavailable")
		}
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return nil, fmt.Errorf("untrusted file %s", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return nil, fmt.Errorf("untrusted owner %s", path)
	}
	return info, nil
}

func (m Manager) archive() (manifest, map[string][]byte, error) {
	root, err := os.OpenRoot(m.Source)
	if err != nil {
		return manifest{}, nil, err
	}
	defer root.Close()
	info, err := root.Lstat("release.json")
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return manifest{}, nil, errors.New("invalid release manifest")
	}
	file, err := root.Open("release.json")
	if err != nil {
		return manifest{}, nil, err
	}
	defer file.Close()
	manifestData, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil || len(manifestData) > 1<<20 {
		return manifest{}, nil, errors.New("release manifest exceeds byte ceiling")
	}
	var release manifest
	decoder := json.NewDecoder(bytes.NewReader(manifestData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&release); err != nil {
		return manifest{}, nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return manifest{}, nil, errors.New("trailing release manifest")
	}
	if release.Schema != 2 || release.Architecture != runtime.GOARCH || release.Version == "" {
		return manifest{}, nil, errors.New("incompatible release manifest")
	}
	if len(release.Checksums) < 8 || len(release.Checksums) > 100 {
		return manifest{}, nil, errors.New("invalid release members")
	}
	required := []string{"hostlens", "config.example.yaml"}
	for _, name := range profileNames {
		required = append(required, "profiles/"+name)
	}
	for _, name := range unitNames {
		required = append(required, "systemd/"+name)
	}
	for _, name := range required {
		if _, ok := release.Checksums[name]; !ok {
			return manifest{}, nil, fmt.Errorf("missing release member %s", name)
		}
	}
	if _, err := releaseProfiles(release); err != nil {
		return manifest{}, nil, err
	}
	data := make(map[string][]byte, len(release.Checksums))
	totalBytes := 0
	for name, want := range release.Checksums {
		if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "..") || strings.Contains(name, "\\") {
			return manifest{}, nil, errors.New("unsafe release member")
		}
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<20 {
			return manifest{}, nil, fmt.Errorf("invalid release member %s", name)
		}
		member, err := root.Open(name)
		if err != nil {
			return manifest{}, nil, err
		}
		content, readErr := io.ReadAll(io.LimitReader(member, (64<<20)+1))
		if err := errors.Join(readErr, member.Close()); err != nil || len(content) > 64<<20 {
			return manifest{}, nil, fmt.Errorf("invalid release member %s", name)
		}
		totalBytes += len(content)
		if totalBytes > 128<<20 {
			return manifest{}, nil, errors.New("release exceeds total byte ceiling")
		}
		digest := sha256.Sum256(content)
		if hex.EncodeToString(digest[:]) != want {
			return manifest{}, nil, fmt.Errorf("release checksum mismatch: %s", name)
		}
		data[name] = content
	}
	build, err := buildinfo.Read(bytes.NewReader(data["hostlens"]))
	if err != nil || build.Path != "github.com/Monska85/hostlens/cmd/hostlens" {
		return manifest{}, nil, errors.New("unrecognized release executable")
	}
	return release, data, nil
}

func addFingerprint(state hash.Hash, name string, data []byte) {
	digest := sha256.Sum256(data)
	state.Write([]byte(name))
	state.Write([]byte{0})
	state.Write(digest[:])
}

func releaseFingerprint(release manifest) string {
	state := sha256.New()
	keys := make([]string, 0, len(release.Checksums))
	for name := range release.Checksums {
		keys = append(keys, name)
	}
	slices.Sort(keys)
	for _, name := range keys {
		state.Write([]byte(name))
		state.Write([]byte{0})
		state.Write([]byte(release.Checksums[name]))
		state.Write([]byte{0})
	}
	return hex.EncodeToString(state.Sum(nil))
}

func (m Manager) Preview(action string, start bool) (Plan, error) {
	if action != "install" && action != "upgrade" {
		return Plan{}, errors.New("invalid lifecycle action")
	}
	if action == "upgrade" && os.Geteuid() != 0 {
		return Plan{}, errors.New("upgrade preview requires root to inspect protected installation state")
	}
	release, _, err := m.archive()
	if err != nil {
		return Plan{}, err
	}
	profiles, err := releaseProfiles(release)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Action: action, Version: release.Version, Source: m.Source, Create: []string{}, Replace: []string{}, Retain: []string{"configuration and profiles already present", "token store and credentials", "administrator permissions"}, Start: []string{}}
	state := sha256.New()
	state.Write([]byte(fmt.Sprintf("%s:%t", action, start)))
	state.Write([]byte(release.Version + ":" + release.Architecture))
	plan.sourceDigest = releaseFingerprint(release)
	state.Write([]byte(plan.sourceDigest))
	existingGroups := map[string]string{}
	for _, name := range groupNames {
		if item, err := user.LookupGroup(name); err != nil {
			plan.Create = append(plan.Create, "group:"+name)
		} else {
			for _, otherID := range existingGroups {
				if item.Gid == otherID || item.Gid == "0" {
					return Plan{}, errors.New("conflicting service groups")
				}
			}
			existingGroups[name] = item.Gid
			state.Write([]byte("group:" + name + ":" + item.Gid))
			plan.Retain = append(plan.Retain, "group:"+name)
		}
	}
	existingUsers := map[string]string{}
	for _, name := range userNames {
		if item, err := user.Lookup(name); err != nil {
			plan.Create = append(plan.Create, "user:"+name)
		} else {
			shared, groupErr := user.LookupGroup("hostlens-shared")
			if groupErr != nil || item.Gid != shared.Gid || item.Uid == "0" {
				return Plan{}, fmt.Errorf("conflicting service identity %s", name)
			}
			for _, otherID := range existingUsers {
				if item.Uid == otherID {
					return Plan{}, errors.New("shared service UID")
				}
			}
			existingUsers[name] = item.Uid
			state.Write([]byte("user:" + name + ":" + item.Uid + ":" + item.Gid))
			plan.Retain = append(plan.Retain, "user:"+name)
		}
	}
	for _, target := range []string{"/etc/hostlens", "/etc/hostlens/profiles", "/etc/hostlens/secrets"} {
		path := m.path(target)
		state.Write([]byte(target))
		info, statErr := os.Lstat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			plan.Create = append(plan.Create, target)
			continue
		}
		if statErr != nil {
			return Plan{}, statErr
		}
		if err := core.TrustedDirectory(path); err != nil {
			return Plan{}, fmt.Errorf("untrusted installation directory %s: %w", target, err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		state.Write([]byte(fmt.Sprintf("%d:%d:%d", stat.Uid, stat.Gid, info.Mode().Perm())))
		plan.Retain = append(plan.Retain, target)
	}
	paths := []string{"/usr/local/bin/hostlens", "/etc/hostlens/config.yaml"}
	oldUnits, hasProvenance, err := readUnitProvenance(m.path(provenancePath))
	if err != nil {
		return Plan{}, err
	}
	for _, name := range profiles {
		paths = append(paths, "/etc/hostlens/profiles/"+name)
	}
	for _, name := range unitNames {
		paths = append(paths, "/etc/systemd/system/"+name)
	}
	paths = append(paths, provenancePath)
	for _, target := range paths {
		path := m.path(target)
		info, err := trusted(path)
		if err != nil {
			return Plan{}, err
		}
		state.Write([]byte(target))
		if info == nil {
			if action == "upgrade" && target == "/usr/local/bin/hostlens" {
				return Plan{}, fmt.Errorf("installed file missing: %s", target)
			}
			plan.Create = append(plan.Create, target)
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return Plan{}, err
		}
		addFingerprint(state, target, content)
		if strings.HasPrefix(target, "/etc/systemd/system/") && action == "upgrade" {
			name := filepath.Base(target)
			if hasProvenance && !oldUnits.matches(name, content) {
				return Plan{}, fmt.Errorf("modified installed unit: %s", target)
			}
			if !hasProvenance {
				packaged, e := uninstall.PackagedUnit(name)
				if e != nil || !slices.Equal(content, packaged) {
					return Plan{}, fmt.Errorf("modified installed unit: %s", target)
				}
			}
		}
		if target == "/usr/local/bin/hostlens" {
			build, err := buildinfo.ReadFile(path)
			if err != nil || build.Path != "github.com/Monska85/hostlens/cmd/hostlens" {
				return Plan{}, errors.New("unrecognized installed executable")
			}
			if action == "install" {
				return Plan{}, errors.New("HostLens is already installed; use upgrade")
			}
			plan.Replace = append(plan.Replace, target)
		} else if strings.HasPrefix(target, "/etc/systemd/system/") || target == provenancePath {
			if action == "install" && target != provenancePath {
				return Plan{}, fmt.Errorf("unit already exists: %s", target)
			}
			plan.Replace = append(plan.Replace, target)
		} else {
			plan.Retain = append(plan.Retain, target)
		}
	}
	if action == "install" && start {
		if info, inspectErr := trusted(m.path("/etc/hostlens/config.yaml")); inspectErr != nil {
			return Plan{}, inspectErr
		} else if info != nil {
			configured, _, loadErr := core.Load(m.path("/etc/hostlens/config.yaml"), server.ToolEffects())
			if loadErr != nil {
				return Plan{}, fmt.Errorf("retained configuration: %w", loadErr)
			}
			if !configured.ReadOnly {
				return Plan{}, errors.New("install --start requires read-only configuration")
			}
		}
	}
	if action == "upgrade" {
		if _, _, err := core.Load(m.path("/etc/hostlens/config.yaml"), server.ToolEffects()); err != nil {
			return Plan{}, fmt.Errorf("installed configuration: %w", err)
		}
		if _, err := exec.LookPath("systemctl"); err != nil {
			return Plan{}, errors.New("systemctl required for upgrade")
		}
		for _, unit := range unitNames {
			active, stateErr := unitActive(context.Background(), unit)
			if stateErr != nil {
				return Plan{}, stateErr
			}
			if active {
				plan.Start = append(plan.Start, unit)
			}
		}
	}
	if start && action == "install" {
		plan.Start = append(plan.Start, "hostlens-gateway.service")
	}
	for _, unit := range plan.Start {
		state.Write([]byte("active:" + unit))
	}
	plan.Digest = hex.EncodeToString(state.Sum(nil))
	return plan, nil
}

func capture(path string) (snapshot, error) {
	result := snapshot{path: path}
	info, err := trusted(path)
	if err != nil {
		return result, err
	}
	if info == nil {
		return result, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	result.data, result.mode, result.uid, result.gid, result.exists = data, info.Mode().Perm(), int(stat.Uid), int(stat.Gid), true
	return result, nil
}

func write(path string, content []byte, mode os.FileMode, uid, gid int) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".hostlens-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if err := file.Chown(uid, gid); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	parent, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(parent.Sync(), parent.Close())
}

func configuredTemplate(template []byte, gateway, observer, shared, tokens int) ([]byte, error) {
	if len(template) > 1<<20 {
		return nil, errors.New("release configuration exceeds byte ceiling")
	}
	config := core.Config{ReadOnly: true}
	decoder := yaml.NewDecoder(bytes.NewReader(template))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("trailing release configuration document")
	}
	var document yaml.Node
	if err := yaml.Unmarshal(template, &document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("invalid release configuration document")
	}
	ids := map[string]int{"gateway_uid": gateway, "observer_uid": observer, "shared_gid": shared, "token_gid": tokens}
	for index := 0; index < len(document.Content[0].Content); index += 2 {
		key, value := document.Content[0].Content[index], document.Content[0].Content[index+1]
		if id, ok := ids[key.Value]; ok {
			value.Value, value.Tag = strconv.Itoa(id), "!!int"
			delete(ids, key.Value)
		}
	}
	if len(ids) != 0 {
		return nil, errors.New("release configuration lacks service identity fields")
	}
	config.GatewayUID, config.ObserverUID = uint32(gateway), uint32(observer)
	config.SharedGID, config.TokenGID = uint32(shared), uint32(tokens)
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("release configuration invalid: %w", err)
	}
	return yaml.Marshal(&document)
}

func (m Manager) Apply(ctx context.Context, action, digest string, start bool, progress io.Writer) (result Plan, err error) {
	if os.Geteuid() != 0 {
		return Plan{}, errors.New("lifecycle apply requires root")
	}
	plan, err := m.Preview(action, start)
	if err != nil {
		return Plan{}, err
	}
	if digest == "" || digest != plan.Digest {
		return Plan{}, errors.New("plan changed; preview again")
	}
	release, data, err := m.archive()
	if err != nil || release.Version != plan.Version {
		return Plan{}, errors.New("release changed; preview again")
	}
	if releaseFingerprint(release) != plan.sourceDigest {
		return Plan{}, errors.New("release changed; preview again")
	}
	profiles, err := releaseProfiles(release)
	if err != nil {
		return Plan{}, err
	}
	nativeRun := m.runner()
	run := func(parent context.Context, name string, args ...string) error {
		step, cancel := context.WithTimeout(parent, 30*time.Second)
		defer cancel()
		return nativeRun(step, name, args...)
	}
	changed := []snapshot{}
	createdDirectories := []string{}
	priorActive := []string{}
	started := []string{}
	unitsInstalled := false
	backupDir := ""
	defer func() {
		if err == nil {
			if backupDir != "" {
				if cleanErr := os.RemoveAll(backupDir); cleanErr != nil {
					err = fmt.Errorf("upgrade activated; remove rollback copy %s: %w", backupDir, cleanErr)
				}
			}
			return
		}
		rollback, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var restoreErr error
		if action == "install" && start && unitsInstalled {
			restoreErr = errors.Join(restoreErr, run(rollback, "systemctl", "disable", "--now", "hostlens-gateway.service"))
		}
		for _, unit := range started {
			restoreErr = errors.Join(restoreErr, run(rollback, "systemctl", "stop", unit))
		}
		for i := len(changed) - 1; i >= 0; i-- {
			item := changed[i]
			if item.exists {
				restoreErr = errors.Join(restoreErr, write(item.path, item.data, item.mode, item.uid, item.gid))
			} else {
				removeErr := os.Remove(item.path)
				if !errors.Is(removeErr, os.ErrNotExist) {
					restoreErr = errors.Join(restoreErr, removeErr)
				}
			}
		}
		for i := len(createdDirectories) - 1; i >= 0; i-- {
			restoreErr = errors.Join(restoreErr, os.Remove(createdDirectories[i]))
		}
		restoreErr = errors.Join(restoreErr, run(rollback, "systemctl", "daemon-reload"))
		for _, unit := range priorActive {
			restoreErr = errors.Join(restoreErr, run(rollback, "systemctl", "start", unit))
		}
		if restoreErr == nil && backupDir != "" {
			restoreErr = os.RemoveAll(backupDir)
		}
		if restoreErr != nil && backupDir != "" {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("rollback copy retained at %s", backupDir))
		}
		err = errors.Join(err, restoreErr)
	}()
	for _, name := range groupNames {
		if _, lookupErr := user.LookupGroup(name); lookupErr != nil {
			if err = run(ctx, "groupadd", "--system", name); err != nil {
				return Plan{}, err
			}
		}
	}
	for _, name := range userNames {
		if _, lookupErr := user.Lookup(name); lookupErr != nil {
			if err = run(ctx, "useradd", "--system", "--no-create-home", "--gid", "hostlens-shared", "--shell", "/usr/sbin/nologin", name); err != nil {
				return Plan{}, err
			}
		}
	}
	lookupID := func(group bool, name string) (int, error) {
		var value string
		if group {
			item, e := user.LookupGroup(name)
			if e != nil {
				return 0, e
			}
			value = item.Gid
		} else {
			item, e := user.Lookup(name)
			if e != nil {
				return 0, e
			}
			value = item.Uid
		}
		return strconv.Atoi(value)
	}
	shared, err := lookupID(true, "hostlens-shared")
	if err != nil {
		return Plan{}, err
	}
	tokens, err := lookupID(true, "hostlens-tokens")
	if err != nil {
		return Plan{}, err
	}
	gateway, err := lookupID(false, "hostlens-gateway")
	if err != nil {
		return Plan{}, err
	}
	observer, err := lookupID(false, "hostlens-observer")
	if err != nil {
		return Plan{}, err
	}
	for _, item := range []struct {
		path string
		gid  int
	}{{"/etc/hostlens", shared}, {"/etc/hostlens/profiles", shared}, {"/etc/hostlens/secrets", tokens}} {
		path := m.path(item.path)
		if _, statErr := os.Lstat(path); errors.Is(statErr, os.ErrNotExist) {
			if err = os.Mkdir(path, 0750); err != nil {
				return Plan{}, err
			}
			createdDirectories = append(createdDirectories, path)
			if err = os.Chown(path, 0, item.gid); err != nil {
				return Plan{}, err
			}
		} else if statErr != nil {
			return Plan{}, statErr
		}
	}
	if action == "upgrade" {
		backupDir, err = os.MkdirTemp(m.path("/etc/hostlens"), ".rollback-*")
		if err != nil {
			return Plan{}, err
		}
		fmt.Fprintf(progress, "HOSTLENS_STAGE: protected rollback copy %s\n", backupDir)
		for _, target := range append([]string{"/usr/local/bin/hostlens", provenancePath}, unitNames...) {
			path := target
			if !strings.HasPrefix(target, "/") {
				path = "/etc/systemd/system/" + target
			}
			old, captureErr := capture(m.path(path))
			if captureErr != nil {
				return Plan{}, captureErr
			}
			if !old.exists {
				continue
			}
			copyPath := filepath.Join(backupDir, filepath.Base(path))
			if err = write(copyPath, old.data, old.mode, old.uid, old.gid); err != nil {
				return Plan{}, err
			}
			verified, verifyErr := capture(copyPath)
			if verifyErr != nil || !verified.exists || !bytes.Equal(verified.data, old.data) || verified.mode != old.mode || verified.uid != old.uid || verified.gid != old.gid {
				err = errors.Join(verifyErr, fmt.Errorf("rollback copy verification failed: %s", path))
				return Plan{}, err
			}
		}
		for _, unit := range unitNames {
			active, stateErr := unitActive(ctx, unit)
			if stateErr != nil {
				return Plan{}, stateErr
			}
			if active {
				priorActive = append(priorActive, unit)
			}
		}
		for _, unit := range priorActive {
			if err = run(ctx, "systemctl", "stop", unit); err != nil {
				return Plan{}, err
			}
		}
	}
	put := func(target string, content []byte, mode os.FileMode, gid int, replace bool) error {
		path := m.path(target)
		old, e := capture(path)
		if e != nil {
			return e
		}
		if old.exists && !replace {
			return nil
		}
		changed = append(changed, old)
		if e = write(path, content, mode, 0, gid); e != nil {
			return e
		}
		return nil
	}
	fmt.Fprintln(progress, "HOSTLENS_STAGE: installing verified release files")
	config, configErr := configuredTemplate(data["config.example.yaml"], gateway, observer, shared, tokens)
	if configErr != nil {
		return Plan{}, configErr
	}
	if err = put("/usr/local/bin/hostlens", data["hostlens"], 0755, 0, true); err != nil {
		return Plan{}, err
	}
	if err = put("/etc/hostlens/config.yaml", config, 0640, shared, false); err != nil {
		return Plan{}, err
	}
	for _, name := range profiles {
		if err = put("/etc/hostlens/profiles/"+name, data["profiles/"+name], 0640, shared, false); err != nil {
			return Plan{}, err
		}
	}
	for _, name := range unitNames {
		if err = put("/etc/systemd/system/"+name, data["systemd/"+name], 0644, 0, true); err != nil {
			return Plan{}, err
		}
	}
	provenance, marshalErr := json.Marshal(packagedUnitProvenance(data))
	if marshalErr != nil {
		return Plan{}, marshalErr
	}
	if err = put(provenancePath, provenance, 0600, 0, true); err != nil {
		return Plan{}, err
	}
	unitsInstalled = true
	configured, _, loadErr := core.Load(m.path("/etc/hostlens/config.yaml"), server.ToolEffects())
	if loadErr != nil {
		return Plan{}, fmt.Errorf("installed configuration invalid: %w", loadErr)
	}
	if action == "install" && start && !configured.ReadOnly {
		return Plan{}, errors.New("install --start requires read-only configuration")
	}
	if err = run(ctx, "systemctl", "daemon-reload"); err != nil {
		return Plan{}, err
	}
	toStart := slices.Clone(priorActive)
	if action == "install" && start {
		toStart = append(toStart, "hostlens-gateway.service")
	}
	for _, unit := range toStart {
		if err = run(ctx, "systemctl", "start", unit); err != nil {
			return Plan{}, err
		}
		started = append(started, unit)
		if err = waitReady(ctx, unit, configured); err != nil {
			return Plan{}, err
		}
	}
	if action == "install" && start {
		if err = run(ctx, "systemctl", "enable", "hostlens-gateway.service"); err != nil {
			return Plan{}, err
		}
	}
	return plan, nil
}
