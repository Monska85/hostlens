# harden-token-store-directory

Validate the token store parent directory (directory type, root-or-euid owner, no group/other write) on every read, not only on administrative changes, and share one verification helper for both paths.
