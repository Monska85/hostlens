## Context

The passing CI baseline builds archives, then runs four platform cases and two systemd cases sequentially. Both task runners dispatch shell scripts, whose language-specific helpers live under `tools/`. See the proposal for the requested scope.

## Goals / Non-Goals

Share case definitions and selection between CI and local execution. Preserve all current checks, build-once release artifacts, container boundaries, and explicit local build commands. Native arm64 coverage and server deployment are outside this change.

## Decisions

- **One manifest.** A JSON manifest records case identifiers, images, architectures, and lifecycle modes. A Python standard-library helper validates it, emits GitHub's `include` JSON, and dispatches local cases through existing shell entry points.
- **Bounded local workers.** A thread pool supervises independent case processes, with two workers by default and a configurable limit. Each case gets its own log and container name. Failures aggregate without cancelling other cases; interruption terminates active process groups and container traps clean up resources.
- **Explicit preparation.** Local matrix commands never download images, provision VMs, or rebuild. They check selected prerequisites before launching tests. A build-input fingerprint outside the release archives detects source changes; manifest checksums and candidate version checks retain archive integrity.
- **CI dependencies.** A checks job runs Go validation, while a build job emits the matrix and candidate bundle. Matrix jobs consume that bundle and prepare only their case's image or emulator. A final gate requires checks, build, and every matrix case to succeed, including failures or skipped dependencies.
- **Native engines.** Query Docker for its Linux architecture and resolve `native` to Debian on that architecture. Build Linux helpers for both architectures. Native arm64 requires no QEMU; explicit amd64 cases on an arm64 engine require that engine's existing amd64 emulation. Systemd cases use the native engine architecture. Keep unit-file rendering portable so a macOS build host can produce Linux archives.
- **Independent runners.** `just test-matrix <target>` and `make test-matrix TARGET=<target>` call the shared shell wrapper directly. Empty selection runs every case; a list command exposes valid targets.

## Risks / Trade-offs

- **Resource contention.** Parallel containers share the host kernel and resources. Limit concurrency and preserve per-container resource caps.
- **Artifact preparation cost.** Developers must rebuild after relevant input changes. This prevents rapid checks from silently using old code.
- **Hosted overhead.** Separate jobs download the candidate bundle and prepare images independently. This costs runner minutes but isolates failures and permits parallel execution.
- **Cancellation.** Docker clients alone do not own container lifetimes. Named containers and shell cleanup traps make interrupted cases removable without touching unrelated containers.
