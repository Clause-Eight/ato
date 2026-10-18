# Draft: conference talk proposal

Status: draft, ready to submit to a security or developer conference.

**Title:** Check digits beat regex: finding EU personal data in code without drowning in false positives

**Format:** 30 minutes, technical, with a live demo.

**Abstract**

Every European country hides structure in its national identifiers: a birth date, a county code, a weighted check digit. Most data-loss tools ignore that structure and match on shape, so a ten-digit order number looks the same as a Bulgarian personal number, and teams turn the scanner off after the first week.

This talk walks through how the identifiers of Romania, Bulgaria, Hungary, Poland, Czechia, Slovakia and Croatia are built, and how their own rules (mod 11, mod 97, ISO 7064, Luhn, dates in disguise) make a scanner both quieter and more accurate. It shows the results of a synthetic benchmark (98% precision against 42% for pattern matching) and what scanning a 15,000-file open-source repository taught about false positives: decimal digits, signed integers and tables of random seeds.

It closes with how to keep the scanner itself from leaking: masked reports, value-free fingerprints, and why found values never belong in CI logs. The tool shown is pii-eu, a free, open-source Go CLI.

**Takeaways**

- How common EU identifiers encode checks you can use, and their failure rates on random input.
- A practical confidence model: validation, nearby keywords, test paths and published sample values.
- How to run a personal-data scan in pre-commit and CI without making the CI logs a new leak.

**Speaker notes for the committee**

All examples are synthetic and generated live; no real personal data is shown.
