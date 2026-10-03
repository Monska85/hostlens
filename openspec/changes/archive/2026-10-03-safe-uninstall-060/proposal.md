## Why

The 0.6.0 rewrite has no uninstall command. The 0.5.0 command can spend its whole deadline scanning large backup mounts and leave a partially removed installation.

## What changes

Add a preview-first Linux uninstall that verifies installed HostLens files before stopping services or removing them. The applied command removes the known systemd units and installed binary, with named progress stages. It retains configuration, secrets, service identities, journals, and administrator grants and reports them explicitly. It never runs an unbounded ownership scan or deletes an account without proof about stray files.

## Impact

`hostlens uninstall --apply` becomes the supported bounded removal path for a 0.6.0 system installation. Identity and administrator data cleanup remains an explicit operator decision documented in the removal guide.
