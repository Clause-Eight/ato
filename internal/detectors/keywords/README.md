# Keywords

One file per official EU language, named by its ISO 639-1 code. Each line is `concept: word, word, ...`.
A detector lists the concepts it cares about; a word from any language raises a match's confidence when it
appears on the same line or the line before.

Matching ignores case, diacritics, Greek accents, underscores and dashes, so `numéro`, `numero` and
`NUMERO_` all match `numéro`. Write words the way a native speaker would, with their diacritics.

Concepts: `national-id`, `tax-id`, `social-security`, `health-insurance`, `phone`, `email`, `bank-account`,
`card`, `vat`, `passport`, `id-card`, `birth-date`, `address`, `postcode`, `driving-licence`, `licence-plate`,
`location`, `name`.

Avoid words that are common in code for other reasons (`id`, `nr`, `code`, `number` on their own). Native
speakers: see the review checklist in CONTRIBUTING.md.
