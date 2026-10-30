# 10. What context may do to a value's confidence

Date: 2026-10-30 · Status: accepted

## Context

A value starts at its detector's base confidence and moves with its context: its own keyword raises it, an
unlikely word, a line of long numbers or a time-like name lowers it, and until now a test, fixture or example
path lowered it too. The default report level is medium, so a low-base identifier (PESEL, OIB, EGN, TAJ, BSN,
DNI, emails) in a test file was reported only with its own keyword, and before #256 often not even then.

The benchmark of 30 October 2026 on open-source application code (docs/BENCHMARKS.md) caught 38.8% of inserted
values, against 54.1% for Presidio. Most misses were in tests, fixtures, seeders and migrations, which is where
the product's main use case lives: real data copied from production into test data. The test-path rule
existed to keep made-up test data quiet, but a value that passes its own check digit is just as likely to be
real in a fixture as anywhere else, and more likely to cause harm there, since fixtures are shared and copied.

## Decision

1. A test path no longer lowers a value from a detector that is on by default. Those detectors confirm values
   with a check digit or an equivalent rule. The reason list still says "in a test, fixture or example path".
2. Detectors without such a rule (names, addresses, dates of birth, postcodes, plates: the off-by-default and
   keyword-only detectors) keep the test-path step down; test data is full of made-up ones.
3. Context lowers a value one step at most (#256), and every signal is still listed.
4. A value without its own keyword, next to another personal field in the same record (first or last name,
   full name, date of birth, street or address line, postcode, nationality, gender) on its line or the line
   before, is raised one step. The reason names the field.
5. `bench/recall_test.go` checks recall and precision on application-style files with the default detectors in
   CI. Thresholds sit a few points below the measured values.

## Consequences

- On the open-source corpus at the defaults, with the other changes of the same milestone (#265, #267, #271): 71.9%
  of inserted values caught (was 38.8%), 99.2% of warnings real (was 97.8%). The remaining alarms are mostly
  placeholder emails on real domains in fixtures.
- Test data generated with valid identifiers (for example by a faker that computes check digits) is now
  reported. That is the intended trade: pii-eu cannot tell a generated valid number from a real one, and
  `pii-eu fix` replaces both with values that carry the synthetic mark, which are then reported as samples.
- Teams that relied on fixtures staying quiet will see new findings after upgrading; a baseline accepts them.
