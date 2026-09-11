# Production audit review

Scope: the production-audit implementation changes, including new files, reviewed in place under the user's implementation authorization. No PR or release is part of this review.

## Round 1

The sf-peer-review panel covered security, specification, adversarial behavior, idiomatic Go, performance and maintainability. All unique findings were verified and fixed; duplicate reports are grouped below.

| Severity | Finding                                                                | Resolution                                                                     |
| -------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| High     | Service aliases could bypass a denied canonical identity               | Validate and authorize systemd's returned `Id` before exposing properties      |
| Medium   | Denied dependency names could escape through service properties        | Filter dependency identities through journal policy                            |
| Medium   | Root path handling failed safe open or exact authorization             | Open `.` relative to the root handle and normalize the resolved path           |
| Medium   | OS permission errors were reported as policy denial or missing sources | Classify typed OS errors before policy fallbacks                               |
| Medium   | Service discovery ignored runtime availability                         | Require both audit authority and a supported systemd runtime                   |
| Medium   | Valid device and swap dependencies were discarded                      | Recognize native unit suffixes                                                 |
| Medium   | Failed oversized reads escaped the aggregate inspection budget         | Charge bytes read, including oversized observations, and stop subsequent reads |
| Medium   | Process pagination reread the entire inventory                         | Select the enumerated PID page before reading individual processes             |
| Medium   | Oversized page requests collected evidence before rejection            | Enforce the configured ceiling at audit dispatch                               |

Regression tests cover these fixes. The container race suite, vet and portable builds passed after correction of the test fixture and root-open implementation.

## Round 2

The panel reviewed the revised runtime, application acceptance and shared CI matrix. Five medium findings were verified and fixed:

- **Process error categories:** Preserve policy, OS permission, absence, malformed-data and budget causes. An unavailable final identity check marks retained evidence unverified; an observed identity mismatch rejects it.
- **Descriptor byte budget:** Charge link bytes and stop descriptor inspection when the shared budget expires.
- **Root mount dependencies:** Preserve the valid `-.mount` identity while honoring explicit denial.
- **Fixture diagnostics:** Print bounded initialization and startup log tails on failure, preserving exit status.
- **Evidence assertions:** Require a root account, root mount, visible kernel control and effective HostLens settings instead of accepting empty successful responses.

The updated race suite, static checks and all ten container matrix cases passed, including the stronger assertions against PostgreSQL, nginx, Apache and MySQL.

## Round 3

The panel reviewed other authors' changes with security, adversarial, performance, maintainability, idiomatic, specification, documentation and CI coverage. All three independent review passes returned no further findings. The review stopped after three rounds, within the five-round limit.

## Final local evidence

- **Container checks:** Go race tests, vet, Linux builds, portable macOS/Windows compilation and Python regressions passed. GoReleaser configuration and both installable archives passed validation.
- **Matrix:** All ten cases passed: three native amd64 distribution cases, emulated arm64, both systemd privilege modes, and four real application fixtures. The first full attempt used the wrong argv convention for the extracted emulator; the documented configuration passed without changing product code.

## Contract verification and delivery gate

All three specification deltas are implemented and synchronized. Policy and selectors map to shared contract/gateway enforcement; Linux audit collectors implement bounded native evidence; unit regressions and container application acceptance cover the stated scenarios. No implementation task or review finding remains open.

Commit, push and hosted CI verification follow this local record. A successful local run does not assert hosted CI success or a published release. Remaining product limits are documented in the operator guide: no active firewall collection, SQL/internal application conclusions, pending-upgrade evaluation or extra OS authority.
