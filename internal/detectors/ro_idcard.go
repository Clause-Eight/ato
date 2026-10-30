package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/internal/domain"
)

// Romanian identity card series and number (seria și numărul cărții de identitate): a two-letter series for
// the issuing county (an abbreviation of the county or its seat, with an X or Z variant once a series runs out;
// Bucharest has several) and a six-digit number, often written "seria XX nr. 000000". There is no check digit,
// so a keyword is required. The series list is the county table in
// https://ro.wikipedia.org/wiki/Carte_de_identitate_românească, which follows the SIRUTA county abbreviations.
// It covers the cards issued before the 2021 electronic identity card.
func init() {
	Register(Detector{
		Name:         "ro-id-card",
		Label:        "Romanian identity card series and number (carte de identitate)",
		Country:      "RO",
		OnByDefault:  true,
		Pattern:      regexp.MustCompile(`[A-Z]{2}(?: ?-? ?| ?(?:nr|NR|Nr)\.? ?)\d{6}`),
		Normalize:    normalizeROIDCard,
		Validate:     validROIDCard,
		Needs:        Needs{Check: func(line string) bool { return capsThenDigit(line, 2, 5) }},
		Keywords:     []string{"serie", "seria", "buletin", "carte de identitate", "ci", "serie ci", "seria ci", "nr ci"},
		Concepts:     []Concept{IDCard},
		Base:         domain.Low,
		NeedsKeyword: true,
		Validation:   "county series is issued and the number has six digits",
	})
}

var roIDCardSeries = func() map[string]bool {
	m := map[string]bool{}
	for _, s := range strings.Fields(`DP DR DT DX RD RR RT RX RK RZ AX AR ZR AS AZ XC ZC XH ZH XB XT ZT BV ZV XR XZ ZB
		KS KL KX CJ KT KZ TX KV DD ZD DZ GL ZL GG GZ HR HD XD SZ MX MZ IZ IF XF MM XM MH ZS MS NT NZ OT SL
		PH PX PK SM SX SB SR SV XV TR TM TZ TC VS XS VX VN`) {
		m[s] = true
	}
	return m
}()

func normalizeROIDCard(s string) string {
	if len(s) < 2 {
		return s
	}
	return s[:2] + DigitsOnly(s[2:])
}

func validROIDCard(s string) bool {
	if len(s) != 8 || !roIDCardSeries[s[:2]] {
		return false
	}
	_, ok := Digits(s[2:])
	return ok
}
