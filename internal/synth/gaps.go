package synth

import (
	"fmt"
	"math/big"
	"math/rand"
	"slices"
	"strings"
)

// Generators for the detectors that closed the coverage gaps of 2026-10-31: each computes its check digits
// without calling the detector.
func init() {
	Register("hu-personal-id", func(r *rand.Rand) string {
		for {
			year, month, day := Date(r)
			sex := r.Intn(2)
			first := 1 + sex
			if year >= 2000 {
				first = 3 + sex
			}
			body := fmt.Sprint(first) + Pad(year%100, 2) + Pad(month, 2) + Pad(day, 2) + Digits(r, 3)
			weights := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
			if year >= 1997 {
				weights = []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
			}
			if c := WeightedSum(body, weights) % 11; c < 10 {
				return body + fmt.Sprint(c)
			}
		}
	})
	Register("pt-niss", func(r *rand.Rand) string {
		body := "1" + Digits(r, 9)
		return body + fmt.Sprint(9-WeightedSum(body, []int{29, 23, 19, 17, 13, 11, 7, 5, 3, 2})%10)
	})
	Register("de-kvnr", func(r *rand.Rand) string {
		letter := Letters(r, 1, "")
		digits := Digits(r, 8)
		all := Pad(int(letter[0]-'A')+1, 2) + digits
		sum := 0
		for i, c := range all {
			p := int(c-'0') * []int{1, 2}[i%2]
			sum += p%10 + p/10
		}
		return letter + digits + fmt.Sprint(sum%10)
	})
	Register("gb-utr", func(r *rand.Rand) string {
		body := Digits(r, 9)
		v := string("21987654321"[WeightedSum(body, []int{6, 7, 8, 9, 10, 5, 4, 3, 2})%11]) + body
		if r.Intn(3) == 0 {
			return v[:5] + " " + v[5:]
		}
		return v
	})
	Register("gb-upn", func(r *rand.Rand) string {
		const alphabet = "ABCDEFGHJKLMNPQRTUVWXYZ0123456789"
		las := []string{"201", "302", "330", "800", "873", "916", "938"}
		rest := las[r.Intn(len(las))] + Digits(r, 4) + Pad(10+r.Intn(15), 2) + Digits(r, 2)
		if r.Intn(4) == 0 {
			rest += Letters(r, 1, "ABCDEFGHJKLMNPQRTUVWXYZ")
		} else {
			rest += Digits(r, 1)
		}
		sum := 0
		for i := range rest {
			sum += strings.IndexByte(alphabet, rest[i]) * (i + 2)
		}
		return string(alphabet[sum%23]) + rest
	})
	Register("nl-onderwijsnummer", func(r *rand.Rand) string {
		for {
			body := "10" + Digits(r, 6)
			if c := (WeightedSum(body, []int{9, 8, 7, 6, 5, 4, 3, 2}) - 5) % 11; c < 10 {
				return body + fmt.Sprint(c)
			}
		}
	})
	Register("cz-bank-account", func(r *rand.Rand) string {
		banks := []string{"0100", "0300", "0600", "0800", "2010", "3030", "5500", "6210"}
		part := func(n int) string {
			for {
				weights := []int{6, 3, 7, 9, 10, 5, 8, 4, 2}[10-n:]
				body := fmt.Sprint(1+r.Intn(9)) + Digits(r, n-2)
				if c := (11 - WeightedSum(body, weights)%11) % 11; c < 10 {
					return body + fmt.Sprint(c)
				}
			}
		}
		v := part(10) + "/" + banks[r.Intn(len(banks))]
		if r.Intn(3) == 0 {
			v = part(6) + "-" + v
		}
		return v
	})
	Register("es-ccc", func(r *rand.Rand) string {
		digit := func(s string) string {
			sum := WeightedSum(s, []int{1, 2, 4, 8, 5, 10, 9, 7, 3, 6})
			c := 11 - sum%11
			if c == 11 {
				c = 0
			} else if c == 10 {
				c = 1
			}
			return fmt.Sprint(c)
		}
		bank, branch, account := Digits(r, 4), Digits(r, 4), Digits(r, 10)
		check := digit("00"+bank+branch) + digit(account)
		if r.Intn(3) == 0 {
			return bank + " " + branch + " " + check + " " + account
		}
		return bank + branch + check + account
	})
	Register("no-kontonr", func(r *rand.Rand) string {
		for {
			body := Digits(r, 10)
			c := (11 - WeightedSum(body, []int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2})%11) % 11
			if c == 10 {
				continue
			}
			v := body + fmt.Sprint(c)
			if r.Intn(2) == 0 {
				return v[:4] + "." + v[4:6] + "." + v[6:]
			}
			return v
		}
	})
	Register("pl-nrb", func(r *rand.Rand) string {
		bban := Digits(r, 24)
		v := Pad(98-Mod97(bban+LettersAsNumbers("PL")+"00"), 2) + bban
		if r.Intn(2) == 0 {
			parts := []string{v[:2]}
			for i := 2; i < 26; i += 4 {
				parts = append(parts, v[i:i+4])
			}
			return strings.Join(parts, " ")
		}
		return v
	})
	Register("rf-creditor-reference", func(r *rand.Rand) string {
		ref := Digits(r, 4+r.Intn(14))
		if r.Intn(3) == 0 {
			ref = Letters(r, 2, "") + ref
		}
		v := "RF" + Pad(98-Mod97(LettersAsNumbers(ref+"RF")+"00"), 2) + ref
		if r.Intn(2) == 0 {
			var groups []string
			for i := 0; i < len(v); i += 4 {
				groups = append(groups, v[i:min(i+4, len(v))])
			}
			return strings.Join(groups, " ")
		}
		return v
	})
	Register("ch-qr-reference", func(r *rand.Rand) string {
		table := []int{0, 9, 4, 6, 8, 2, 7, 1, 3, 5}
		body := Digits(r, 26)
		c := 0
		for _, x := range body {
			c = table[(c+int(x-'0'))%10]
		}
		v := body + fmt.Sprint((10-c)%10)
		if r.Intn(2) == 0 {
			return v[:2] + " " + v[2:7] + " " + v[7:12] + " " + v[12:17] + " " + v[17:22] + " " + v[22:]
		}
		return v
	})
	Register("iccid", func(r *rand.Rand) string {
		codes := []string{"40", "359", "36", "49", "33", "34", "39", "48", "420", "31", "44"}
		body := "89" + codes[r.Intn(len(codes))]
		body += Digits(r, 18+r.Intn(2)-len(body))
		return body + fmt.Sprint(LuhnDigit(body))
	})
	Register("es-cups", func(r *rand.Rand) string {
		const letters = "TRWAGMYFPDXBNJZSQVHLCKE"
		digits := []string{"0021", "0022", "0031", "0026"}[r.Intn(4)] + Digits(r, 12)
		n, _ := new(big.Int).SetString(digits, 10)
		m := int(new(big.Int).Mod(n, big.NewInt(529)).Int64())
		v := "ES" + digits + string(letters[m/23]) + string(letters[m%23])
		// The border point suffix is rare and not covered by the check letters.
		if r.Intn(20) == 0 {
			v += fmt.Sprint(r.Intn(10)) + Letters(r, 1, "FPRCXYZ")
		}
		return v
	})
	Register("es-cadastral-reference", func(r *rand.Rand) string {
		alphabet := []rune("ABCDEFGHIJKLMNÑOPQRSTUVWXYZ")
		weights := []int{13, 15, 12, 5, 4, 17, 9, 21, 3, 7, 1}
		check := func(part string) string {
			sum := 0
			for i, c := range []rune(part) {
				if c >= '0' && c <= '9' {
					sum += weights[i] * int(c-'0')
				} else {
					sum += weights[i] * (slices.Index(alphabet, c) + 1)
				}
			}
			return string("MQWERTYUIOPASDFGHJKLBZX"[sum%23])
		}
		var a, b string
		if r.Intn(3) == 0 {
			a = Digits(r, 5) + Letters(r, 1, "ABCDEFGHJ") + Digits(r, 1)
			b = Digits(r, 7)
		} else {
			a = Digits(r, 7)
			b = Letters(r, 2, "BCDFGHJKLMNPQRSTVWXYZ") + Digits(r, 4) + Letters(r, 1, "ABCDEFGHJKLMNPQRSTUVWXYZ")
		}
		property := Pad(1+r.Intn(60), 4)
		return a + b + property + check(a+property) + check(b+property)
	})
}
