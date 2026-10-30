# 11. Whole quoted values of distinct identifiers

Date: 2026-10-31 · Status: accepted

## Context

After ADR 10 the open-source benchmark caught 71.9% of inserted values against Presidio's 54.1%, but Presidio
was still ahead on PESEL, OIB, Spanish DNI and Bulgarian EGN. pii-eu reported those only with their own keyword,
because their base confidence is low: a bare eleven-digit number passes one of them often enough to be noise.

Data and fixtures write values as quoted strings: `"ref": "…"`, `'external_id' => '…'`. A quoted string that is
exactly a valid identifier is stronger evidence than the same digits inside a line. We tried raising every such
value one step. On the benchmark it caught 98.5%; on four open-source applications without inserted values
(Mastodon, OpenEMR, Medusa, ERPNext) it added about 1,250 findings, nearly 1,000 of them quoted medical and billing
codes in OpenEMR that passed plain checksums (Czech birth numbers, Greek AFM, Austrian tax numbers, TAJ, NIF, BSN,
NIP), and about 260 fake emails in tests (#274).

## Decision

1. A value that is the whole of a quoted string is raised one step only for identifiers that a random string of
   their shape rarely passes: those with a date of birth and a check digit, or a check letter (CNP, EGN, PESEL,
   OIB, DNI, NIE, Italian fiscal code, NIR, German tax ID, the Belgian, Estonian, Lithuanian, Slovenian and
   Finnish personal numbers). The list is `distinct` in `internal/engine/line.go`.
2. It does not stack with a raise from a record's other personal fields, and never applies to a value context
   has lowered.
3. Emails are not raised this way. Presidio reports every email; most emails in code and tests are fake, and the
   placeholder domains in #271 catch only some of them.
4. Cards under names that usually label other numbers (`id`, `ref`, `order`) stay lowered.

## Consequences

- Open-source benchmark at the defaults: 88.7% caught, 99.4% of warnings real; the four applications gain 2
  findings.
- With every detector on, an unlabelled quoted eleven-digit value is often valid for several countries and is
  reported under one of them; the per-detector table counts that as a false positive for the others.
- pii-eu stays behind Presidio on emails and on cards under ID-like names, by choice.
