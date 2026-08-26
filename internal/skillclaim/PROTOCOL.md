# Agent Skills claim protocol v1

The neutral root is `$XDG_STATE_HOME/agent-skills/claims/v1`; a producer never owns a shared
runtime root. The filename is `sha256(canonical destination)`. A claim binds one top-level skill,
not a runtime root. `kind=file` uses the sole record path `.`; directory records are sorted relative
paths. SHA-256 strings are lowercase hex.

Claims are strict JSON: unknown fields, duplicate keys and trailing JSON values are rejected.
Required lifecycle fields are `state`, `operation_id`, and monotonic `generation`. States are
`reserved`, `active`, `updating`, `releasing`, `restoring`. A non-active record is fail-closed unless
the same operation continues under the ordered claim lock. Creation is O_EXCL; updates bind both
previous generation and canonical-JSON digest. Writers sync the claim and its parent directory.
