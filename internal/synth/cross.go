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
	Register("eu-phone", phone)
}

// phoneRanges are number ranges from the member states' numbering plans: country code, leading digits of the
// national significant number and its length.
var phoneRanges = []struct {
	cc, lead string
	length   int
}{
	{"40", "7", 9}, {"40", "21", 9}, {"359", "88", 9}, {"359", "2", 8}, {"36", "30", 9}, {"36", "1", 8},
	{"48", "50", 9}, {"48", "22", 9}, {"49", "151", 11}, {"49", "171", 10}, {"49", "30", 10}, {"43", "664", 10},
	{"43", "1", 8}, {"32", "47", 9}, {"32", "2", 8}, {"385", "91", 9}, {"385", "1", 8}, {"357", "99", 8},
	{"357", "22", 8}, {"420", "60", 9}, {"420", "2", 9}, {"45", "2", 8}, {"372", "5", 8}, {"372", "6", 7},
	{"358", "40", 9}, {"358", "9", 8}, {"33", "6", 9}, {"33", "1", 9}, {"30", "69", 10}, {"30", "21", 10},
	{"353", "87", 9}, {"353", "1", 8}, {"39", "3", 10}, {"39", "06", 10}, {"371", "2", 8}, {"370", "6", 8},
	{"352", "621", 9}, {"356", "79", 8}, {"356", "21", 8}, {"31", "62", 9}, {"31", "20", 9}, {"351", "91", 9},
	{"351", "21", 9}, {"421", "90", 9}, {"421", "2", 9}, {"386", "41", 8}, {"386", "1", 8}, {"34", "6", 9},
	{"34", "91", 9}, {"46", "70", 9}, {"46", "8", 9},
}

// phone writes a number in international form, grouped the way it is usually printed, or now and then as a
// Romanian or Bulgarian mobile in national form.
func phone(r *rand.Rand) string {
	if r.Intn(8) == 0 {
		lead := []string{"07", "087", "088", "089"}[r.Intn(4)]
		n := lead + Digits(r, 10-len(lead))
		return n[:4] + " " + n[4:7] + " " + n[7:]
	}
	p := phoneRanges[r.Intn(len(phoneRanges))]
	nsn := p.lead + Digits(r, p.length-len(p.lead))
	prefix := "+"
	if r.Intn(5) == 0 {
		prefix = "00"
	}
	switch r.Intn(4) {
	case 0:
		return prefix + p.cc + nsn
	case 1:
		return prefix + p.cc + " " + strings.Join(groups(nsn, 3), " ")
	case 2:
		return prefix + p.cc + "-" + strings.Join(groups(nsn, 3), "-")
	default:
		trunk := ""
		if p.cc != "39" && r.Intn(2) == 0 {
			trunk = " (0)"
		}
		return prefix + p.cc + trunk + " " + nsn[:len(p.lead)] + " " + strings.Join(groups(nsn[len(p.lead):], 2), " ")
	}
}

// groups splits s into groups of n characters, the last one shorter.
func groups(s string, n int) []string {
	var out []string
	for i := 0; i < len(s); i += n {
		out = append(out, s[i:min(i+n, len(s))])
	}
	return out
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
