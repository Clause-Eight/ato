# 13. Health, genetic and biometric data found by context

Date: 2026-10-31 · Status: accepted

## Context

ATO reported Art. 9 data only by field name: a column called `diagnosis` or `face_embedding` is a
`special-category` finding, whatever it holds. The data itself went unseen: a fixture with a patient's diagnosis
code, an HL7 admission message copied from a hospital interface, a FHIR export, a genotype file or a stored face
embedding. These are the most harmful kinds of personal data to leak, and test data is where they get copied.

They do not fit a detector. A detector confirms one value with its own rule, and the value is personal data by
itself: a valid CNP names a person wherever it appears. A diagnosis code does not. "E11" names a disease, and codes
of that shape fill seat maps, part numbers, hex and version strings. It is health data only in a record about
someone. HL7 segments, FHIR resources, VCF files and templates have no single value to confirm at all; what makes
them personal data is the shape of the whole record.

## Decision

1. **Codes need a person.** ICD-10, ICD-11, ATC and Italian AIC codes are detectors (`icd10`, `icd11`, `atc-code`,
   `it-aic`) with their structure as the validator (ICD-10 chapter ranges, the ICD-11 stem layout, the ATC
   anatomical groups, the AIC check digit). Each needs a medical word within 60 bytes before it (`diagnosis`,
   `icd`, `atc`, `medication`, `farmaco`, in several languages; in a file with columns, the column's name) and a
   new `NeedsPerson` rule: the same record must name a person. A record is the row in a file with columns, and
   otherwise the text within 300 bytes of the code on its line and the line before, so that a minified file or a
   fuzzing corpus is not one record. It names a person when it has a patient or person key (`patient_id`, `mrn`,
   `ssn`, `pesel`), a personal field (`first_name`, `date_of_birth`), or another personal-data value that does not
   overlap the code. Short or overloaded words (`dx`, CIE and CIM for ICD, `diagnostic`) are not medical words:
   in a scan of a Go module cache they matched SVG offsets, colour spaces and linter output. A code inside another identifier (the first group of an
   IBAN read as an ICD-11 code) is dropped. "patient" is a person key but not a medical word: a room number
   next to a patient is not a diagnosis.
2. **Detectors carry an Art. 9 category.** `Detector.Special` sets `special_category` on their `value` findings.
3. **Records are a new kind.** The `internal/art9` package reads the shape of whole records, line by line with
   state across lines, and reports `record` findings: `hl7-patient` (a PID segment with a family name and an
   identifier or birth date), `fhir-patient` (a Patient with a name and a birth date or identifier, or a clinical
   resource whose subject is named or identified; a bare `Patient/123` reference is not enough), `vcf-genotypes`
   (a VCF with sample columns and a called genotype, reported once at the header), `rsid-genotypes` (five lines of
   a genotyping export, three rsID-genotype pairs, or one next to the word genotype) and `biometric-template` (a
   field named for a template given 64 numbers or more, or an encoded string of 100 characters). It imports
   `domain` only; the engine adds test paths, ignore comments and fingerprints, as it does for sinks. Each check
   is switched off in `[detectors]` like a detector.
4. **Records start high and follow ADR 10 for test paths.** They have no check digit, so a test or fixture path
   lowers them one step, to medium, which is still reported. Records about a placeholder are low samples: the HL7
   and FHIR specification example patients, stand-in names (Doe, Mustermann), names with digits as generated test
   data writes them, constant vectors, and the public 1000 Genomes and HapMap reference samples.
5. **Fingerprints never come from the record's line**, which holds its values: a record's fingerprint is its
   check, its description and its place among the file's records of that check.
6. The findings report schema becomes 2.2: kind `record`, and `special_category` on `value` findings.

## Consequences

- Precision first: a diagnosis or medicine code without a medical word and a person on its record is not
  reported, so codes in prose, in catalogues and in pretty-printed JSON whose person key is more than one line
  away are missed. FHIR resources whose `resourceType` comes after their name are missed too.
- On the benchmark corpus of the old detectors the results are unchanged (99.1% precision at the defaults); the
  new detectors, written in records with a patient key, are found without false positives.
- A scan of a 4.9 GB Go module cache (437,000 files) reports none of the new findings at the default level; the
  only ones at low are Synthea-generated FHIR encounters in a test fixture, marked as samples.
- `.vcf.gz`, BCF and other binary genomic formats are not read.
- `ato fix` replaces codes with synthetic codes of the same layout; it does not rewrite records.
