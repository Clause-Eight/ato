package art9

import (
	"fmt"
	"regexp"
	"strings"
)

// A person's genotype at a SNP is written as its dbSNP reference (rs and a number, https://www.ncbi.nlm.nih.gov/snp/docs/RefSNP_about/)
// and the two alleles called, such as AG or A/G. Consumer genotyping exports put one call per line: the rsID,
// the chromosome, the position and the genotype, tab- or comma-separated, the genotype as two letters or as two
// columns. Code and fixtures write calls as keys and values, such as "rs53576": "AG".
//
// A file is reported once: when it has five or more calls in the export layout, or three or more as keys and
// values, or one with the word genotype on its line or the line before. A variant written with its two possible
// alleles (rs334 A/T in a catalogue) is not a call unless the file says genotype; a reference to rs53576 alone
// never is.

var (
	// exportCall is one line of a genotyping export: rsID, chromosome, position, then one two-letter genotype or
	// two one-letter alleles.
	exportCall = regexp.MustCompile(`^\s*"?(rs\d{1,10})"?\s*[\t,;]\s*"?(?:\d{1,2}|X|Y|XY|MT|M)"?\s*[\t,;]\s*"?\d{1,10}"?\s*[\t,;]\s*"?([ACGTDI-]{2}|[ACGTDI0-]"?\s*[\t,;]\s*"?[ACGTDI0-])"?\s*$`)
	// keyedCall is a call written as a key and a value.
	keyedCall = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])["']?rs\d{2,10}["']?\s*(?::|=>|=)\s*["']?([ACGT]{2}|[ACGT][/|][ACGT])(?:["']|[^A-Za-z0-9]|$)`)
	// slashedCall is an rsID followed by two alleles with a slash, which is a call only when the file says so.
	slashedCall = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])rs\d{2,10}\b\W{1,3}[ACGT][/|][ACGT]\b`)
)

var genotypeWords = []string{"genotype", "genotyp", "génotype", "genotipo", "genótipo", "genotip", "genotüüp", "genotyyppi", "генотип", "γονότυπος"}

type genotypeScanner struct {
	export, keyed            int
	exportSeen, saidGenotype bool
	firstExport, firstKeyed  place
}

func (g *genotypeScanner) line(at place) {
	if !strings.Contains(at.text, "rs") {
		return
	}
	if m := exportCall.FindStringSubmatch(at.text); m != nil {
		if !g.exportSeen {
			g.firstExport, g.exportSeen = at, true
		}
		if strings.ContainsAny(m[2], "ACGT") {
			g.export++
		}
		return
	}
	keyed := keyedCall.FindAllStringIndex(at.text, -1)
	n := len(keyed)
	said := mentionsGenotype(at.previous + "\n" + at.text)
	if n == 0 && said {
		n = len(slashedCall.FindAllStringIndex(at.text, -1))
	}
	if n == 0 {
		return
	}
	if g.keyed == 0 {
		g.firstKeyed = at
		if len(keyed) > 0 {
			g.firstKeyed.column = len([]rune(at.text[:keyed[0][0]])) + 1
		}
	}
	g.keyed += n
	g.saidGenotype = g.saidGenotype || said
}

func mentionsGenotype(text string) bool {
	lower := strings.ToLower(text)
	for _, w := range genotypeWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

func (g *genotypeScanner) end(s *Scanner) []Hit {
	switch {
	case g.export >= 5:
		return []Hit{s.hit(Genotypes, g.firstExport, fmt.Sprintf("genotype export (%d calls)", g.export),
			[]string{fmt.Sprintf("%d genotype calls by rsID in the layout of a genotyping export", g.export)}, "")}
	case g.keyed >= 3 || g.keyed >= 1 && g.saidGenotype:
		reasons := []string{fmt.Sprintf("%d genotype calls by rsID", g.keyed)}
		if g.saidGenotype {
			reasons = append(reasons, "the word genotype is nearby")
		}
		return []Hit{s.hit(Genotypes, g.firstKeyed, fmt.Sprintf("genotype calls (%d)", g.keyed), reasons, "")}
	}
	return nil
}
