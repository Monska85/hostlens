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
ProtectKernelLogs=yes
ProtectClock=yes
ProtectHostname=yes
PrivateIPC=yes
ProtectProc=invisible
ProcSubset=pid
RestrictSUIDSGID=yes
LockPersonality=yes
RestrictRealtime=yes
RestrictNamespaces=yes
SystemCallArchitectures=native
SystemCallFilter=@system-service openat2
SystemCallFilter=~@mount @reboot @swap @raw-io
MemoryDenyWriteExecute=yes
CapabilityBoundingSet=
ReadWritePaths=/run/hostlens
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
[Install]
WantedBy=multi-user.target
`
}

// ObserverSocketUnit exposes the observer IPC endpoint through socket
// activation. The socket stays listening independently of the observer
// service so a later Docker installation activates the next request without
// another reconciliation; the service remains gated by its Requisite.
// The socket belongs to the diagnostic backend identities, and the observer
// still rejects peers by UID, so an unrelated local process with group
// access is rejected before Docker access.
func ObserverSocketUnit(c config.Config) string {
	quoted := strings.ReplaceAll(c.Docker.ObserverSocket, `"`, `\"`)
	return fmt.Sprintf(`[Unit]
Description=HostLens Docker observer IPC socket
# Docker daemon restarts must never permanently disable activation: a
# request arriving while Docker is down fails its Requisite check, and the
# next request after Docker returns must activate the observer again.
StartLimitIntervalSec=0
[Socket]
ListenStream=%s
SocketUser=hostlens-diagnostics
SocketGroup=hostlens-gateway
SocketMode=0660
RemoveOnStop=yes
# Activation attempts while Docker is down must never trip the socket
# trigger limit and permanently disable the endpoint.
TriggerLimitIntervalSec=0
[Install]
WantedBy=sockets.target
`, quoted)
}

// ObserverUnit runs the isolated observer. Docker group authority is
// process-scoped only: SupplementaryGroups= grants access to the running
// service without persistent account membership. The dependency direction
// guarantees HostLens never starts, stops, or restarts Docker. The service
// inherits the activated IPC socket and needs no filesystem writes, and the
// sandbox matches the repository baseline for privileged processes.
func ObserverUnit(c config.Config) string {
	group := strings.ReplaceAll(c.Docker.Group, `"`, `\"`)
	return fmt.Sprintf(`[Unit]
Description=HostLens isolated Docker observer
Requisite=docker.service
After=docker.service
PartOf=docker.service
# Docker daemon restarts propagate stop/restart here. Keep the observer
# available across daemon upgrade cycles instead of tripping the default
# start-rate limit.
StartLimitIntervalSec=0
[Service]
Type=simple
User=hostlens-observer
Group=hostlens-observer
SupplementaryGroups=%s
ExecStart=/usr/local/bin/hostlens-docker-observer serve --system
Restart=on-failure
RestartSec=2
UMask=0027
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
ProtectKernelLogs=yes
ProtectClock=yes
ProtectHostname=yes
PrivateIPC=yes
ProtectProc=invisible
ProcSubset=pid
RestrictSUIDSGID=yes
LockPersonality=yes
RestrictRealtime=yes
RestrictNamespaces=yes
SystemCallArchitectures=native
SystemCallFilter=@system-service openat2
SystemCallFilter=~@mount @reboot @swap @raw-io
MemoryDenyWriteExecute=yes
CapabilityBoundingSet=
IPAddressDeny=any
RestrictAddressFamilies=AF_UNIX
[Install]
WantedBy=multi-user.target
`, group)
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
ProtectKernelLogs=yes
ProtectClock=yes
ProtectHostname=yes
PrivateIPC=yes
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
