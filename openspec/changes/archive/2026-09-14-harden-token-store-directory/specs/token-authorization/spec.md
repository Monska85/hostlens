## ADDED Requirements

### Requirement: Protected token store location

The token store directory SHALL be verified on every read and every administrative change: it SHALL be a real directory (symlinks rejected), SHALL be owned by root or the effective user, and SHALL NOT grant write access to group or others. A store whose directory fails these preconditions SHALL be denied as an authentication failure, not accepted as an empty store. Existing token file checks (regular file opened with `O_NOFOLLOW`, owner root or the effective user, no group write and no other access, bounded size) SHALL remain unchanged.

#### Scenario: Swappable directory

- **WHEN** a local attacker with write access to the token store directory replaces the store file with a crafted credential set
- **THEN** the write access itself makes the directory fail the precondition and verification rejects the store before any role is honored

#### Scenario: Group-writable directory

- **WHEN** the token store directory grants group write access
- **THEN** bearer verification and administrative listing fail closed with a precondition error instead of reading its contents

#### Scenario: Directory owned by an untrusted user

- **WHEN** the token store directory is owned by neither root nor the effective user
- **THEN** reading the store is refused with the same precondition denial as permission failures

#### Scenario: Existing valid deployment unchanged

- **WHEN** the token store directory is owned by root or the effective user with no group or other write access
- **THEN** verification, administration, and rotation behave exactly as before