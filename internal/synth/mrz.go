package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

func init() {
	// The line of a passport or ID card MRZ that carries the dates and check digits (ICAO Doc 9303), for the
	// fictional state Utopia. Mostly passports: the ID card line ends in a composite check digit that also
	// covers the line above, so a changed last digit there cannot be caught on its own.
	Register("mrz", func(r *rand.Rand) string {
		if r.Intn(10) == 0 {
			return td1Middle(r)
		}
		return td3Lower(r)
	})
}

// td3Lower is the lower line of a TD3 passport MRZ: 44 characters.
func td3Lower(r *rand.Rand) string {
	number := Letters(r, 2, "") + Digits(r, 7)
	if r.Intn(4) == 0 {
		number = Letters(r, 1, "") + Digits(r, 7) + "<"
	}
	optional := strings.Repeat("<", 14)
	optionalCheck := "<"
	switch r.Intn(3) {
	case 0:
		optional = Digits(r, 9) + strings.Repeat("<", 5)
		optionalCheck = icaoCheck(optional)
	case 1:
		optionalCheck = "0"
	}
	docPart := number + icaoCheck(number)
	birth := yymmdd(r, false)
	birthPart := birth + icaoCheck(birth)
	expiry := yymmdd(r, true)
	expiryPart := expiry + icaoCheck(expiry)
	rest := optional + optionalCheck
	composite := icaoCheck(docPart + birthPart + expiryPart + rest)
	return docPart + "UTO" + birthPart + sex(r) + expiryPart + rest + composite
}

// td1Middle is the middle line of a TD1 ID card MRZ: 30 characters. The composite check digit covers the upper
// line too, so it is computed over a made-up upper line (I<UTO, document number and check, filler).
func td1Middle(r *rand.Rand) string {
	number := Letters(r, 1, "") + Digits(r, 8)
	upperData := number + icaoCheck(number) + strings.Repeat("<", 15)
	birth := yymmdd(r, false)
	expiry := yymmdd(r, true)
	optional := strings.Repeat("<", 11)
	birthPart, expiryPart := birth+icaoCheck(birth), expiry+icaoCheck(expiry)
	composite := icaoCheck(upperData + birthPart + expiryPart + optional)
	return birthPart + sex(r) + expiryPart + "UTO" + optional + composite
}

func sex(r *rand.Rand) string { return string("MF<"[r.Intn(3)]) }

// yymmdd is a birth date, or an expiry date between 2020 and 2039.
func yymmdd(r *rand.Rand, expiry bool) string {
	year, month, day := Date(r)
	if expiry {
		year = 2020 + r.Intn(20)
	}
	return Pad(year%100, 2) + Pad(month, 2) + Pad(day, 2)
}

// icaoCheck is the ICAO 9303 check digit: values 0-9, A-Z as 10-35, < as 0, weighted 7, 3, 1, mod 10.
func icaoCheck(s string) string {
	weights := []int{7, 3, 1}
	sum := 0
	for i, c := range s {
		v := 0
		switch {
		case c >= '0' && c <= '9':
			v = int(c - '0')
		case c >= 'A' && c <= 'Z':
			v = int(c-'A') + 10
		}
		sum += v * weights[i%3]
	}
	return fmt.Sprint(sum % 10)
}
