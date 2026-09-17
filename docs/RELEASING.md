# Development, validation, and releases

Make and Just expose equivalent commands and invoke each tool or script directly. Tests run in disposable Docker containers; checks never install missing prerequisites or modify the administrator host's services.

## Toolchain

Use the Go version in `go.mod`, Python 3.11 or newer with virtual-environment support, ShellCheck, Docker, and either GNU Make or Just. Packaging also requires [GoReleaser OSS](https://goreleaser.com/install/) at the version pinned in `.github/workflows/ci.yml`. `make deps` or `just deps` prepares the runtime modules and locked development tools locally. Setup never installs global tools.

- **Go:** Runtime, protocol, collectors, and native acceptance helpers.
- **Python:** Standard-library release metadata and a serial matrix dispatcher.
- **POSIX shell:** Container entry points and command dispatch.

`make fmt` and `make fmt-check` cover Go, Python, and POSIX shell. `make lint` checks Python, shell, and workflows; `make check` adds container tests. The Just commands have the same names. `test` includes Go race tests and vet plus Python utility regressions, all in containers.

Agents format edited Markdown through their harness skill. Agents also validate project OpenSpec artifacts with their environment CLI. Neither tool is a project dependency or CI requirement.

`HOSTLENS_MOD_CACHE` selects the runtime module cache. Set it before setup, compilation, packaging, or tests to use the same cache consistently. `make build` compiles Linux binaries to `bin/` using Go alone. `make package` writes distribution archives to `dist/archives/` with temporary packaging trees under `dist/`; generated output is ignored by Git.

## Local validation

Prepare dependencies and the test image explicitly:

```sh
make deps
make test-image
make check
make package
make verify-archives
```

The Go test container has no network, a read-only root and source/module mounts, temporary writable storage, dropped capabilities, and bounded memory/processes. `HOSTLENS_TEST_IMAGE` overrides its image. `make test-image` builds its Linux Go toolchain, Python, race-test compiler, and vulnerability scanner from pinned inputs. No host Go installation or scanner executable is mounted. Matrix invocation compiles temporary acceptance helpers with the host Go toolchain and prepared module cache. Containers execute those helpers and the selected archives; they do not mount Go.

Prepare the representative environments before full matrix acceptance:

```sh
docker pull debian:stable-slim
docker pull ubuntu:24.04
docker pull archlinux:base
make systemd-image
make test-targets
docker pull --platform linux/amd64 postgres:17-bookworm
docker pull --platform linux/amd64 nginx:stable-bookworm
docker pull --platform linux/amd64 httpd:2.4-bookworm
docker pull --platform linux/amd64 mysql:8.4
make test-matrix TARGET=native
make test-matrix
```

The local full matrix requires arm64 emulation on an amd64 engine. Set `HOSTLENS_QEMU_AARCH64` to a static interpreter with `openat2` support. Ordinary system QEMU uses `HOSTLENS_QEMU_PRESERVE_ARGV0=0`. This tested alternative copies an interpreter without registering host binfmt handlers:

```sh
image=tonistiigi/binfmt:qemu-v10.2.3@sha256:400a4873b838d1b89194d982c45e5fb3cda4593fbfd7e08a02e76b03b21166f0
docker pull --platform linux/amd64 "$image"
container=$(docker create --platform linux/amd64 "$image")
trap 'docker rm "$container" >/dev/null' EXIT
HOSTLENS_QEMU_AARCH64=$(mktemp /tmp/hostlens-qemu-aarch64.XXXXXX)
docker cp "$container:/usr/bin/qemu-aarch64" "$HOSTLENS_QEMU_AARCH64"
chmod 755 "$HOSTLENS_QEMU_AARCH64"
export HOSTLENS_QEMU_AARCH64
export HOSTLENS_QEMU_PRESERVE_ARGV0=1
```

`packaging/tests/matrix.json` defines the shared platform and lifecycle cases. `native` selects Debian for the connected Linux engine's architecture, not the client machine. Native arm64 uses arm64 images and freshly built helpers, with no QEMU. Explicit amd64 cases on an arm64 engine require working amd64 emulation; Arch's case remains amd64.

An omitted target runs every case serially. Console output shows the archive directory, version, image, architecture, named stages, and outcome. Any failed case fails the command after the remaining cases finish. Cancellation stops the active case and its container. CI schedules cases concurrently and retains their ordinary job logs.

Each case verifies and extracts its archive into fresh disposable storage. It checks archive safety, the complete compressed stream, member hashes, executable modes, required notices, and the expected version. Coreutils verifies the checksum list; a focused case checks only its selected archive. Expanded packaging directories are not test inputs.

`HOSTLENS_VERSION` must agree between packaging and validation; unset it for the development version. To inspect an existing candidate, set `HOSTLENS_ARCHIVES` to its directory or pass `--archives` directly to the dispatcher. Acceptance tests those selected bytes; it does not assert that they match current source. Run `make package` first when validating source changes.

Systemd acceptance uses a disposable native-architecture container with private writable cgroups and declared extra capabilities. It tests restricted and standard privilege separately, including install, upgrade, removal, and secret containment. It has no host filesystem mounts or external network. The shared kernel and namespace limits remain material; no VM fallback is automatic.

### Application audit acceptance

The shared matrix includes `audit-postgres`, `audit-nginx`, `audit-apache` and `audit-mysql`. Each uses a maintained official amd64 application image with its non-root application identity, no network, a read-only root filesystem, dropped capabilities, resource limits and temporary writable state. The fixture starts the real server on isolated loopback and audits its process, listener, configuration, log and file metadata through generic MCP calls. Explicit source-denial controls must also pass.

Run one with `make test-matrix TARGET=audit-postgres` or its Just equivalent after preparing images and archives. Application-specific setup exists only in acceptance fixtures; runtime collectors have no application integrations. Database fixtures use disposable unauthenticated local initialization solely inside their network-isolated containers. No host ports, persistent database volumes or database credentials are created.

## Coverage

Run `make coverage` or `just coverage` with the same image and module-cache prerequisites as `test`. The command runs container checks once and writes `coverage/index.html`, `coverage/coverage.out`, and `coverage/functions.txt`. Open the HTML report to inspect uncovered statements; the console prints the aggregate total.

Go counters cover `internal/...` across unit and integration tests. The separate-process CLI fixture builds instrumented executables; native `covdata` merges their counters with test counters. Per-package test output is not the aggregate report. Python utility tests and the separately built platform/systemd matrix run in CI but are outside this Go statement percentage. Coverage does not measure every branch or prove native macOS/Windows behavior.

A failed run returns failure and removes previous managed reports. Cancellation removes its disposable container and temporary reports. CI retains successful reports as the `go-coverage` artifact for seven days; no hosted coverage service or percentage gate is required.

## Hosted CI

CI runs on default-branch pushes, pull requests, and manual dispatch. The release workflow reuses it for tags. The jobs are:

1. **Checks:** Locked tool setup, formatting, lint, vulnerability scan, and container regression tests with coverage reports.
2. **Build:** One versioned amd64/arm64 candidate with checked transport checksums.
3. **Matrix:** Independent platform and lifecycle jobs verifying, extracting, and executing that exact candidate. Debian arm64 runs natively on `ubuntu-24.04-arm`, and both systemd acceptance cases also run natively on arm64; amd64 platform and systemd cases use `ubuntu-24.04`. Platform preparation verifies the engine architecture and pulls the matching image; hosted cases do not use QEMU.
4. **Gate:** Fails if checks, build, or any required matrix case fails or is skipped.

Use `CI gate` as the aggregate required check. Workflows use read-only permissions except release delivery, and checkout does not retain credentials. External actions and critical tool images use commit or digest pins; distribution tags intentionally track representative releases. Update pins deliberately and rerun acceptance. Dependabot proposes weekly PRs for `github-actions` and `tools/dev` Go modules; SHA-pinned actions arrive as version-bump PRs, and every pin update is reviewed and accepted deliberately rather than merged automatically.

GitHub provides [standard arm64 runners for public and private repositories](https://docs.github.com/en/actions/reference/runners/github-hosted-runners). The [Ubuntu arm64 image](https://github.com/actions/partner-runner-images/blob/main/images/arm-ubuntu-24-image.md) includes Docker and Python. Native hosted evidence requires a successful run of this configuration; local emulation remains separate evidence.

A fresh `govulncheck` scan fails on reachable vulnerabilities or scanner errors. Use `make scan-vulnerabilities` locally with network access; the scan executes in a disposable container. Results apply to the vulnerability database at scan time.

### Optional caches and cancellation

Hosted image preparation uses Buildx layer caches separated by image kind, architecture and trust. Container compilation uses a separate cache keyed by validation inputs and actual image identity. Go tests always execute with `-count=1`; selected archives and freshly built acceptance helpers are never restored from compiler caches. PR cache writes cannot populate default-branch or release restore refs.

Manual CI dispatch and reusable callers can set `disable_cache: true` to disable hosted layer, compiler and setup-go caches. Cache misses use normal preparation, and optional layer-export failures do not hide build or test failures. Local `test-image` and `systemd-image` commands require no hosted cache credentials.

Local compiler reuse is optional:

```sh
mkdir -m 700 /tmp/hostlens-build-cache
HOSTLENS_BUILD_CACHE=/tmp/hostlens-build-cache make coverage
HOSTLENS_BUILD_CACHE=/tmp/hostlens-build-cache make scan-vulnerabilities
```

The parent must already exist, belong to the caller and have mode 0700. Missing or unsafe parents fall back to disposable compilation. Only a dedicated image-scoped leaf is writable in the container; source and module mounts remain read-only. Unset the variable for disposable compilation. Remove the dedicated cache after all users of it finish to reclaim space; it contains no credentials or test verdicts.

New ordinary runs supersede older runs for the same workflow, event and branch/PR. Other branches and release validation use separate groups. Cancellation preserves failure/cancellation status and cleans up the active acceptance container. The matrix remains capped at three concurrent jobs; see the [CI execution evidence](../openspec/changes/archive/2026-09-12-optimize-ci-execution/validation.md) for measured results.

## Packaging responsibilities

`make package` and `just package` run the same GoReleaser OSS snapshot command. Snapshot mode creates local artifacts without publishing, including when CI supplies a release-tag version through `HOSTLENS_VERSION`.

- **GoReleaser:** `.goreleaser.yaml` runs standard Go build commands, creates amd64/arm64 tar.gz archives, and writes one standard `checksums.txt`.
- **Tool contract snapshot:** `docs/v1/tools.json` is generated from a diagnostics-role `tools/list` call by `HOSTLENS_UPDATE_SNAPSHOT=1 go test ./internal/gateway/ -run TestToolsSnapshot -count=1` (run after changing any tool's contract, and commit the regenerated file). `tools/release/prepare.py` copies it unchanged into every architecture staging tree and fails when the copies diverge; each archive therefore ships `tools.json`, and the release workflow additionally uploads it as a standalone checksummed asset next to `checksums.txt`.
- **Preparation adapter:** `tools/release/prepare.py` clears declared staging paths, collects dependency license notices, and generates the per-member upgrade manifest. It does not create archives or checksum files.
- **Validation:** `tools/archive` shares the runtime upgrade reader and adds packaging assertions. Matrix and container helpers exercise HostLens behavior and privilege boundaries.

Archives contain both executables, a minimal installation configuration, profiles, operator guidance, the tool contract snapshot, and license notices. The example inherits runtime defaults and specifies only its version, system mode, and standard privilege. Go module manifests and pre-rendered service units are omitted: these are executable distributions, and installation generates units from the selected configuration. Go embeds module build information in each executable; dependency license notices remain in the archive.

Packaging uses fresh declared staging trees. After a failed rebuild, fix the failure and rerun `package` before testing that source. Plain `go build` does not need Python, Docker, or GoReleaser. Native OS packages and product container images are not required for the current systemd installation contract.

## Release procedure

Release publication requires owner authorization. Before tagging, review [validation limits](v1/VALIDATION.md) and configure default-branch and version-tag protections appropriate to the project.

1. Select a reviewed commit in the default branch history and an unused `vMAJOR.MINOR.PATCH` tag, optionally with a SemVer prerelease suffix. Build metadata suffixes are unsupported.
2. Create and push the tag through the authorized Git workflow. The release workflow validates tag syntax and ancestry, then runs the full CI suite.
3. The delivery job downloads the tested archives from that run. The standard `gh` CLI attaches the two archives and checksums to a draft after checking their hashes, without rebuilding, and records GitHub build provenance attestations (`actions/attest-build-provenance`) for the archives and the checksum manifest. Attestations require a public (or org-owned paid) repository; while the repository is private the publish job reports their unavailability explicitly and delivery stays checksum-verified. Once public, repository readers verify with `gh attestation verify <asset> -R Monska85/hostlens`.
4. Inspect the draft, test results, and release notes before publishing. A failed upload may leave a partial draft; never silently replace published assets. The publish job refuses to run when the release already exists instead of creating a duplicate draft.

`make release-check` validates the GoReleaser packaging configuration and requires the pinned GoReleaser version. Only delivery receives `contents: write` through `GITHUB_TOKEN`; no host rollout credentials are needed.

Archives include the upgrade manifest, project license, attribution, and dependency notices. Checksums detect corruption, not independently authenticated publisher identity; the release workflow records build provenance attestations to authenticate the build's origin. Draft upload, signing, and provenance are distinct from local archive verification; [validation evidence](v1/VALIDATION.md) records what has actually run.

Operators perform explicit [upgrades](v1/OPERATIONS.md#audit-upgrade-and-removal). Publishing a release never changes a server.
