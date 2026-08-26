# Agent Skills claim protocol v1

The neutral root is `$XDG_STATE_HOME/agent-skills/claims/v1`; a producer never owns a shared
runtime root. The filename is `sha256(canonical destination).json`. A claim binds one top-level skill,
not a runtime root. `kind=file` uses the sole record path `.`; directory records are sorted relative
paths. SHA-256 strings are lowercase hex.

Claims are strict JSON: unknown fields, duplicate keys and trailing JSON values are rejected.
Required lifecycle fields are `state`, `operation_id`, and monotonic `generation`. States are
`reserved`, `active`, `updating`, `releasing`, `restoring`. A non-active record is fail-closed unless
the same operation continues under the ordered claim lock. Creation is O_EXCL; updates bind both
previous generation and the framed claim digest defined below. Writers sync the claim and its parent
directory.

Canonicalize a destination by first making it absolute and clean. Starting at that path, use `lstat`
to walk toward the filesystem root until an existing ancestor is found, recording every missing
basename. Reject the destination when that first existing object is a symlink and no basename was
recorded. Resolve symlinks in the existing ancestor, append the recorded basenames in reverse order,
and clean the result. Claim and lock hashes use the UTF-8 bytes of that resulting platform-native
absolute path exactly as stored in `destination`.

The lock for `<digest>.json` is the directory `<digest>.json.lock`, created with an exclusive mkdir
and mode `0700`. For a multi-claim transaction, canonicalize and deduplicate destinations, sort the
full lock paths by byte order, then acquire every directory in that order. If any mkdir reports that
the directory exists, release only locks acquired by this attempt and fail closed; v1 has no timeout
or automatic stale-lock deletion. Hold all locks through claim and protected destination mutations,
then remove them in reverse order. Create the claims directory with mode `0700` before acquisition.
The single-claim convenience operations acquire and release only one such lock and must not be composed
to approximate a multi-claim transaction.

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
restoring under a new operation, and updating/releasing/restoring→active under that operation.
The latter releasing/restoring transitions are rollback-only and must retain the complete prior
payload. Removal is allowed only from releasing/restoring with the exact producer, operation,
generation, and framed predecessor digest.

Create a new claim as a mode `0600` regular file with exclusive creation, then sync the file and claims
directory. Replace an existing claim by writing and syncing a mode `0600` temporary regular file in the
same claims directory, atomically renaming it over the claim, syncing the resulting file, and syncing the
directory. Remove a claim and then sync the claims directory. Readers reject symlinks, non-regular files,
and any claim with group/other permission bits.
