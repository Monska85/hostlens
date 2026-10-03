## Decision

The 0.6.0 archive has no ownership manifest and installations are performed by the administrator. Uninstall therefore owns only the three named systemd units and the installed executable. It validates all existing targets before any mutation. A target with an unexpected type or content blocks the operation rather than being overwritten or removed.

The normal path never walks the host filesystem. It disables and stops existing verified units, removes their files, reloads systemd, then removes the verified executable. Unit files are restored if removal or reload fails, leaving a verifiable retry path. Configuration, token store, profiles, users, groups, journals, and manual permissions remain, and the JSON report states this. A completed retry skips absent resources without contacting systemd.

Account deletion is deliberately separate: without a complete ownership inventory, the program cannot prove absence of stray files on local, backup, FUSE, or remote mounts. The old global scan was both slow and incomplete under a fixed deadline. A future separately authorized purge may use a mount-aware proof or explicit operator waiver, but 0.6.0 does not claim to provide that proof.
