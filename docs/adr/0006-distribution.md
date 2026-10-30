# 6. Distribution without repository access

Date: 2026-10-27 · Status: proposed

pii-eu is called ATO since 2.0.0; see [ADR 12](0012-rename-to-ato.md).

## Context

Customers install pii-eu today with access to the private repository, or with a token that can read its
releases. Neither works for a customer who is not in the Clause-Eight organisation. Keyless signing in
release.yml writes the repository's name to Sigstore's public transparency log, which a private product may not
want.

## Options

| Option | For | Against |
|---|---|---|
| Downloads from Clause Eight Cloud, per organisation token | One account for both products; downloads are audited; works offline after download | Needs the Cloud deployed |
| Private container registry (GHCR or ECR) with pull tokens | Easy for CI | Customers need a token per registry; binaries still needed for laptops |
| Signed binaries in a bucket with expiring links | No server code | Links get shared; no record of who downloaded |

Signing: a Clause Eight key held in a KMS, `cosign sign-blob --key` without uploading to the transparency
log, and the public key published on the product page.

## Proposed decision

Downloads from Clause Eight Cloud with organisation tokens, plus a container image in a private registry for
CI. Key-based signing replaces keyless signing for releases after this ADR is accepted.

## To decide

- Whether past keyless releases are a problem worth acting on.
- Which registry, once the Cloud hosting account exists.
