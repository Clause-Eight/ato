package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

func init() {
	Register("iban", func(r *rand.Rand) string {
		country, length := pick(r, map[string]int{"DE": 22, "RO": 24, "HU": 28, "PL": 28, "FR": 27, "NL": 18, "ES": 24, "IT": 27})
		bban := Digits(r, length-4)
		switch country {
		case "RO", "NL":
			bban = Letters(r, 4, "") + bban[4:]
		case "IT":
			bban = Letters(r, 1, "") + bban[1:]
		}
		check := 98 - Mod97(LettersAsNumbers(bban+country)+"00")
		iban := fmt.Sprintf("%s%02d%s", country, check, bban)
		if r.Intn(2) == 0 {
			return iban
		}
		var groups []string
		for i := 0; i < len(iban); i += 4 {
			groups = append(groups, iban[i:min(i+4, len(iban))])
		}
		return strings.Join(groups, " ")
	})
	Register("card", func(r *rand.Rand) string {
		prefixes := []struct {
			prefix string
			length int
		}{{"4", 16}, {"51", 16}, {"55", 16}, {"2221", 16}, {"34", 15}, {"37", 15}}
		p := prefixes[r.Intn(len(prefixes))]
		body := p.prefix + Digits(r, p.length-1-len(p.prefix))
		return body + fmt.Sprint(LuhnDigit(body))
	})
	Register("eu-vat", func(r *rand.Rand) string {
		if r.Intn(2) == 0 {
			body := "1" + Digits(r, 7)
			return "DE" + body + fmt.Sprint(Mod1110Digit(body))
		}
		for {
			body := Digits(r, 9)
			if c := WeightedSum(body, []int{6, 5, 7, 2, 3, 4, 5, 6, 7}) % 11; c != 10 {
				return "PL" + body + fmt.Sprint(c)
			}
		}
	})
	Register("email", func(r *rand.Rand) string {
		return fmt.Sprintf("%s.%s%d@%s.eu", Letters(r, 5, "abcdefghij"), Letters(r, 4, "klmnoprst"), r.Intn(100), Letters(r, 6, "abcdefgh"))
	})
	Register("eu-phone", func(r *rand.Rand) string {
		return "+40 7" + Digits(r, 2) + " " + Digits(r, 3) + " " + Digits(r, 3)
	})
}

func pick(r *rand.Rand, m map[string]int) (string, int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	k := keys[r.Intn(len(keys))]
	return k, m[k]
}
