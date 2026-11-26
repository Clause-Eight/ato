# Install

ATO is one static program with no dependencies. Pick one way.

## With Homebrew (macOS and Linux)

```bash
brew install clause-eight/tap/ato
brew upgrade ato        # later, for a new release
```

## With Go

Needs Go 1.24 or later.

```bash
go install github.com/Clause-Eight/ato/v2/cmd/ato@latest
```

The program lands in `$(go env GOPATH)/bin` (usually `~/go/bin`). If `ato` is not found, add that folder to your `PATH`:

```bash
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc   # or ~/.bashrc
```

## From source

```bash
git clone https://github.com/Clause-Eight/ato.git
cd ato
go build -o ato ./cmd/ato
./ato version
```

## Release binaries

Binaries for Linux, macOS and Windows (amd64 and arm64) are attached to each release on the [releases page](https://github.com/Clause-Eight/ato/releases), with an SBOM per archive and a signed `checksums.txt`. To verify a download:

```bash
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/Clause-Eight/(ato|pii-eu)/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --ignore-missing -c checksums.txt
```

## Check it works

```bash
ato version
ato scan .
```

Coming from pii-eu 1.x? See [UPGRADING.md](UPGRADING.md).
