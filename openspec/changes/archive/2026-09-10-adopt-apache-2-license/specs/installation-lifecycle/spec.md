## ADDED Requirements

### Requirement: Project license and attribution in archives

HostLens SHALL use Apache-2.0 for project-owned code and documentation. Each executable archive SHALL include the full project license and a readable attribution notice identifying HostLens, Monska85, and the original repository. Dependency license and attribution notices SHALL retain their applicable terms. Archive checksums SHALL cover these files. Packaging SHALL fail if the project license or notice is missing.

#### Scenario: Redistributable archive

- **WHEN** an amd64 or arm64 archive is built
- **THEN** it includes the Apache-2.0 license, HostLens attribution, and dependency notices with valid manifest checksums

#### Scenario: Missing licensing material

- **WHEN** the project license or attribution notice is absent
- **THEN** packaging fails instead of producing an archive without that material
