# Reduction and QA decisions

## Context

Existing tests use a disposable Go container and a six-case platform/systemd matrix. Separate-process CLI/MCP acceptance already exists. The current launcher discards its temporary filesystem, so coverage needs an explicit artifact export path.

## Goals / Non-Goals

Remove code only when a caller trace establishes that behavior is preserved. Improve evidence for real failure paths rather than maximize test count or coverage percentage. Keep native platform adapters and existing privilege boundaries.

## Decisions

- Replace trivial command-forwarding scripts at their current callers; retain stable Make/Just recipe names and substantive scripts shared by workflows.
- Use Go's coverage tools and the existing container workflow. Avoid a new test framework, hosted coverage service, or runtime dependency.
- Start QA review with a measured baseline and the existing acceptance scenarios. Extend current fixtures and end-to-end tests where possible; consolidate tests only when distinct assertions survive.
- Export reports as developer artifacts outside release inputs. Preserve failure and cancellation behavior, including Docker engines that remap user IDs.

## Risks / Trade-offs

Statement coverage cannot prove every branch, native-kernel behavior, or separately launched uninstrumented executable. Document those limits and pair reports with acceptance evidence. Container-only validation remains mandatory; no VM is permitted for this work.

## Migration Plan

No operator migration is expected. Publish and verify the code-reduction checkpoint before resuming QA, as requested in the revised sequence. Amend main again after the QA cycle and final validation. Use an exact remote SHA lease and allow at most three further amendments for actual hosted CI failures.
