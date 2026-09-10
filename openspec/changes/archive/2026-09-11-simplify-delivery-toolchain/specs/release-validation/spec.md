## ADDED Requirements

### Requirement: Separate compilation and distribution

Developer build commands SHALL compile the project binaries using the Go toolchain without requiring Python, Docker, or release-publishing tools. A distinct packaging command SHALL produce the installable archives through the same implementation used in CI. Packaging SHALL use GoReleaser OSS for archive creation and a standard SHA-256 checksum list. It SHALL preserve the existing upgrade manifest, licenses, and permissions. The checksum list SHALL cover both architecture archives and support standard checksum verification.

#### Scenario: Compile for development

- **WHEN** a contributor builds the binaries with Go and prepared module dependencies
- **THEN** compilation does not run archive creation, container tests, or release publication

#### Scenario: Create a distributable candidate

- **WHEN** a maintainer runs the packaging command
- **THEN** its archives can enter the shared validation matrix and retain the existing installation and upgrade contract

### Requirement: Container-owned validation toolchain

The prepared test image SHALL supply its Linux Go toolchain, race-test prerequisites, Python runtime, and dependency vulnerability scanner. Test execution SHALL NOT require mounting a separate host Linux Go installation or host-native scanner executable. Image preparation SHALL remain explicit, and tests SHALL preserve resource bounds, offline runtime-module access, and disposable filesystem isolation.

#### Scenario: Non-Linux development client

- **WHEN** a contributor has a prepared test image and source/module mounts available to a Linux Docker engine
- **THEN** container validation uses executable tools from the image rather than binaries built for the client operating system
