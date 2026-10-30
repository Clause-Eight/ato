# 1. Go, one static binary

Date: 2026-10-18 · Status: accepted

pii-eu is called ATO since 2.0.0; see [ADR 12](0012-rename-to-ato.md).

## Context

pii-eu runs in pre-commit hooks, CI jobs and on laptops. Install friction decides whether people keep it turned on. Rust and Go both give a single static binary; Python and Node need a runtime.

## Decision

Write pii-eu in Go. Ship one static binary per platform, with no runtime dependencies. Keep third-party modules to a minimum (TOML parsing only, at the start).

## Consequences

- Fast startup and parallel scanning come from the standard library.
- Cross-compiling for Linux, macOS and Windows on amd64 and arm64 is one release job.
- Detectors are plain Go files with table tests, which keeps "add a country" approachable for contributors.
