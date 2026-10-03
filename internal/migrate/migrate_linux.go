//go:build linux

package migrate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/bmatcuk/doublestar/v4"
	"go.yaml.in/yaml/v3"
)

const maxInput = 4 << 20

type Options struct {
	From, Output                                            string
	GatewayUID, ObserverUID, RepairUID, SharedGID, TokenGID uint32
}

type Report struct {
	SourceVersion    int      `json:"source_version"`
	TokensPreserved  int      `json:"tokens_preserved"`
	ProfilesUnmapped []string `json:"profiles_unmapped,omitempty"`
	Actions          []string `json:"actions"`
}

type oldConfig struct {
	Version   int    `yaml:"version"`
	Mode      string `yaml:"mode"`
	Privilege string `yaml:"privilege"`
	MCP       struct {
		ReadOnly bool `yaml:"read_only"`
	} `yaml:"mcp"`
	Metrics struct {
		Enabled        bool `yaml:"enabled"`
		AllowAnonymous bool `yaml:"allow_anonymous"`
	} `yaml:"metrics"`
	Allow           oldRules  `yaml:"allow"`
	Deny            oldRules  `yaml:"deny"`
	ProfileDirs     []string  `yaml:"profile_dirs"`
	Socket          string    `yaml:"socket"`
	AdminSocket     string    `yaml:"admin_socket"`
	GatewayUser     string    `yaml:"gateway_user"`
	DiagnosticsUser string    `yaml:"diagnostics_user"`
	Limits          oldLimits `yaml:"limits"`
	Health          oldHealth `yaml:"health"`
	Logging         struct {
		Level                string `yaml:"level"`
		AuditSuccessfulCalls bool   `yaml:"audit_successful_calls"`
	} `yaml:"logging"`
	Remediation struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"remediation"`
	Server struct {
		Bind []string `yaml:"bind"`
		Port int      `yaml:"port"`
		TLS  struct {
			Enabled  bool   `yaml:"enabled"`
			CertFile string `yaml:"cert_file"`
			KeyFile  string `yaml:"key_file"`
		} `yaml:"tls"`
		AllowInsecureHTTP bool     `yaml:"allow_insecure_http"`
		TrustedProxies    []string `yaml:"trusted_proxies"`
		ClientIPHeader    string   `yaml:"client_ip_header"`
		AllowedOrigins    []string `yaml:"allowed_origins"`
	} `yaml:"server"`
	TokenStore string   `yaml:"token_store"`
	Profiles   []string `yaml:"profiles"`
	Docker     struct {
		Enabled        bool   `yaml:"enabled"`
		DaemonSocket   string `yaml:"daemon_socket"`
		ObserverSocket string `yaml:"observer_socket"`
		Group          string `yaml:"group"`
	} `yaml:"docker"`
}

type oldRules struct {
	Audit   []string `yaml:"audit"`
	Files   []string `yaml:"files"`
	Journal []string `yaml:"journal"`
	Docker  []string `yaml:"docker"`
}

type oldLimits struct {
	ToolTimeout      time.Duration `yaml:"tool_timeout"`
	Concurrent       int           `yaml:"max_concurrent_operations"`
	ConfigBytes      int           `yaml:"max_config_bytes"`
	LogEntries       int           `yaml:"max_log_entries"`
	DefaultLogWindow time.Duration `yaml:"default_log_window"`
	MaxLogWindow     time.Duration `yaml:"max_log_window"`
	ResponseBytes    int           `yaml:"max_response_bytes"`
	InspectionBytes  int           `yaml:"max_inspection_bytes"`
	RequestBytes     int           `yaml:"max_request_bytes"`
	PageSize         int           `yaml:"max_page_size"`
	IdleTimeout      time.Duration `yaml:"idle_timeout"`
	ExplainEntries   int           `yaml:"max_explain_entries"`
	ExplainTimeout   time.Duration `yaml:"explain_timeout"`
}

type oldThreshold struct {
	Warning  float64 `yaml:"warning"`
	Critical float64 `yaml:"critical"`
}
type oldHealth struct {
	Services           oldThreshold  `yaml:"failed_services"`
	Sample             time.Duration `yaml:"cpu_sample"`
	Required           []string      `yaml:"required"`
	ExcludeFilesystems []string      `yaml:"exclude_filesystems"`
	Usage              oldThreshold  `yaml:"usage"`
	Load               oldThreshold  `yaml:"load"`
}

type oldToken struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Roles   []string  `json:"roles"`
	Expires time.Time `json:"expires"`
	Status  string    `json:"status"`
	Hash    string    `json:"hash"`
}

var legacyProfileName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type oldProfile struct {
	Profiles []string `yaml:"profiles"`
	Allow    oldRules `yaml:"allow"`
	Deny     oldRules `yaml:"deny"`
}

func validLegacyPattern(category, value string) bool {
	if len(value) > 4096 {
		return false
	}
	switch category {
	case "audit":
		switch value {
		case "*", "processes", "network", "accounts", "storage", "updates", "security", "services", "hostlens", "paths":
			return true
		}
		return false
	case "files":
		return strings.HasPrefix(value, "/") && path.Clean(value) == value && doublestar.ValidatePattern(value)
	case "docker":
		if value == "*" {
			return true
		}
		switch value {
		case "daemon", "containers", "images", "volumes", "networks", "disk_usage":
			return true
		}
		kind, selector, ok := strings.Cut(value, "/")
		if !ok || selector == "" || strings.Contains(selector, "/") || strings.Contains(selector, "..") || strings.ContainsAny(selector, " =\n\r") {
			return false
		}
		switch kind {
		case "container", "stats", "logs", "image", "volume", "network":
			return doublestar.ValidatePattern(selector)
		}
		return false
	default:
		return value != "" && !strings.ContainsAny(value, "/ =\n\r") && doublestar.ValidatePattern(value)
	}
}

func validateLegacyRules(rules oldRules) error {
	for _, set := range []struct {
		category string
		values   []string
	}{{"audit", rules.Audit}, {"files", rules.Files}, {"journal", rules.Journal}, {"docker", rules.Docker}} {
		for _, value := range set.values {
			if !validLegacyPattern(set.category, value) {
				return fmt.Errorf("invalid legacy %s pattern %q", set.category, value)
			}
		}
	}
	return nil
}

func activeLegacyProfiles(names, directories []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	if len(directories) == 0 {
		directories = []string{"/etc/hostlens/profiles"}
	}
	seen := make(map[string]bool)
	visiting := make(map[string]bool)
	bytesRead := 0
	var visit func(string, int) error
	visit = func(name string, depth int) error {
		if !legacyProfileName.MatchString(name) || depth > 64 {
			return errors.New("invalid legacy profile name or traversal limit")
		}
		if visiting[name] {
			return errors.New("legacy profile include cycle")
		}
		if seen[name] {
			return nil
		}
		if len(seen) >= 128 {
			return errors.New("legacy profile traversal limit")
		}
		var data []byte
		var err error
		for _, directory := range directories {
			if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
				return errors.New("invalid legacy profile directory")
			}
			data, err = readProtected(filepath.Join(directory, name+".yaml"))
			if err == nil {
				break
			}
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if err != nil {
			return fmt.Errorf("legacy profile %q: %w", name, err)
		}
		bytesRead += len(data)
		if bytesRead > 8<<20 {
			return errors.New("legacy profiles exceed aggregate limit")
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		var profile oldProfile
		if err := decoder.Decode(&profile); err != nil {
			return fmt.Errorf("legacy profile %q: %w", name, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return errors.New("exactly one legacy profile document required")
		}
		if err := validateLegacyRules(profile.Allow); err != nil {
			return fmt.Errorf("legacy profile %q allow: %w", name, err)
		}
		if err := validateLegacyRules(profile.Deny); err != nil {
			return fmt.Errorf("legacy profile %q deny: %w", name, err)
		}
		visiting[name] = true
		for _, include := range profile.Profiles {
			if err := visit(include, depth+1); err != nil {
				return err
			}
		}
		delete(visiting, name)
		seen[name] = true
		return nil
	}
	for _, name := range names {
		if err := visit(name, 0); err != nil {
			return nil, err
		}
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func readProtected(path string) ([]byte, error) {
	file, err := core.OpenTrusted(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > maxInput || info.Size() < 0 {
		return nil, errors.New("unsafe migration source")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxInput+1))
	if err != nil || len(data) > maxInput {
		return nil, errors.New("migration source exceeds limit")
	}
	return data, nil
}

func convertTokens(data []byte) ([]auth.Token, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var old []oldToken
	if err := decoder.Decode(&old); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("trailing token data")
	}
	result := make([]auth.Token, 0, len(old))
	ids := make(map[string]struct{}, len(old))
	hashes := make(map[string]struct{}, len(old))
	for _, item := range old {
		if item.ID == "" || item.Name == "" || len(item.Hash) != sha256.Size*2 || (item.Status != "active" && item.Status != "revoked") {
			return nil, errors.New("invalid legacy token")
		}
		if _, err := hex.DecodeString(item.Hash); err != nil {
			return nil, errors.New("invalid legacy token hash")
		}
		if _, exists := ids[item.ID]; exists {
			return nil, errors.New("duplicate legacy token ID")
		}
		if _, exists := hashes[item.Hash]; exists {
			return nil, errors.New("duplicate legacy token hash")
		}
		ids[item.ID] = struct{}{}
		hashes[item.Hash] = struct{}{}
		observe := false
		for _, role := range item.Roles {
			switch role {
			case "health", "inspect", "diagnostics":
				observe = true
			case "metrics":
			default:
				return nil, errors.New("unknown legacy token role")
			}
		}
		record := auth.Token{ID: item.ID, Name: item.Name, Expires: item.Expires, Revoked: item.Status != "active" || !observe, Hash: item.Hash, Legacy: true}
		if observe {
			record.Roles = []string{"observe"}
		}
		result = append(result, record)
	}
	return result, nil
}

// Prepare creates a separate candidate. It does not alter the published
// installation or activate repairs, and it never converts old grants silently.
func Prepare(options Options) (Report, error) {
	if options.From == "" || options.Output == "" || !filepath.IsAbs(options.From) || !filepath.IsAbs(options.Output) {
		return Report{}, errors.New("absolute source and output paths required")
	}
	data, err := readProtected(options.From)
	if err != nil {
		return Report{}, err
	}
	var old oldConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&old); err != nil {
		return Report{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Report{}, errors.New("exactly one legacy YAML document required")
	}
	if old.Mode != "system" {
		return Report{}, errors.New("only system-mode installations can be migrated")
	}
	if old.Privilege != "" && old.Privilege != "standard" && old.Privilege != "restricted" {
		return Report{}, errors.New("invalid legacy privilege")
	}
	if old.Remediation.Enabled {
		return Report{}, errors.New("unsupported legacy remediation")
	}
	if err := validateLegacyRules(old.Allow); err != nil {
		return Report{}, fmt.Errorf("legacy configuration allow: %w", err)
	}
	if err := validateLegacyRules(old.Deny); err != nil {
		return Report{}, fmt.Errorf("legacy configuration deny: %w", err)
	}
	if len(old.Server.Bind) == 0 {
		old.Server.Bind = []string{"127.0.0.1"}
	}
	if old.Server.Port == 0 {
		old.Server.Port = 8080
	}
	explicitStore := old.TokenStore != ""
	if !explicitStore {
		old.TokenStore = "/etc/hostlens/secrets/tokens.json"
	}
	if old.Version != 1 || len(old.Server.Bind) != 1 || old.Server.Port < 1 || old.Server.Port > 65535 {
		return Report{}, errors.New("version 1 configuration with one listener required")
	}
	unmapped, err := activeLegacyProfiles(old.Profiles, old.ProfileDirs)
	if err != nil {
		return Report{}, err
	}
	if !filepath.IsAbs(old.TokenStore) {
		return Report{}, errors.New("absolute legacy token store required")
	}
	tokenData, err := readProtected(old.TokenStore)
	if errors.Is(err, os.ErrNotExist) && !explicitStore {
		tokenData = []byte("[]")
		err = nil
	}
	if err != nil {
		return Report{}, err
	}
	tokens, err := convertTokens(tokenData)
	if err != nil {
		return Report{}, err
	}
	converted, err := json.Marshal(tokens)
	if err != nil || len(converted) > 1<<20 {
		return Report{}, errors.New("converted token store exceeds version 2 limit")
	}
	config := core.Config{
		Version: 2, Listen: net.JoinHostPort(old.Server.Bind[0], fmt.Sprint(old.Server.Port)), ReadOnly: true,
		TokenStore: filepath.Join(options.Output, "tokens.json"), ProfileDir: filepath.Join(options.Output, "profiles"),
		ObserverSocket: "/run/hostlens-observer/observer.sock", RepairSocket: "/run/hostlens-repair/repair.sock",
		GatewayUID: options.GatewayUID, ObserverUID: options.ObserverUID, RepairUID: options.RepairUID, SharedGID: options.SharedGID, TokenGID: options.TokenGID,
	}
	if old.Docker.Enabled {
		config.DockerSocket = old.Docker.DaemonSocket
		if config.DockerSocket == "" {
			config.DockerSocket = "/var/run/docker.sock"
		}
	}
	if old.Server.TLS.Enabled {
		config.TLSCert, config.TLSKey = old.Server.TLS.CertFile, old.Server.TLS.KeyFile
	}
	if err := config.Validate(); err != nil {
		return Report{}, err
	}
	report := Report{SourceVersion: 1, TokensPreserved: len(tokens), ProfilesUnmapped: unmapped, Actions: []string{
		"Only the listener, TLS paths, local Docker socket, and token metadata are mapped; review all other version 1 settings.",
		"Review old profile grants and create new per-tool profiles before activation.",
		"Install the candidate under administrator-owned paths, update token_store and profile_dir to final locations, and set service identities and group access.",
		"Stop old services before switching binaries; retain old state and binaries for rollback.",
	}}
	if err := os.Mkdir(options.Output, 0700); err != nil {
		return Report{}, err
	}
	root, err := os.OpenRoot(options.Output)
	if err != nil {
		return Report{}, err
	}
	defer root.Close()
	if err := root.Mkdir("profiles", 0700); err != nil {
		return Report{}, err
	}
	for name, value := range map[string]any{"config.candidate.yaml": config, "tokens.json": tokens, "migration-report.json": report} {
		var content []byte
		if name == "tokens.json" {
			content = converted
		} else if name == "config.candidate.yaml" {
			content, err = yaml.Marshal(value)
		} else {
			content, err = json.MarshalIndent(value, "", "  ")
		}
		if err != nil {
			return Report{}, err
		}
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return Report{}, err
		}
		_, err = file.Write(content)
		if err == nil {
			err = file.Sync()
		}
		err = errors.Join(err, file.Close())
		if err != nil {
			return Report{}, err
		}
	}
	return report, nil
}
