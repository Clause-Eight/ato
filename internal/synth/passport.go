package synth

import (
	"fmt"
	"math/rand"
)

func init() {
	Register("passport", func(r *rand.Rand) string {
		layouts := []func() string{
			func() string { return Letters(r, 1, "") + " " + Digits(r, 7) },                // AT with its space
			func() string { return Letters(r, 1, "") + Digits(r, 7) },                      // AT, EE, SK
			func() string { return Letters(r, 1, "") + Digits(r, 6) },                      // PT
			func() string { return Letters(r, 1, "") + Digits(r, 6+r.Intn(3)) },            // CY
			func() string { return Letters(r, 2, "") + Digits(r, 6) },                      // BE
			func() string { return Letters(r, 2, "") + Digits(r, 7) },                      // FI, GR, PL
			func() string { return "P" + Letters(r, 1, "") + Digits(r, 7) },                // SI
			func() string { return Letters(r, 2, alnum) + Digits(r, 7) },                   // IE, IT, LV
			func() string { return Letters(r, 3, "") + Digits(r, 6) },                      // ES
			func() string { return Digits(r, 2) + Letters(r, 2, "") + Digits(r, 5) },       // FR
			func() string { return Digits(r, 9) },                                          // BG, DK, HR, RO, GB
			func() string { return Digits(r, 8) },                                          // CZ, SE, RO
			func() string { return Digits(r, 7) },                                          // MT
			func() string { return "X" + Digits(r, 7) },                                    // CH
			func() string { return Letters(r, 2, nlLetters) + nlMiddle(r) + Digits(r, 1) }, // NL
			func() string { return dePassportSerial(r) },                                   // DE
			func() string { s := dePassportSerial(r); return s + fmt.Sprint(mrzDigit(s)) }, // DE with check digit
		}
		return layouts[r.Intn(len(layouts))]()
	})
}

// nlLetters are the capitals of Dutch document numbers: every letter but O.
const nlLetters = "ABCDEFGHIJKLMNPQRSTUVWXYZ"

// nlMiddle is the six letters or digits in the middle of a Dutch document number, with at least one digit so that
// the value never reads as a word.
func nlMiddle(r *rand.Rand) string {
	return Letters(r, 3, nlLetters+"0123456789") + Digits(r, 3)
}

// dePassportSerial is a German passport serial: C F G H J K, then eight characters of the document alphabet, at
// least one of them a digit.
func dePassportSerial(r *rand.Rand) string {
	return Letters(r, 1, "CFGHJK") + Letters(r, 7, "0123456789CFGHJKLMNPRTVWXYZ") + Digits(r, 1)
}
