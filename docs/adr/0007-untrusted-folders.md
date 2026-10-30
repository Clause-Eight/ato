# 7. Scanning folders we do not trust

Date: 2026-10-30 · Status: accepted

pii-eu is called ATO since 2.0.0; see [ADR 12](0012-rename-to-ato.md).

## Context

Inside a git work tree, pii-eu asks git for the file list so `.gitignore` applies exactly as git applies it,
and reads staged files from the index for pre-commit hooks. Git reads the repository's own `.git/config`, and
several settings there run a command: `core.fsmonitor` runs on `ls-files`, hooks run on some commands, and
pagers, external diff programs, credential helpers and SSH commands run when the matching command needs them.

Auditors and consultants scan folders they did not write: a client's tarball, an archive from a ticket, a
repository cloned for review. The 2026-10-30 review showed that `pii-eu scan .` in a folder whose
`.git/config` sets `core.fsmonitor` ran that command (#257). A scanner for sensitive data that runs the
scanned folder's code is a worse risk than the data it looks for.

## Decision

Every git call goes through one helper, `gitCommand` in `internal/walk/walk.go`, which:

- overrides the repository's command-running settings with `-c`, which takes precedence over every config
  file: `core.fsmonitor=false`, `core.untrackedCache=false`, `core.hooksPath` set to the null device,
  `core.pager=cat`, empty `core.sshCommand`, `core.gitProxy`, `core.askPass`, `diff.external` and
  `credential.helper`, and `protocol.allow=never`;
- sets `GIT_TERMINAL_PROMPT=0`, `GIT_OPTIONAL_LOCKS=0` (no index writes), and clears `GIT_EXTERNAL_DIFF` and
  `GIT_PAGER`;
- uses only commands that do not run content filters or text conversion: `rev-parse`, `ls-files`,
  `diff --cached --name-only --no-ext-diff --no-textconv`, and `cat-file blob` for staged content in place of
  `show`.

The user's own global and system configuration still apply, so a global excludes file keeps working: those are
trusted, the scanned repository is not.

A test builds a repository whose `core.fsmonitor` would create a file, lists and reads it, and fails if the
file appears.

## Consequences

- Scanning a client's folder is safe without `--no-git`, and `.gitignore` still applies.
- Any new git call must use `gitCommand`; the list of overridden settings is reviewed when git adds a setting
  that runs a command.
- `--no-git` remains the most conservative mode, since it does not run git at all.
