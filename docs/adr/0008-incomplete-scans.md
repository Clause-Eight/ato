# 8. A clean result means everything was read

Date: 2026-10-30 · Status: accepted

pii-eu is called ATO since 2.0.0; see [ADR 12](0012-rename-to-ato.md).

## Context

Exit code `3` meant "some files could not be read": a read error, a corrupt archive, an archive limit reached.
Several other ways of not reading data still ended in `0`, "No personal data found":

- a `.zip` file, which was not opened (only gzip and tar were);
- an office document, PDF, database file or another compression format, skipped as binary;
- a CSV, TSV or JSON Lines file cut at `max_rows` (100,000 by default), shown only as "1 sampled";
- a file over `max_file_size`, or an archive nested deeper than `max_archive_depth`.

The 2026-10-30 review reproduced each of them (#258). In CI, `0` lets a change through. A privacy check that
passes on data it never read gives false assurance, which is worse than no check, and the gap is invisible to
the person reading the result.

## Decision

1. A skipped or sampled file that may hold personal data and was not read in full is marked `NotRead`
   (`not_read` in the JSON report). That covers files cut at `max_rows`, over a size or archive limit, nested too
   deep, and files in formats pii-eu does not open but that often hold personal data (office documents, PDFs,
   `.7z`, `.rar`, `.bz2`, `.xz`, `.zst`, `.lz4`, SQLite and Access databases, Parquet, Avro). Other binary files
   (images, fonts, executables) are skipped as before and do not count.
2. A scan exits `3` when a file could not be read, or when any file is `NotRead`. Findings still take
   precedence: a scan with findings exits `1`.
3. The text report lists each such file: "did not read all of FILE (reason)".
4. `--allow-partial` and `allow_partial = true` keep the exit code at `0` or `1` for files not read in full.
   Unreadable files always exit `3`.
5. `.zip` files are opened like tar files, with the same byte, entry and depth limits. A zip needs random
   access, so it is read into memory first, counted against `max_archive_bytes`. Encrypted entries are listed as
   unreadable.

## Compatibility

`docs/COMPATIBILITY.md` promises that the meaning of exit codes does not change within a major version. We
treat this as a correction of what `3` meant to say: the scan is incomplete. A team that relied on `0` for
sampled files gets `3` after upgrading, sees the files listed with the reason, and can set `allow_partial`.
The change ships in a minor release and is named in the release notes and in COMPATIBILITY.md.

## Consequences

- CI gates no longer pass on unread exports or office files.
- Repositories with large fixtures now report them; most teams will either raise `max_rows`, exclude the
  path, or set `allow_partial` deliberately, which is the decision we want them to make.
- Reading office documents and PDFs remains on the backlog (v1.5); until then they are reported as not read
  rather than ignored.
