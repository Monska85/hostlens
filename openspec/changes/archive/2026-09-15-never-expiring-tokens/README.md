# never-expiring-tokens

Allow administrators to create bearer tokens with no expiration (--expires never) using a zero-timestamp store sentinel that older binaries fail closed on, with friendly list display and rotation that imposes a finite overlap deadline on the old token.
