# Proposal: version-coherent-shipped-docs

## Why

Shipped `OPERATIONS.md`, `INSTALL.md`, and `VALIDATION.md` carry hard-coded version literals (`0.1.0-dev`, and one stale `0.2.0` in INSTALL.md:187), so archive documentation names the wrong release forever. These docs ship inside every archive.

## What Changes

- `prepare.py` substitutes the known repository placeholder forms (`hostlens-0.1.0-dev-`, `hostlens-0.1.0-linux-`) with the candidate version in shipped Markdown copies; historical version facts (prose like "after the v0.1.0 release") are never rewritten.
- The repository's stale `hostlens-0.2.0-linux-amd64.tar.gz` literal in INSTALL.md is corrected to the repository placeholder form.
- A release test asserts shipped docs name the candidate archive version and leave historical prose intact.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `release-validation` — the versioned-delivery requirement now also requires shipped documentation to name the archive version it ships in, with a coherence scenario.

## Impact

- `tools/release/prepare.py`, `tools/release/test_release.py`, `docs/v1/INSTALL.md`.
- Tagged-release archives carry docs that reference the release's own archive names; development snapshots keep `0.1.0-dev` names.