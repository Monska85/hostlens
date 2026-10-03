# Validate HostLens

The repository's `Makefile` and lowercase `justfile` expose equivalent commands. Each calls its script or tool directly. Run `make help` or `just help` for the current list.

`make test` runs Go race tests, vet, both Linux builds, portable package boundary builds, and the MCP contract snapshot check in a disposable check container. `make package` builds the amd64 and arm64 release archives. `make verify-archives` checks checksums, expected members, and the release manifest. `make test-systemd` tests a fresh installation, repair, read-only activation, and a rejected configuration candidate with restoration, and bounded uninstall in a disposable systemd container. `make test-live-docker` uses a private nested Docker Engine, never the administrator's Docker socket. `make benchmark` includes a bounded `tools/list` request for comparison with the old gateway benchmark and a separate end-to-end tool call benchmark. Migration from release 0.5.0 to configuration version 2 is tested with protected source fixtures; an installed-host upgrade rehearsal remains an operator step.

A container shares its host kernel and may not provide all namespace or service behavior of a physical machine. Report unavailable nested-engine, systemd, or architecture checks as unavailable. Never install HostLens or modify this PC's systemd units for validation. A native arm64 CI runner is the release gate for arm64 runtime behavior.

## Rewrite measurements

The published 0.5.0 source tree contained 12,831 production Go lines under `cmd` and `internal`; the 0.6.0 candidate contains 3,191. Go test lines changed from 14,790 to 1,361, while Python and shell lines changed from 2,177 to 1,115. These are physical line counts, including comments and blanks, and do not measure runtime quality. The live Docker and systemd acceptance checks exercise paths outside Go unit coverage.

On the same disposable amd64 check image with a two-CPU limit, five bounded `tools/list` runs measured 213–226 µs, about 415 KB, and 796–797 allocations per request for the published 0.5.0 gateway; the final rewrite measured 198–203 µs, about 359 KB, and 605 allocations. The old benchmark used fake authentication; the new one reads a real token store on each request. The latency ranges overlap, so these measurements support a reduction in allocation cost, not a latency speedup claim. The separate end-to-end `host_status` tool call measured 374–388 µs and about 783 KB per call; there is no matching old benchmark for that path.
