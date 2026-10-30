# 9. How pii-eu is compared with other scanners

Date: 2026-10-30 · Status: accepted

## Context

The product page and docs/BENCHMARKS.md compare pii-eu with other scanners. The first comparison (19 and 30
October 2026) had weaknesses found in the 2026-10-30 review (#260):

- The only corpus came from pii-eu's own generator: the same author wrote the values, the noise and the
  validators, and the noise had nine templates with no millisecond timestamps or minified JSON.
- docs/BENCHMARKS.md reported numbers with every detector on (99.6% precision), while the page reported the
  defaults (99.9%): two numbers for one claim.
- Presidio ran line by line with its prose recognisers (names, places, dates, URLs) on code, which no one
  scanning code would do, and its speed was measured that way.
- The comparison script lived outside the repository, ran one seed and pinned no versions.
- HoundDog.ai, the closest competitor for personal data in code, was not compared.

A comparison that names other products has to be fair and checkable, or it should not be published.

## Decision

1. The comparison lives in `bench/compare/`: the script, pinned Python requirements, and a README with the
   exact commands. `results.json` records every tool's version, the open-source commits and the date.
2. pii-eu runs with its defaults only. Published precision and recall are default numbers; all-detector numbers
   stay in the per-detector table of docs/BENCHMARKS.md, labelled as such.
3. Each other tool runs at its documented defaults, changed only where a default would be unfair to it, and
   every change is written down: Presidio's EU recognisers are enabled for English, its prose recognisers are
   left out, and it uses its batch engine.
4. Two value corpora: the generated one (three seeds) and one built from real open-source application files
   (fixtures, migrations, seeders, specs) plus a generated application log with millisecond timestamps, with
   generated values inserted. The noise of the second is not written by us.
5. Personal data in logs is compared with HoundDog.ai on generated code in eight languages, including cases that
   go through a local variable, which pii-eu misses (#265). HoundDog does not look at values, so it is not in
   the value tests.
6. Scoring is per line and per value; a catch on an inserted line counts whatever type the tool reported, which
   favours the other tools. Alarms on original open-source lines are reported with their types, since some of
   them are real example data.
7. The page shows one precision number, from the default run, with a link to the method.

## Consequences

- Anyone with the repository can rerun the numbers; the published table changes when the code does.
- The comparison shows where pii-eu loses (emails without a keyword, indirect log calls) as well as where it
  wins, and the page keeps those rows.
- The generated values still come from `internal/synth`; valid identifiers need a generator that knows each
  rule. The independence is in the surrounding text, not the values.
