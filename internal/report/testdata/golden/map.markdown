# Data map

Drafted by pii-eu test from the code. It is a starting point for the record of processing activities (GDPR Art. 30): review purposes, legal bases and retention periods, which code cannot show. Erasure and retention show whether the code has a path that removes the data, not that it runs.

Categories of personal data: name, special-category/health.

## services/billing

| Recipient | Purpose | Established in the EEA | Where |
|---|---|---|---|
| Stripe | payments | no: check transfers and hosting region | `services/billing/requirements.txt:2` |

Personal data written to logs or telemetry:

- Personal data written to a log (contact) at `services/billing/api.py:9`

## services/clinic

| Store | Categories of personal data | Special categories (Art. 9) | Erasure (Art. 17) | Retention (Art. 5(1)(e)) | Source |
|---|---|---|---|---|---|
| `patients` | name | health | SQL DELETE (`services/clinic/repo.sql:7`) | none found | `services/clinic/db/001.sql:3` |

| Recipient | Purpose | Established in the EEA | Where |
|---|---|---|---|
| Mistral AI | AI model provider | yes | `services/clinic/notes.py:1` |

