# Apache-2.0 adoption tasks

## 1. License and distribution

- [x] 1.1 Add the official Apache-2.0 LICENSE and HostLens NOTICE with the owner's project identity and original repository.
- [x] 1.2 Include project licensing and attribution in both archive architectures, preserve dependency notices, and update current documentation.
- [x] 1.3 Verify archive contents, checksums, and missing-license failure in a disposable container; format Markdown and validate specifications.

## Verification evidence

The official Apache text was copied byte-for-byte. Both architectures were rebuilt with `scripts/build-archives.sh`. A disposable, network-disabled container verified project license and notice bytes, every manifest checksum, outer archive checksums, and upstream NOTICE preservation. Missing LICENSE and missing NOTICE each failed before archive output. Cached Prettier formatted Markdown after the wrapper failed to resolve npm. No runtime code, dependency versions, commits, publication, or signatures changed.
