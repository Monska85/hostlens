package lifecycle

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Monska85/hostlens/internal/config"
)

func GatewayUnit() string {
	return `[Unit]
Description=HostLens MCP gateway
After=hostlens-diagnostics.service
[Service]
Type=simple
User=hostlens-gateway
Group=hostlens-gateway
ExecStart=/usr/local/bin/hostlens serve --system
Restart=on-failure
UMask=0027
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
CapabilityBoundingSet=
ReadWritePaths=/run/hostlens
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
[Install]
WantedBy=multi-user.target
`
}
func DiagnosticsUnit(c config.Config) string {
	cap := ""
	if c.Privilege == "standard" {
		cap = "CAP_DAC_READ_SEARCH"
	}
	inaccessible := []string{filepath.Dir(c.TokenStore)}
	if c.Server.TLS.KeyFile != "" {
		inaccessible = append(inaccessible, c.Server.TLS.KeyFile)
	}
	for i, p := range inaccessible {
		inaccessible[i] = `"` + strings.ReplaceAll(p, `"`, `\"`) + `"`
	}
	return fmt.Sprintf(`[Unit]
Description=HostLens diagnostic backend
[Service]
Type=simple
User=hostlens-diagnostics
Group=hostlens-gateway
ExecStart=/usr/local/bin/hostlens-diagnostics serve --system
Restart=on-failure
UMask=0007
RuntimeDirectory=hostlens
RuntimeDirectoryMode=0770
RuntimeDirectoryPreserve=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=read-only
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictSUIDSGID=no
LockPersonality=yes
RestrictRealtime=yes
RestrictNamespaces=yes
SystemCallArchitectures=native
SystemCallFilter=@system-service openat2
SystemCallFilter=~@mount @reboot @swap @raw-io
CapabilityBoundingSet=%s
AmbientCapabilities=%s
ReadWritePaths=/run/hostlens
InaccessiblePaths=%s
RestrictAddressFamilies=AF_UNIX
IPAddressDeny=any
[Install]
WantedBy=multi-user.target
`, cap, cap, strings.Join(inaccessible, " "))
}
