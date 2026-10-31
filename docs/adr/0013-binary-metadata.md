# 13. Reading metadata from binary files

Date: 2026-10-31 · Status: accepted

## Context

Some files are personal data as a whole, or carry it where a line scanner does not look:

- a database dump or backup committed next to the code (`.sql`, `.dump`, `.bak`, `.sql.gz`, pg_dump's custom
  format) holds copies of whole tables;
- an office document names its author in its metadata (`dc:creator`, `cp:lastModifiedBy`), even when its body
  holds nothing personal;
- a photo taken with a phone records where it was taken in its EXIF GPS tags;
- a scan of a passport or identity card is an image or a PDF, often named after what it is.

ATO is one static binary without cgo (ADR 1), and it scans folders it does not trust (ADR 7). Reading binary
formats means parsing input written by anyone, in formats with offsets, lengths and compression that can point
anywhere or expand without bound.

## Decision

1. A new package, `internal/binmeta`, reads only what the findings need, with the standard library only
   (`archive/zip`, `encoding/xml`, `compress/zlib`, `encoding/binary`). It imports nothing from this module, and
   `internal/archtest` allows only `engine` to import it.
   - EXIF: JPEG markers up to the start of scan, the TIFF header and IFDs, the GPS IFD's latitude and longitude.
     PNG `eXIf` and WebP `EXIF` chunks are found by their chunk lists. HEIF and AVIF are not walked box by box:
     the reader looks for `Exif\0\0` followed by a TIFF header.
   - Office: Office Open XML `docProps/core.xml` and OpenDocument `meta.xml`, for `dc:creator`,
     `cp:lastModifiedBy` and `meta:initial-creator`. No other part is decompressed.
   - PDF: literal strings in content streams, inflated when Flate-compressed, only to look for machine readable
     zones with the `mrz` detector.
   - pg_dump custom format: the `COPY ... FROM stdin;` statements in the table of contents, which name each
     table and its columns. The rows are compressed and are not read.
2. Every reader works on bytes read under a cap (256 KiB for a JPEG, 32 MiB otherwise, below `max_file_size`
   and the archive byte budget when the file sits in an archive) and bounds what it derives from them: IFD
   entries and chained IFDs, TIFF counts and offsets checked against the data, zip entries (10,000), metadata
   part size (1 MiB, read through a limit whatever the zip header claims), author length and count, PDF streams,
   inflated bytes per stream and per file, and lines. Encrypted zip entries are not opened. `encoding/xml`
   neither fetches DTDs nor expands external entities. Malformed input gives no result, never a panic; each
   reader has a fuzz test.
3. Findings about a file use a new kind, `file`. They sit at line 1 column 1 (a text dump at the line of its
   first row statement), cannot be replaced by `ato fix`, and never carry a value: a dump names the table and
   the columns whose names say they hold personal data; an author and a GPS position are masked; an ID scan
   names the word in its file name.
4. Not read stays not read (ADR 8). Reading an office document's metadata, a PDF's text for an MRZ or a pg_dump
   archive's table of contents is not reading its body or rows, so these files stay listed as not read in full,
   with a reason that says what was read. Images are counted as scanned: their metadata is what was asked.
5. No OCR. Recognising text in images needs a model or a native library, which a single static Go binary does
   not carry. Scans of identity documents are found by name only (passport, carte d'identité, Personalausweis,
   buletin, dowód osobisty and their equivalents in the EU languages), with medium confidence for a word in the
   file name and low for an ambiguous word or a folder name, and a PDF's own text is searched for an MRZ. An image
   of a passport named `IMG_0001.jpg` is not found.
6. Each detector (`database-dump`, `document-author`, `photo-location`, `id-document-scan`) is on by default and
   can be switched off under `[detectors]`. A format is only parsed when a detector that uses it is on.

## Consequences

- A committed dump, a photo with a position or a document with its author's name now fails a scan.
- The findings report gains the kind `file` (schema 2.2), an optional enum value under the compatibility rules.
- Readers of new binary formats follow the same rules: standard library, a cap on input and on everything
  derived from it, a fuzz test, and a not-read status for whatever part is not read.
