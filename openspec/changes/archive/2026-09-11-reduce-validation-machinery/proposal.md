## Why

The tab 4 review found a local CI scheduler and duplicate archive validation protecting workflow choices rather than product behavior. Remove those mechanisms and require a measured net reduction across implementation, tests, and configuration.

## What Changes

- **BREAKING developer workflow:** run local matrix cases sequentially against explicitly selected release archives; retain native CI parallelism and shared case selection.
- Execute fresh archive extraction through the runtime Go reader. Remove source fingerprints, expanded-tree comparison, reusable helper hashes, and the independent Python archive parser.
- Build acceptance helpers from the selected checkout into disposable storage. Remove helper compilation from distribution packaging.
- Keep GoReleaser OSS for packaging in one root configuration; upload tested archives with the standard GitHub CLI.
- Remove the empty shipped web-common profile dependency and generated dependencies.json. Preserve administrator-managed profiles and all license notices.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `release-validation`: explicit archive acceptance, serial local execution, and disposable test helpers replace infrastructure-specific candidate protocols.

## Impact

Packaging, test dispatch, CI, task runners, runtime archive-reader placement, profile installation defaults, tests, and documentation change. Runtime collection and privilege boundaries remain intact. No commit, push, release publication, or VM operation is authorized for this stage.
