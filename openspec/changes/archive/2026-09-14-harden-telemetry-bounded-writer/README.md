# harden-telemetry-bounded-writer

Replace the bytes.Buffer-embedded limitedBuffer in internal/telemetry with a plain bounded writer so promoted ReadFrom can no longer bypass the 1 MiB ceiling.
