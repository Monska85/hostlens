# Design

## Context

The gateway coordinates configuration reload through backend `/prepare` and `/activate` calls. Its current decoder enforces a byte ceiling and one JSON document, but a decoded map can be empty or false without causing an error. Local JSON transports repeat similar decoder logic. The observer adopts a socket-activated descriptor through `net.FileListener`; it also reads configuration through a separate path from the backend. Docker error bodies currently cross the observer boundary.

## Goals / Non-Goals

**Goals:** Fail closed on ambiguous activation replies, share strict bounded decoding, release adopted descriptors, and reduce verified request-path work.

**Non-Goals:** Change MCP tool schemas, add host mutation authority, cache diagnostic evidence, or add platform support.

## Decisions

- Decode control acknowledgements into typed payloads and require the expected field to be true. The backend already emits these fields. Checking HTTP status alone leaves an activation ambiguity.
- Use one internal bounded JSON decoder for local IPC and CLI replies. Keep call-site limits and error reporting local so each boundary retains its contract.
- Close the inherited descriptor after `net.FileListener` duplicates it. Keep ownership of the returned listener with the server.
- Share a narrow Linux configuration reader between the backend and observer. Importing the collector package into the observer would enlarge that executable's authority surface.
- Report Docker and observer refusal status without copying upstream error bodies. A status and typed issue preserve failure classification without forwarding untrusted content.
- Treat a truncated container inventory as an incomplete reference basis. Resolve no selector from it and retain only directly observed resource fields; suppress derived counts and unused or reclaimable claims.
- Revalidate container policy with the current identity and all names after collecting detail, stats, or logs. Stable-ID selectors require this recheck too, because their aliases can change during collection.
- Prefer measured and behavior-preserving request-path reductions over broad caches. Credentials and observations remain live per call.

## Risks / Trade-offs

- A stricter gateway can reject an older backend's incomplete reply. The binaries ship and upgrade as one versioned artifact set; the current backend already returns positive fields.
- Shared decoding changes error text at some internal boundaries. Tests assert failure behavior and visible error classes rather than private wording.
- Redacted Docker refusal text gives operators less daemon detail through MCP. The status and issue code remain available; administrators can inspect Docker separately.
- Container detail, stats, and logs now require one additional bounded inventory observation after collection, including stable-ID calls. This costs a daemon GET but closes the alias-change authorization race.

## Migration Plan

Package gateway and backend together. On rollback, restore the previous coherent archive; no data migration is required.
