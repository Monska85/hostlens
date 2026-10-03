package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const maxConfigBytes = 1 << 20

type Config struct {
	Version        int      `yaml:"version"`
	Listen         string   `yaml:"listen"`
	ReadOnly       bool     `yaml:"read_only"`
	TokenStore     string   `yaml:"token_store"`
	ProfileDir     string   `yaml:"profile_dir"`
	ActiveProfiles []string `yaml:"active_profiles"`
	DockerSocket   string   `yaml:"docker_socket"`
	ObserverSocket string   `yaml:"observer_socket"`
	RepairSocket   string   `yaml:"repair_socket"`
	GatewayUID     uint32   `yaml:"gateway_uid"`
	ObserverUID    uint32   `yaml:"observer_uid"`
	RepairUID      uint32   `yaml:"repair_uid"`
	SharedGID      uint32   `yaml:"shared_gid"`
	TokenGID       uint32   `yaml:"token_gid"`
	TLSCert        string   `yaml:"tls_cert,omitempty"`
	TLSKey         string   `yaml:"tls_key,omitempty"`
}

func decodeYAML(reader io.Reader, into any) error {
	data, err := io.ReadAll(io.LimitReader(reader, maxConfigBytes+1))
	if err != nil || len(data) > maxConfigBytes {
		return errors.New("configuration exceeds byte ceiling or cannot be read")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(into); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing configuration document")
	}
	return nil
}

func ValidProfileName(name string) bool {
	if name == "" || len(name) > 64 || strings.ContainsAny(name, "/\\.\x00\n\r") {
		return false
	}
	for _, char := range name {
		if char != '-' && char != '_' && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func (c Config) Validate() error {
	if c.Version != 2 {
		return errors.New("configuration version 2 required")
	}
	host, portText, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("listener port must be between 1 and 65535")
	}
	if host != "127.0.0.1" && host != "::1" && (c.TLSCert == "" || c.TLSKey == "") {
		return errors.New("non-loopback listener requires TLS certificate and key")
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return errors.New("TLS certificate and key must be configured together")
	}
	for _, value := range []string{c.TokenStore, c.ProfileDir} {
		if !filepath.IsAbs(value) || value == "/" || filepath.Clean(value) != value {
			return errors.New("absolute token store and profile directory required")
		}
	}
	if c.DockerSocket != "" && (!filepath.IsAbs(c.DockerSocket) || filepath.Clean(c.DockerSocket) != c.DockerSocket) {
		return errors.New("Docker socket must be an absolute local path")
	}
	for _, value := range []string{c.ObserverSocket, c.RepairSocket, c.TLSCert, c.TLSKey} {
		if value != "" && (!filepath.IsAbs(value) || filepath.Clean(value) != value) {
			return errors.New("IPC and TLS paths must be absolute and clean")
		}
	}
	if c.ObserverSocket != "" && c.ObserverSocket == c.RepairSocket ||
		c.ObserverSocket != "" && c.ObserverSocket == c.DockerSocket ||
		c.RepairSocket != "" && c.RepairSocket == c.DockerSocket ||
		c.ObserverSocket == c.TokenStore || c.RepairSocket == c.TokenStore {
		return errors.New("IPC, Docker, and token paths must be distinct")
	}
	if c.GatewayUID == 0 || c.ObserverUID == 0 || c.GatewayUID == c.ObserverUID || c.GatewayUID == c.RepairUID || c.ObserverUID == c.RepairUID {
		return errors.New("gateway and observer must be non-root and service UIDs must be distinct")
	}
	if c.SharedGID == 0 || c.TokenGID == 0 || c.SharedGID == c.TokenGID {
		return errors.New("distinct non-root IPC and token reader group GIDs required")
	}
	return nil
}

func Load(path string, tools map[string]Effect) (Config, Policy, error) {
	file, err := OpenTrusted(path)
	if err != nil {
		return Config{}, Policy{}, err
	}
	defer file.Close()
	config := Config{ReadOnly: true}
	if err := decodeYAML(file, &config); err != nil {
		return Config{}, Policy{}, err
	}
	if err := config.Validate(); err != nil {
		return Config{}, Policy{}, err
	}
	for _, candidate := range []struct {
		path    string
		private bool
	}{{config.TLSCert, false}, {config.TLSKey, true}} {
		if candidate.path == "" {
			continue
		}
		file, err := OpenTrusted(candidate.path)
		if err != nil {
			return Config{}, Policy{}, errors.New("protected TLS certificate and key files required")
		}
		info, err := file.Stat()
		file.Close()
		if err != nil || candidate.private && info.Mode().Perm()&0007 != 0 {
			return Config{}, Policy{}, errors.New("protected TLS certificate and key files required")
		}
	}
	if err := TrustedDirectory(config.ProfileDir); err != nil {
		return Config{}, Policy{}, errors.New("protected profile directory required")
	}
	root, err := os.OpenRoot(config.ProfileDir)
	if err != nil {
		return Config{}, Policy{}, err
	}
	defer root.Close()
	profiles := make(map[string]Profile)
	var load func(string, int) error
	load = func(name string, depth int) error {
		if !ValidProfileName(name) || depth > 32 || len(profiles) > 128 {
			return errors.New("invalid profile name or include limit")
		}
		if _, ok := profiles[name]; ok {
			return nil
		}
		file, err := root.Open(name + ".yaml")
		if err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || !trustedOwner(info) {
			file.Close()
			return fmt.Errorf("profile %q is not a protected regular file", name)
		}
		var profile Profile
		decodeErr := decodeYAML(file, &profile)
		closeErr := file.Close()
		if err := errors.Join(decodeErr, closeErr); err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
		profiles[name] = profile
		for _, include := range profile.Includes {
			if err := load(include, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, name := range config.ActiveProfiles {
		if err := load(name, 0); err != nil {
			return Config{}, Policy{}, err
		}
	}
	policy, err := Compile(profiles, config.ActiveProfiles, tools)
	if err != nil {
		return Config{}, Policy{}, err
	}
	return config, policy, nil
}
