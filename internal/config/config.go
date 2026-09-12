package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/netip"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
	"go.yaml.in/yaml/v3"
)

type Rules struct {
	Audit   []string `yaml:"audit"`
	Files   []string `yaml:"files"`
	Journal []string `yaml:"journal"`
}
type Profile struct {
	Profiles []string `yaml:"profiles"`
	Allow    Rules    `yaml:"allow"`
	Deny     Rules    `yaml:"deny"`
}
type TLS struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}
type Server struct {
	Bind              []string `yaml:"bind"`
	Port              int      `yaml:"port"`
	TLS               TLS      `yaml:"tls"`
	AllowInsecureHTTP bool     `yaml:"allow_insecure_http"`
	TrustedProxies    []string `yaml:"trusted_proxies"`
	ClientIPHeader    string   `yaml:"client_ip_header"`
	AllowedOrigins    []string `yaml:"allowed_origins"`
}
type Limits struct {
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
type Threshold struct {
	Warning  float64 `yaml:"warning"`
	Critical float64 `yaml:"critical"`
}
type Health struct {
	Services           Threshold     `yaml:"failed_services"`
	Sample             time.Duration `yaml:"cpu_sample"`
	Required           []string      `yaml:"required"`
	ExcludeFilesystems []string      `yaml:"exclude_filesystems"`
	Usage              Threshold     `yaml:"usage"`
	Load               Threshold     `yaml:"load"`
}
type Logging struct {
	Level                string `yaml:"level"`
	AuditSuccessfulCalls bool   `yaml:"audit_successful_calls"`
}
type Metrics struct {
	Enabled        bool `yaml:"enabled"`
	AllowAnonymous bool `yaml:"allow_anonymous"`
}
type Config struct {
	Metrics         Metrics `yaml:"metrics"`
	Profile         `yaml:",inline"`
	Version         int      `yaml:"version"`
	Mode            string   `yaml:"mode"`
	Privilege       string   `yaml:"privilege"`
	Server          Server   `yaml:"server"`
	ProfileDirs     []string `yaml:"profile_dirs"`
	TokenStore      string   `yaml:"token_store"`
	Socket          string   `yaml:"socket"`
	AdminSocket     string   `yaml:"admin_socket"`
	GatewayUser     string   `yaml:"gateway_user"`
	DiagnosticsUser string   `yaml:"diagnostics_user"`
	Limits          Limits   `yaml:"limits"`
	Health          Health   `yaml:"health"`
	Logging         Logging  `yaml:"logging"`
	Remediation     struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"remediation"`
}

func Decode(data []byte, out any) error {
	if len(data) > 1048576 {
		return errors.New("configuration exceeds 1 MiB")
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if e := d.Decode(out); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return errors.New("exactly one YAML document required")
	}
	return nil
}
func validate(c Config) error {
	if c.Version != 1 {
		return errors.New("unsupported configuration version")
	}
	if c.Remediation.Enabled {
		return errors.New("remediation is unavailable in v1")
	}
	if c.Mode != "system" && c.Mode != "user" {
		return errors.New("mode must be system or user")
	}
	if c.Privilege != "standard" && c.Privilege != "restricted" {
		return errors.New("privilege must be standard or restricted")
	}
	if c.Mode == "user" && c.Privilege != "restricted" {
		return errors.New("user mode requires restricted privilege")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 || len(c.Server.Bind) == 0 {
		return errors.New("invalid server port or empty bind")
	}
	var ips []netip.Addr
	for _, s := range c.Server.Bind {
		a, e := netip.ParseAddr(s)
		if e != nil || a.Zone() != "" || a.Is4In6() {
			return fmt.Errorf("invalid bind IP %q", s)
		}
		if !a.IsLoopback() && !c.Server.TLS.Enabled && !c.Server.AllowInsecureHTTP {
			return errors.New("non-loopback plaintext requires allow_insecure_http")
		}
		for _, b := range ips {
			if a.Is4() == b.Is4() && (a == b || a.IsUnspecified() || b.IsUnspecified()) {
				return errors.New("duplicate or overlapping binds")
			}
		}
		ips = append(ips, a)
	}
	if c.Server.TLS.Enabled && (c.Server.TLS.CertFile == "" || c.Server.TLS.KeyFile == "") {
		return errors.New("TLS requires cert_file and key_file")
	}
	if c.Server.ClientIPHeader != "X-Forwarded-For" {
		return errors.New("only X-Forwarded-For is supported")
	}
	for _, s := range c.Server.TrustedProxies {
		if _, e := netip.ParsePrefix(s); e != nil {
			if _, e = netip.ParseAddr(s); e != nil {
				return fmt.Errorf("invalid trusted proxy %q", s)
			}
		}
	}
	l := c.Limits
	if l.ToolTimeout <= 0 || l.ToolTimeout > contract.MaxToolTimeout || l.Concurrent < 1 || l.Concurrent > 128 || l.ConfigBytes < 1 || l.LogEntries < 1 || l.DefaultLogWindow <= 0 || l.MaxLogWindow < l.DefaultLogWindow || l.MaxLogWindow > 365*24*time.Hour || l.ResponseBytes < 1024 || l.ResponseBytes > 16<<20 || l.InspectionBytes < 1 || l.InspectionBytes > 64<<20 || l.RequestBytes < 1024 || l.RequestBytes > 1<<20 || l.PageSize < 1 || l.PageSize > 10000 || l.IdleTimeout <= 0 || l.IdleTimeout > 5*time.Minute || l.ExplainEntries < 1 || l.ExplainTimeout <= 0 || l.ExplainTimeout > time.Minute {
		return errors.New("invalid operation limits")
	}
	if l.ConfigBytes > l.InspectionBytes || l.LogEntries > 10000 || l.ExplainEntries > 100000 {
		return errors.New("limits exceed safety ceilings")
	}
	if c.Health.Sample <= 0 || c.Health.Sample >= l.ToolTimeout {
		return errors.New("CPU sample must fit tool timeout")
	}
	for _, t := range []Threshold{c.Health.Usage, c.Health.Load, c.Health.Services} {
		if math.IsNaN(t.Warning) || math.IsNaN(t.Critical) || math.IsInf(t.Warning, 0) || math.IsInf(t.Critical, 0) || t.Warning < 0 || t.Critical <= t.Warning {
			return errors.New("threshold requires 0 <= warning < critical")
		}
	}
	if c.Health.Usage.Critical > 100 {
		return errors.New("usage threshold exceeds 100 percent")
	}
	for _, s := range c.Health.Required {
		switch s {
		case "memory", "swap", "filesystem", "services", "load", "cpu":
		default:
			return fmt.Errorf("unknown required check %q", s)
		}
	}
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("invalid logging level")
	}
	return nil
}

func defaults() Config {
	c := Config{Metrics: Metrics{Enabled: true}, Version: 1, Mode: "user", Privilege: "restricted", Server: Server{Bind: []string{"127.0.0.1"}, Port: 8080, ClientIPHeader: "X-Forwarded-For"}, Limits: Limits{10 * time.Second, 4, 65536, 200, 15 * time.Minute, 24 * time.Hour, 131072, 1048576, 65536, 200, 30 * time.Second, 1000, 2 * time.Second}, Health: Health{Threshold{1, 2}, time.Second, []string{"memory", "swap", "filesystem", "services", "load", "cpu"}, nil, Threshold{80, 95}, Threshold{1, 2}}, Logging: Logging{"info", true}}
	return c
}
