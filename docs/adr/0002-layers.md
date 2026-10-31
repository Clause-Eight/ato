# 2. Layers and what may import what

Date: 2026-10-18 · Status: accepted

## Context

The tool will grow by adding detectors and output formats. Those additions must not touch the core.

## Decision

- `domain`: shared types, imports nothing from this module.
- `detectors`: one file per identifier, registered in a map; imports `domain` only.
- `walk`, `config`, `report`, `baseline`: import `domain` only.
- `schema` (fields in schemas and models) and `infra` (data stores in Terraform, ADR 4): import `domain` only.
- `binmeta` (metadata of binary files, ADR 13): imports nothing from this module.
- `art9` (health, genetic and biometric records, ADR 14): imports `domain` only.
- `engine`: runs detectors over files; imports `domain`, `detectors`, `walk`, `schema`, `classify`, `infra`,
  `binmeta` and `art9`.
- `cli`: the only package that reads arguments, prints and chooses exit codes.

`internal/archtest` fails the build when an import breaks these rules.

## Consequences

A new identifier is one new file in `detectors`; a new format is one new file in `report`.
