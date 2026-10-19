# Name lists: sources and licence

The files `<country>.txt` in this folder are the name list of the `person-name` detector, one name per line,
embedded into the binary. They hold no person: only given names and surnames, each shared by many people.

## Source

All lists are compiled from [Wikidata](https://www.wikidata.org). Wikidata's structured data is published under
[CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/) (public domain dedication), see
<https://www.wikidata.org/wiki/Wikidata:Licensing>. No other source was used.

## How the lists were made (October 2026)

For each EU member state, with its Wikidata item (Kingdom of Denmark, Q756617, for Denmark):

1. Query the Wikidata Query Service for the given-name items (property P735) and the family-name items (P734)
   of people whose country of citizenship (P27) is that state, counted per item, and keep the 300 most frequent of
   each kind.
2. Take each item's labels in the country's languages and in `mul` (Bulgarian, Greek, Cypriot and Maltese lists also take
   the English label, so both the native script and the usual Latin spelling are present).
3. Keep labels that are a single word written with a capital letter followed by lower-case letters, then the 250
   most frequent given-name items and the 250 most frequent family-name items.
4. Drop placeholder words that appear in name fields of test data without being anybody's name (Test, User,
   Admin, Null, Doe, Mustermann and similar).

The frequency is a proxy: it counts people notable enough to be in Wikidata, not the population. Common names are
well represented; rare and recent names are missing, and some names are over-represented by history. That is
acceptable for a detector that only needs to recognise the usual names in a field already labelled as a name.

## Updating

Run the same two queries per country (counts first, then labels for the counted items), apply the filters above
and replace the files. Keep the two header lines in each file.
