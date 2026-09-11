## Context

The checks job builds a validation image on each hosted runner. Both lifecycle jobs independently build the systemd image. Test and scanner containers mount the module cache, but put their compiler cache in disposable tmpfs. The matrix caps parallelism at three; ordinary CI has no supersession policy. See proposal.md for scope.

## Goals / Non-Goals

Reduce repeated preparation and compilation using standard GitHub, Docker and Go facilities. Preserve offline test execution, resource limits, container-owned tools and the shared local/CI case definition.

Do not remove tests, cache test verdicts, replace GoReleaser, change archive contents or introduce a scheduler, image registry requirement or new runtime dependency. Keep freshly built helpers in disposable storage; compiler-cache reuse does not mean reusing helper executables from another checkout.

## Decisions

- **Measure first.** Capture at least three comparable recent successful runs, including commit, runner labels, job/step durations, queue delays and summed job execution minutes. Record the current matrix and checks. Compare the implementation on the same candidate with caches disabled/empty and warm. Use three samples per condition where available; state sample counts and environmental differences. Do not equate runner minutes with billed cost or sum overlapping jobs into elapsed time.
- **Use native supersession.** Add GitHub concurrency groups scoped to the workflow and branch or PR identity. Cancel obsolete ordinary runs only. Isolate release invocation groups and disable supersession for tag/release validation, including the reusable-workflow caller. Do not let parent and called workflows share a cancelling group. Keep the aggregate gate's failure and cancellation semantics.
- **Cache image layers.** Use official Buildx support and the GitHub cache backend for explicit CI image preparation, with separate scopes for validation and systemd images. Load images into the local engine for the existing tests. Keep local preparation commands functional without GitHub credentials. Prefer layer reuse over introducing a separate image publishing pipeline.
- **Cache compilation, not outcomes.** Restore a dedicated container compiler-cache directory through standard GitHub caching and mount only that directory writable. Scope compatibility by Linux architecture, Go toolchain and validation-toolchain inputs. Go retains responsibility for source/build-flag invalidation; preserve forced test execution. Keep the runtime module mount and source mount read-only, and keep credentials and reports out of caches.
- **Preserve cache trust.** Follow GitHub ref/event cache isolation; untrusted PR writes must not populate namespaces restored by trusted release jobs. Do not use elevated PR events, shared writable host caches, broad permission changes or cross-trust restore fallbacks. Cache misses or unavailable services fall back to normal preparation; build/test failures remain failures. Provide an explicit uncached validation path.
- **Tune only measured bottlenecks.** After caching, evaluate the three-job matrix cap against queue time and actual runner capacity. Retain or adjust it with measured elapsed-time and runner-minute evidence. Remove redundant setup/helper compilation only if the simplification preserves checkout identity and local behavior. Avoid a second helper-artifact pipeline unless its measured savings justify the extra machinery.

## Risks / Trade-offs

Cache transfer can cost more than the computation saved; retain only effective caches and bound local cache ownership/cleanup explicitly. Runner variance and mutable fixture image tags affect comparisons; record observed image identities where available and do not claim causation from incomparable runs.

Cancellation can interrupt container cleanup. Exercise cancellation and recovery rather than relying on YAML validity. Cached compilation must not suppress a deliberately failing test. Local commands must still work with caching disabled and without CI environment variables.

## Migration Plan

Record the baseline, implement scoped cancellation, then add image and compiler reuse. Validate each boundary before tuning parallelism. Run existing checks and every matrix case cold and warm, then publish a concise timing comparison and update operator/developer documentation.

Rollback consists of disabling cache integration or reverting workflow changes. No product data or installation migration is involved. Implementation requires its own publication authorization; this planning task does not trigger CI or push changes.

## Grounding

GitHub documents [output-driven matrices](https://docs.github.com/en/actions/using-jobs/using-a-matrix-for-your-jobs) and [concurrency controls](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency). Docker documents [Buildx caching in Actions](https://docs.docker.com/build/ci/github-actions/cache/); setup-go documents [module and compiler caches](https://github.com/actions/setup-go#caching-dependency-files-and-build-outputs).

[GitHub CLI](https://github.com/cli/cli/blob/trunk/.github/workflows/go.yml), [Restic](https://github.com/restic/restic/blob/master/.github/workflows/tests.yml) and [Caddy](https://github.com/caddyserver/caddy/blob/master/.github/workflows/ci.yml) use standard matrices and project-specific acceptance helpers. They support retaining the current structure, not copying unrelated release complexity. Verify applicable action versions and cache behavior against current official documentation during implementation.

## Verified implementation choices

Official action releases checked on 2026-09-12: Docker setup-buildx v4.3.0 (2026-08-19), Docker build-push v7.3.0 (2026-07-01), and actions/cache v6.1.0 (2026-06-26). Their action manifests use Node 24; hosted Ubuntu runners support this runtime. Workflow references pin the verified tag commits. Docker's [cache documentation](https://docs.docker.com/build/ci/github-actions/cache/) requires cache API v2 with Buildx >= 0.21 and BuildKit >= 0.20; the official setup action supplies the hosted builder.

Layer namespaces are `checks-<architecture>-<trust>` and `systemd-<architecture>-<trust>`. Compiler keys include runner OS/architecture, trust, a hash of Go and validation-toolchain manifests plus the validation Dockerfile, and source revision. Restore prefixes never drop compatibility or trust. Within restored storage, the immutable local image ID separates actual architecture/compiler/toolchain bytes. Tests retain `-count=1`; helpers are rebuilt into disposable directories from the checkout.

| Context                      | Namespace | Restore authority                                 |
| ---------------------------- | --------- | ------------------------------------------------- |
| Default-branch push/dispatch | trusted   | Current/default branch caches                     |
| Tag and release validation   | trusted   | Tag and default branch caches                     |
| Other branch                 | branch    | Current/default ref subject to matching namespace |
| Pull request                 | branch    | PR merge ref and accessible base refs             |

GitHub's [ref isolation](https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching) prevents PR merge-ref caches from being restored by default-branch or tag runs. No elevated PR trigger or cross-trust restore prefix is used. Namespace names are defense in depth, not a replacement for platform ref isolation. Layer export errors are ignored; actual builds and tests must succeed. Manual dispatch and reusable calls support `disable_cache` for an uncached run.

Local `HOSTLENS_BUILD_CACHE` is opt-in and must name an existing caller-owned mode-0700 directory. Missing or unsafe parents fall back to disposable compilation. Only its image-scoped non-secret leaf is mounted writable; source and module mounts remain read-only. The private parent prevents other host users traversing the mode-1777 leaf needed for Docker user remapping. Cache storage is caller-owned and removable when no validation uses it.

Ordinary concurrency keys include workflow, event and branch/PR identity. Release validation selects a unique run ID and disables cancellation; its caller retains a distinct group. This keeps push and PR validation distinct and avoids reusable caller self-cancellation. The matrix cap remains three unless measurements justify changing it.

Hosted restoration assigns compiler files to the runner UID. Before mounting the selected leaf, the launcher grants read/write access to caller-owned files and traversal/write access to caller-owned directories. This is restricted to the already-private cache parent, does not follow cache symlinks, and leaves local remapped-container-owned entries untouched. Without this normalization, capability-dropped containers cannot overwrite restored Go cache entries.
