# 5. pii-eu is sold on its own and inside Clause Eight

Date: 2026-10-27 · Status: accepted

## Context

pii-eu started as a scanner inside Clause Eight. Its inventory, data map and erasure checks are useful to DPOs
and security teams who do not buy a compliance scanner. docs/PRODUCT.md describes the buyers and packaging.

## Decision

- One code base and one binary serve both: the standalone product and the Clause Eight scanner.
- The JSON reports stay the contract between them; Clause Eight reads them as it does today.
- Packages differ by what a customer is given (builds, licence file, report templates), never by switches in
  the code. Anything checked on the customer's machine can be removed, so it is not a control.
- Features that only make sense with a server (history, dashboards, signed evidence) stay in Clause Eight
  Cloud.

## Consequences

- No feature flags for paid tiers in pii-eu.
- A release must keep Clause Eight's integration working: Clause Eight pins a pii-eu version and its tests
  replay a recorded report.
- Distribution without repository access is needed before the first standalone sale (ADR 0006).
