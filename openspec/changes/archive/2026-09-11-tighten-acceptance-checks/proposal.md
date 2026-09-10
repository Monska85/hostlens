## Why

The follow-up review found inconsistent image/checksum selection, redundant dispatch, weak smoke assertions, fixed readiness delays and duplicated defaults.

## What Changes

Pass the selected image, use standard checksum validation in existing containers, remove circular dispatch, assert structured observations, poll bounded readiness and minimize the installation example.

## Capabilities

No requirement change. The implementation corrects existing release-validation and configuration contracts.

## Impact

Packaging and acceptance tooling only. No dependency, commit or publication is required.
