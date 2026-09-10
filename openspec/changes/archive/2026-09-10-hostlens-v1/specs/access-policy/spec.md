# Access policy

## Purpose

Define the observable HostLens v1 access policy behavior, including security boundaries and failure outcomes for administrators and MCP clients.

## ADDED Requirements

### Requirement: Typed profile structure

Each YAML profile SHALL support profiles, allow, and deny. Allow and deny SHALL support files and journal categories in v1. Main configuration SHALL support direct typed rules and profile inclusions. Active unsupported categories such as future windows_events SHALL produce a clear validation error in v1.

#### Scenario: Complete application profile

- **WHEN** nginx includes web-common, allows configuration and logs, and denies private files
- **THEN** all included and direct typed rules contribute to the effective policy

#### Scenario: Future rule activated

- **WHEN** an active profile contains windows_events in Linux v1
- **THEN** configuration validation fails explicitly

### Requirement: Profile directories and activation

System installations SHALL use /etc/hostlens/profiles and user-run instances SHALL use ~/.config/hostlens/profiles as mode-specific defaults. Administrators SHALL be able to configure additional ordered local directories. Only referenced profiles SHALL activate; available allow-all profiles SHALL remain disabled by default. System services SHALL NOT implicitly load personal home profiles.

#### Scenario: Shipped broad profile

- **WHEN** allow-all exists but is not referenced
- **THEN** its permissions do not activate

#### Scenario: Duplicate name

- **WHEN** two configured directories contain the same profile name
- **THEN** validation fails with both locations; no implicit precedence applies

#### Scenario: Unsafe reference

- **WHEN** a profile inclusion contains a remote URL or path traversal
- **THEN** validation rejects it

### Requirement: Includes and global denial

HostLens SHALL resolve included profiles transitively, deduplicate evaluation patterns while preserving provenance, and combine all allows. Every active denial and mandatory internal exclusion SHALL override all allows regardless of inclusion order. Unmatched data SHALL be denied. Cycles, missing active profiles, and invalid patterns SHALL fail validation.

#### Scenario: Global exception

