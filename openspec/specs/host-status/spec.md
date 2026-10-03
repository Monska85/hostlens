# host-status Specification

## Purpose

Give authorized MCP clients a bounded, current view of host health and native service state without implying that unavailable measurements are healthy.

## Requirements

### Requirement: Current host status

HostLens SHALL expose typed host identity, CPU, memory, filesystem, and uptime observations on demand. Each measurement SHALL identify unavailable and failed collection distinctly, and results SHALL contain no invented substitutes or retained history.

#### Scenario: Supported host

- **WHEN** a permitted client requests host status on Linux amd64 or arm64
- **THEN** it receives the current bounded measurements and their observation time

#### Scenario: Partial failure

- **WHEN** one native source fails while others succeed
- **THEN** the result includes the successful measurements and identifies the failed source without reporting overall health as complete

### Requirement: Native service status

HostLens SHALL expose the current state of a named native service only when the client's role and active profile permit that service. It SHALL bound work and reject malformed or unapproved names before accessing the service manager.

#### Scenario: Permitted service

- **WHEN** a client requests an approved systemd unit
- **THEN** HostLens reports the observed unit identity, active state, and substate

#### Scenario: Service unavailable

- **WHEN** the service manager or unit is unavailable
- **THEN** HostLens reports that gap without claiming the unit is stopped

### Requirement: Platform-specific implementation

The public service fields SHALL use generic availability, state, and detail names. Linux validates systemd unit identifiers within its native adapter. Linux amd64 and arm64 SHALL be supported now; unsupported operating systems SHALL fail clearly instead of presenting a partial port as complete.

#### Scenario: Future platform adapter

- **WHEN** a macOS or Windows implementation supplies native status and service operations later
- **THEN** shared authorization and tool behavior need no Linux-specific branch
