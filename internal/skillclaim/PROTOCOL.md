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

`source_digest` is `ManifestDigest(files)`: for sorted records, append UTF-8 `path`, NUL,
lowercase SHA-256, NUL for each record, then SHA-256 the byte stream. `Validate` recomputes it.

The CAS claim digest is language-neutral field framing. Start an empty SHA-256 and append each
UTF-8 key, NUL, UTF-8 value, NUL in this exact order: `protocol=agent-skills-claim-v1`, `schema`,
`name`, `kind`, `state`, `operation_id`, decimal `generation`, `destination`, `producer`, `format`,
`scope`, decimal `consumer_count`, one `consumer` per sorted value, `source_digest`, decimal
`file_count`, then `file_path` and `file_sha256` for every sorted record. The golden manifest and
claim digests are in [`testdata/digest-vectors.json`](testdata/digest-vectors.json).
Directory record paths are POSIX clean relative paths (no `.`, `..`, slash suffix, backslash, or
control byte). State transitions are reserved→active under one operation, active→updating/releasing/
restoring under a new operation, and updating/restoring→active under that operation.