- **WHEN** a profile allows /etc/xyz/** and a direct rule denies /etc/xyz/password
- **THEN** the password file is denied

#### Scenario: Dependency denial

- **WHEN** one included profile denies a path another profile allows
- **THEN** the path is denied

#### Scenario: Cycle

- **WHEN** base includes nginx and nginx includes base
- **THEN** validation reports the dependency cycle without activating partial policy

### Requirement: File glob semantics

File patterns SHALL use absolute paths. A literal matches exactly; * and ? SHALL NOT cross directory separators; ** SHALL support recursive matching. A bare directory SHALL NOT imply descendant access. Matching SHALL use defined platform path semantics and reject malformed patterns.

#### Scenario: Direct children

- **WHEN** /etc/xyz/*.conf is allowed
- **THEN** /etc/xyz/app.conf matches but /etc/xyz/nested/app.conf does not

#### Scenario: Recursive files

- **WHEN** /etc/xyz/** is allowed with no matching deny
- **THEN** descendant files qualify for further access checks

### Requirement: Safe file enforcement

The backend SHALL enforce policy before returning bytes through any tool. Both requested and resolved paths SHALL satisfy policy; symlinks and concurrent path replacement SHALL NOT bypass denials. Special files SHALL be rejected by general file readers. Dedicated collectors SHALL use narrowly defined system sources. OS restrictions SHALL remain effective.

#### Scenario: Symlink escape

- **WHEN** an allowed path resolves to a denied target
- **THEN** access is rejected

#### Scenario: Replacement race

- **WHEN** a path changes between validation and opening
- **THEN** the backend fails safely or uses a verified handle without reading a denied target

#### Scenario: Device request

- **WHEN** allow-all is active and a client requests a raw device
- **THEN** the general file reader rejects it

### Requirement: Mandatory protected sources

HostLens SHALL protect its authentication store, TLS private keys, privileged administrative configuration, and equivalent credentials from diagnostic retrieval even under allow-all. The mandatory inventory SHALL include configured secret locations. Diagnostic profiles SHALL NOT be writable through MCP. Broad read capability SHALL NOT be described as filesystem isolation.

#### Scenario: Allow-all credentials

- **WHEN** a client requests the configured gateway token store
- **THEN** mandatory exclusion denies access

#### Scenario: Alternate read tool

- **WHEN** the same credential source is requested as a log
- **THEN** the same denial applies

### Requirement: Journal units and source boundaries

Journal rules SHALL match systemd unit names using documented unit globs. A query SHALL select an allowed unit and obey all matching denials. Journal and file rules SHALL be distinct; a file denial SHALL NOT imply an automatic journal denial.

#### Scenario: Denied unit

- **WHEN** a wildcard allows units but nginx.service is explicitly denied
- **THEN** journal access for nginx.service is rejected

#### Scenario: Unscoped journal

- **WHEN** a client requests all journal records without selecting an allowed unit
- **THEN** the request is rejected

### Requirement: Built-in observation sources

Basic observation collectors SHALL have documented narrowly scoped built-in source grants so OS, health, service, and package facts work without configuration-file profiles. Explicit active source denials and mandatory exclusions SHALL override those grants. Collector grants SHALL NOT authorize raw file retrieval through read_config or query_logs. Service status SHALL exclude embedded journal excerpts, environment secrets, and raw configuration bodies.

#### Scenario: Basic observations without profiles

- **WHEN** no file profile is active and the OS identity source is not explicitly denied
- **THEN** get_os_info can return selected facts while read_config cannot read the raw source

#### Scenario: Explicit collector source denial

- **WHEN** a direct deny excludes a collector source
- **THEN** that collector reports the policy restriction and required health coverage is marked incomplete where affected

#### Scenario: Service status log bypass

- **WHEN** an inspect-only token requests service status
- **THEN** status facts are returned without recent log bodies even if the native command normally embeds them

### Requirement: Trusted policy files

System-service configuration and active profile definitions SHALL be controlled by the local administrator. Startup and reload SHALL validate ownership and write permissions of files and relevant parent directories, rejecting paths writable by unauthorized identities. User-run instances SHALL use the invoking user's trust boundary. Diagnostics SHALL NOT change profile definitions or their inclusion graph.

#### Scenario: Writable custom directory

- **WHEN** an active system-service profile resides in a directory writable by an unrelated user
- **THEN** startup or reload rejects that source instead of trusting its allow rules

#### Scenario: Profile file ownership change

- **WHEN** an active policy file becomes writable by the diagnostic identity
- **THEN** reload fails and the previous validated policy remains active

### Requirement: Literal mandatory source protection

Configured credential, configuration, and profile source paths SHALL be treated as literal mandatory exclusions, including filenames containing glob metacharacters. Matching caches and fingerprints SHALL distinguish literal paths from glob rules. Known mandatory source objects SHALL remain protected through hard-link aliases.

#### Scenario: Literal glob characters

- **WHEN** a configured secret filename contains brackets, wildcards, or backslashes
- **THEN** both configuration and raw log readers reject that literal file and known hard-link aliases

### Requirement: Bounded include expansion

Policy compilation SHALL bound graph visits separately from expanded provenance rules. A candidate exceeding either 10,000 visits or 10,000 provenance rules SHALL fail validation while retaining every inclusion chain for accepted candidates.

#### Scenario: Shared empty graph

- **WHEN** an empty shared include graph exceeds the traversal ceiling without emitting rules
- **THEN** validation promptly rejects the candidate and leaves the active generation unchanged

### Requirement: Handle-bound built-in observations

Built-in file observations SHALL check requested paths and the resolved names of safely opened regular-file descriptors before reading bytes. Ordinary OS and sysfs symlinks SHALL resolve within the selected filesystem root. Magic links, special files, source escapes, and denied targets SHALL fail without unsafe fallback.

#### Scenario: Replaced observation source

- **WHEN** an OS identity source is concurrently replaced with a symlink to a denied source
- **THEN** OS and inventory tools return only allowed observations or explicit collection issues
