# 15. Licenses checked offline

Date: 2026-10-31 · Status: accepted

## Context

ATO is proprietary (ADR 3) and will be sold per organisation per year. Its buyers scan code that must not
leave their machines, often on machines without internet access, and they buy ATO because it sends nothing
anywhere. A license check that calls a server would break that promise and fail on locked-down machines.

A static binary can always be patched, so no check stops a determined copier. What a license must do is tell an
honest customer what they bought and until when, and give the contract a clear technical counterpart.

## Decision

- A license is a token: the terms (id, organisation, plan, seats, features, issued and expiry dates, key ID) as
  JSON, signed with Ed25519. Its format is `ato-license-v1.<payload>.<signature>`, base64url without padding.
  A license file may hold comment lines around the token; they are never trusted.
- ATO carries only public keys, one `internal/license/keys/<id>.pub` per signing key, and checks a token with
  no network access. Private keys stay with Clause Eight, outside the repository; `cmd/ato-license` makes keys
  and signs licenses and is not part of a release.
- ATO reads the license from `ATO_LICENSE` (the token, or a file's path; for CI) or from the file
  `ato license install` writes to the user's config folder (`<config>/ato/license`, mode 0600).
- A license is valid through its expiry day (UTC) and keeps working for 30 days after it. Without a valid
  license ATO still scans and prints one line on stderr saying it runs as an evaluation; output, reports and
  exit codes are the same. Plans and features are recorded in the token so that a later release can limit what
  an evaluation does without a new license format.
- Adding a key: generate it, commit its `.pub`, release. Rotating: keep old `.pub` files so the licenses they
  signed keep working. Revoking a leaked key: delete its `.pub` and release; every license it signed stops
  working and must be reissued.

## Consequences

- No license server to run, and nothing leaves the customer's machine.
- A license cannot be revoked one by one before it expires; terms are kept short (one year) and a leaked key is
  revoked with a release.
- The machine's clock decides expiry; a customer who turns it back is in breach of contract, not stopped.
- `ATO_LICENSE` is the first environment variable the `ato` command reads.
