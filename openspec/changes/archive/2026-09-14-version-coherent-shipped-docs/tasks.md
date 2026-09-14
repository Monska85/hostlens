# Tasks: version-coherent-shipped-docs

## 1. Implementation

- [x] 1.1 Substitute the placeholder forms with the candidate version for shipped Markdown in `prepare.py`; correct the stale `hostlens-0.2.0` literal in `docs/v1/INSTALL.md` to the repository placeholder. Verify: `go build ./...` untouched, `python3 -m py_compile tools/release/prepare.py`.

## 2. Tests

- [x] 2.1 Release test asserts shipped docs carry the candidate archive name and preserve historical version prose. Verify: `scripts/test-container.sh` (release tests).

## 3. Validation

- [x] 3.1 Build a candidate and inspect shipped docs inside the archive for version coherence. Verify: `make package && make verify-archives`, `openspec validate version-coherent-shipped-docs`, `make build`.