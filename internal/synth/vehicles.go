package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

func init() {
	Register("driving-licence", func(r *rand.Rand) string {
		return licenceGens[r.Intn(len(licenceGens))](r)
	})
	Register("licence-plate", func(r *rand.Rand) string {
		return plateGens[r.Intn(len(plateGens))](r)
	})
}

const alnum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

var licenceGens = []Generator{
	// Germany: authority code, serial, base-36 weighted check mod 11, issue number.
	func(r *rand.Rand) string {
		body := Letters(r, 1, "ABCDEFGHJKLMNP") + Digits(r, 2) + Letters(r, 1, "0123456789") + Letters(r, 5, alnum)
		sum := 0
		for i := 0; i < 9; i++ {
			sum += strings.IndexByte(alnum, body[i]) * (9 - i)
		}
		check := fmt.Sprint(sum % 11)
		if sum%11 == 10 {
			check = "X"
		}
		return body + check + Digits(r, 1)
	},
	// Spain: DNI or NIE with its mod 23 letter.
	func(r *rand.Rand) string {
		n := r.Intn(100_000_000)
		if r.Intn(4) == 0 {
			lead := r.Intn(3)
			n = n % 10_000_000
			return string("XYZ"[lead]) + Pad(n, 7) + string("TRWAGMYFPDXBNJZSQVHLCKE"[(lead*10_000_000+n)%23])
		}
		return Pad(n, 8) + string("TRWAGMYFPDXBNJZSQVHLCKE"[n%23])
	},
	// Finland: DDMMYY, century sign, 002-899, mod 31 control character.
	func(r *rand.Rand) string {
		year, month, day := Date(r)
		sign := "-"
		if year >= 2000 {
			sign = "A"
		}
		ind := 2 + r.Intn(898)
		body := Pad(day, 2) + Pad(month, 2) + Pad(year%100, 2) + Pad(ind, 3)
		n := 0
		for _, c := range body {
			n = n*10 + int(c-'0')
		}
		return body[:6] + sign + body[6:] + string("0123456789ABCDEFHJKLMNPRSTUVWXY"[n%31])
	},
	// Sweden: YYMMDD-NNN plus a Luhn digit.
	func(r *rand.Rand) string {
		year, month, day := Date(r)
		body := Pad(year%100, 2) + Pad(month, 2) + Pad(day, 2) + Digits(r, 3)
		return body[:6] + "-" + body[6:] + fmt.Sprint(LuhnDigit(body))
	},
	func(r *rand.Rand) string { return "E" + Letters(r, 1, "") + Digits(r, 6) },
	func(r *rand.Rand) string { return "ET" + Digits(r, 6) },
	func(r *rand.Rand) string { return Letters(r, 2, "") + Digits(r, 6) },
	func(r *rand.Rand) string { return Letters(r, 3, "") + Digits(r, 6) },
	func(r *rand.Rand) string { return Digits(r, 6) + Letters(r, 4, "") },
	func(r *rand.Rand) string { return "U1" + Letters(r, 1, "") + Digits(r, 6) + Letters(r, 1, "") },
	func(r *rand.Rand) string { return Digits(r, 5) + "/" + Digits(r, 2) + "/" + Digits(r, 4) },
	func(r *rand.Rand) string { return Letters(r, 2, "") + "-" + Digits(r, 6) + " " + Digits(r, 1) },
	func(r *rand.Rand) string { return Letters(r, 1, "") + Digits(r, 8) },
}

const (
	frLetters = "ABCDEFGHJKLMNPQRSTVWXYZ"
	itLetters = "ABCDEFGHJKLMNPRSTVWXYZ"
	consonant = "BCDFGHJKLMNPRSTVWXYZ"
	plLetters = "ACEFGHJKLMNPRSTUVWXY"
)

var plateGens = []Generator{
	func(r *rand.Rand) string {
		if r.Intn(3) == 0 {
			return "B " + Digits(r, 3) + " " + Letters(r, 3, "ABCDEFGHJKLMNPRSTUVWXYZ")
		}
		counties := []string{"AB", "AG", "CJ", "IS", "TM", "BV", "CT", "PH"}
		return counties[r.Intn(len(counties))] + " " + Digits(r, 2) + " " + Letters(r, 3, "ABCDEFGHJKLMNPRSTUVWXYZ")
	},
	func(r *rand.Rand) string {
		return Letters(r, 1+r.Intn(3), "") + "-" + Letters(r, 1+r.Intn(2), "") + " " + fmt.Sprint(1+r.Intn(999))
	},
	func(r *rand.Rand) string {
		return Letters(r, 2, frLetters) + "-" + Digits(r, 3) + "-" + Letters(r, 2, frLetters)
	},
	func(r *rand.Rand) string {
		return Letters(r, 2, itLetters) + " " + Digits(r, 3) + " " + Letters(r, 2, itLetters)
	},
	func(r *rand.Rand) string { return Digits(r, 4) + " " + Letters(r, 3, consonant) },
	func(r *rand.Rand) string { return Letters(r, 2, "") + " " + Digits(r, 3) + Letters(r, 2, plLetters) },
	func(r *rand.Rand) string {
		return Letters(r, 2, "BDFGHJKLNPRSTVXZ") + "-" + Digits(r, 2) + "-" + Letters(r, 2, "BDFGHJKLNPRSTVXZ")
	},
	func(r *rand.Rand) string {
		return Letters(r, 3, "ABCDEFGHJKLMNPRST") + "-" + Pad(1+r.Intn(999), 3)
	},
	func(r *rand.Rand) string {
		return Letters(r, 2, "") + " " + Letters(r, 2, "") + "-" + Pad(1+r.Intn(999), 3)
	},
	func(r *rand.Rand) string {
		provinces := []string{"CA", "CB", "PB", "A", "B", "EH", "TX"}
		return provinces[r.Intn(len(provinces))] + " " + Digits(r, 4) + " " + Letters(r, 2, "ABEKMHPCTX")
	},
	func(r *rand.Rand) string {
		return fmt.Sprint(1+r.Intn(9)) + Letters(r, 1, "ABCEHJKLMPSTUZ") + Digits(r, 1) + " " + Digits(r, 4)
	},
	func(r *rand.Rand) string {
		return fmt.Sprint(1+r.Intn(9)) + "-" + Letters(r, 3, "") + "-" + Digits(r, 3)
	},
	func(r *rand.Rand) string {
		return Letters(r, 3, "ABCDEFGHJKLMNOPRSTUWXYZ") + " " + Digits(r, 2) + Letters(r, 1, "ABCDEFGHJKLMNPRSTUWXYZ")
	},
	func(r *rand.Rand) string { return Letters(r, 2, "") + " " + Digits(r, 2) + " " + Digits(r, 3) },
	func(r *rand.Rand) string { return Letters(r, 3, "") + "-" + Digits(r, 4) },
	func(r *rand.Rand) string {
		return fmt.Sprint(13+r.Intn(13)) + fmt.Sprint(1+r.Intn(2)) + "-" + []string{"D", "C", "G", "KY", "WX"}[r.Intn(5)] + "-" + fmt.Sprint(1+r.Intn(99999))
	},
}
