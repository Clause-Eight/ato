package detectors

import (
	"regexp"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// German tax number (Steuernummer, St.-Nr.), given by the local tax office to sole traders, freelancers and
// other taxpayers who file returns. Each state writes it in its own layout of tax office (F), district (B),
// serial (U) and a final digit (P): FF/BBB/UUUUP in most states (Baden-Württemberg also writes FFBBB/UUUUP),
// FFF/BBB/UUUUP in Bayern and the eastern states, 0FF/BBB/UUUUP in Hessen and FFF/BBBB/UUUP in
// Nordrhein-Westfalen, ten or eleven digits in all. The thirteen-digit federal form puts a state code in front of
// the tax office and a 0 after it: 28 Baden-Württemberg, 9 Bayern, 11 Berlin, 30 Brandenburg, 24 Bremen,
// 22 Hamburg, 26 Hessen, 40 Mecklenburg-Vorpommern, 23 Niedersachsen, 5 Nordrhein-Westfalen,
// 27 Rheinland-Pfalz, 10 Saarland, 32 Sachsen, 31 Sachsen-Anhalt, 21 Schleswig-Holstein, 41 Thüringen. The
// rules for the final digit differ by state and are not published, so it is not verified and a keyword on the
// value's own line is required. Sources: https://de.wikipedia.org/wiki/Steuernummer (the layouts, with an example per state) and
// python-stdnum's de.stnr, https://github.com/arthurdejong/python-stdnum/blob/master/stdnum/de/stnr.py
func init() {
	Register(Detector{
		Name:          "de-stnr",
		Label:         "German tax number (Steuernummer)",
		Country:       "DE",
		OnByDefault:   true,
		Pattern:       regexp.MustCompile(`\d{2,3}/\d{3}/\d{5}|\d{3}/\d{4}/\d{4}|\d{5}/\d{5}|\d{13}|\d{10,11}`),
		Normalize:     DigitsOnly,
		Validate:      validStNr,
		Formatted:     regexp.MustCompile(`^\d{2,3}/\d{3,4}/\d{4,5}$|^\d{5}/\d{5}$`),
		NeedsKeyword:  true,
		KeywordOnLine: true,
		Keywords:      []string{"st.-nr", "st.nr", "stnr", "steuer-nr", "steuernr", "german tax number"},
		Concepts:      []Concept{TaxID},
		Base:          domain.Low,
		Validation:    "a state's layout, and the state code of the thirteen-digit federal form",
	})
}

// stnrStates are the two-digit state codes of the federal form; Bayern (9) and Nordrhein-Westfalen (5) use one
// digit and a three-digit tax office.
var stnrStates = map[string]bool{
	"10": true, "11": true, "21": true, "22": true, "23": true, "24": true, "26": true, "27": true,
	"28": true, "30": true, "31": true, "32": true, "40": true, "41": true,
}

func validStNr(s string) bool {
	if _, ok := Digits(s); !ok {
		return false
	}
	switch len(s) {
	case 10, 11:
		return true
	case 13:
		return s[4] == '0' && (s[0] == '9' || s[0] == '5' || stnrStates[s[:2]])
	}
	return false
}
