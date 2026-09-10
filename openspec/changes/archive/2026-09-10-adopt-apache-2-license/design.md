# License packaging

## Context

See the proposal for the owner's license choice. Existing archives contain dependency license files and a generated placeholder NOTICE.txt.

## Goals / Non-Goals

Use one canonical project LICENSE and NOTICE for source and executable distribution. Publication, signing, and runtime changes remain outside this change.

## Decisions

Copy the official Apache-2.0 text without alteration. Identify the owner using the selected project identity, Monska85. Copy the root NOTICE into the existing archive NOTICE.txt location rather than maintaining two attribution texts. Preserve upstream NOTICE files alongside dependency licenses.

Read both project files before packaging either architecture so missing licensing material fails before archive creation. Include the resulting files in the existing checksum manifest.

## Risks / Trade-offs

Attribution and checksums do not prove publisher authenticity. Signed release provenance remains a separate future decision. Historical v1 planning records retain the license decision as it stood then; current specifications and operator documentation record this adoption.
