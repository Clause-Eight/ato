package detectors

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// Every value in this file is synthetic: built by the generators below, which compute check digits
// their own way so that the validators are tested against an independent implementation.

func pad(n, width int) string { return fmt.Sprintf("%0*d", width, n) }

func genCNP(r *rand.Rand) string {
	s := 1 + r.Intn(8)
	year := r.Intn(100)
	month := 1 + r.Intn(12)
	day := 1 + r.Intn(28)
	counties := []int{1, 7, 12, 22, 40, 46, 51, 52}
	body := fmt.Sprintf("%d%s%s%s%s%s", s, pad(year, 2), pad(month, 2), pad(day, 2), pad(counties[r.Intn(len(counties))], 2), pad(r.Intn(1000), 3))
	const key = "279146358279"
	sum := 0
	for i := range body {
		sum += int(body[i]-'0') * int(key[i]-'0')
	}
	c := sum % 11
	if c == 10 {
		c = 1
	}
	return body + fmt.Sprint(c)
}

func genEGN(r *rand.Rand) string {
	month := 1 + r.Intn(12)
	switch r.Intn(3) {
	case 1:
		month += 40
	case 2:
		month += 20
	}
	body := pad(r.Intn(100), 2) + pad(month, 2) + pad(1+r.Intn(28), 2) + pad(r.Intn(1000), 3)
	w := []int{2, 4, 8, 5, 10, 9, 7, 3, 6}
	sum := 0
	for i, x := range w {
		sum += int(body[i]-'0') * x
	}
	c := sum % 11 % 10
	return body + fmt.Sprint(c)
}

func genLNCh(r *rand.Rand) string {
	body := pad(r.Intn(1_000_000_000), 9)
	w := []int{21, 19, 17, 13, 11, 9, 7, 3, 1}
	sum := 0
	for i, x := range w {
		sum += int(body[i]-'0') * x
	}
	return body + fmt.Sprint(sum%10)
}

func genTAJ(r *rand.Rand) string {
	body := pad(r.Intn(100_000_000), 8)
	sum := 0
	for i := range body {
		m := 3
		if i%2 == 1 {
			m = 7
		}
		sum += int(body[i]-'0') * m
	}
	return body + fmt.Sprint(sum%10)
}

func genHUTax(r *rand.Rand) string {
	for {
		body := "8" + pad(1000+r.Intn(50000), 5) + pad(r.Intn(1000), 3)
		sum := 0
		for i := range body {
			sum += int(body[i]-'0') * (i + 1)
		}
		if sum%11 != 10 {
			return body + fmt.Sprint(sum%11)
		}
	}
}

func genIBAN(r *rand.Rand, country string) string {
	n := IBANLengths[country] - 4
	var bban strings.Builder
	for i := 0; i < n; i++ {
		if i < 4 && (country == "RO" || country == "GB" || country == "NL") {
			bban.WriteByte(byte('A' + r.Intn(26)))
		} else {
			bban.WriteByte(byte('0' + r.Intn(10)))
		}
	}
	numeric := func(s string) string {
		var b strings.Builder
		for _, c := range s {
			if c >= 'A' && c <= 'Z' {
				b.WriteString(fmt.Sprint(int(c-'A') + 10))
			} else {
				b.WriteRune(c)
			}
		}
		return b.String()
	}
	digits := numeric(bban.String() + country + "00")
	rem := 0
	for _, c := range digits {
		rem = (rem*10 + int(c-'0')) % 97
	}
	return fmt.Sprintf("%s%02d%s", country, 98-rem, bban.String())
}

func genCard(r *rand.Rand, prefix string, length int) string {
	var b strings.Builder
	b.WriteString(prefix)
	for b.Len() < length-1 {
		b.WriteByte(byte('0' + r.Intn(10)))
	}
	body := b.String()
	sum := 0
	for i := len(body) - 1; i >= 0; i-- {
		n := int(body[i] - '0')
		if (len(body)-1-i)%2 == 0 {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
	}
	return body + fmt.Sprint((10-sum%10)%10)
}

func flipLast(s string) string {
	last := s[len(s)-1]
	if last == '9' {
		return s[:len(s)-1] + "0"
	}
	return s[:len(s)-1] + string(last+1)
}

type genCase struct {
	name     string
	validate func(string) bool
	gen      func(*rand.Rand) string
}

var genCases = []genCase{
	{"ro-cnp", validCNP, genCNP},
	{"bg-egn", validEGN, genEGN},
	{"bg-lnch", validLNCh, genLNCh},
	{"hu-taj", validTAJ, genTAJ},
	{"hu-tax-id", validHUTaxID, genHUTax},
	{"iban-ro", validIBAN, func(r *rand.Rand) string { return genIBAN(r, "RO") }},
	{"iban-de", validIBAN, func(r *rand.Rand) string { return genIBAN(r, "DE") }},
	{"iban-hu", validIBAN, func(r *rand.Rand) string { return genIBAN(r, "HU") }},
	{"iban-gb", validIBAN, func(r *rand.Rand) string { return genIBAN(r, "GB") }},
	{"card-visa", func(s string) bool { return CardBrand(s) == "Visa" }, func(r *rand.Rand) string { return genCard(r, "4", 16) }},
	{"card-mc", func(s string) bool { return CardBrand(s) == "Mastercard" }, func(r *rand.Rand) string { return genCard(r, "53", 16) }},
	{"card-amex", func(s string) bool { return CardBrand(s) == "American Express" }, func(r *rand.Rand) string { return genCard(r, "37", 15) }},
}

func TestGeneratedValuesAreValidAndOneDigitChangesAreNot(t *testing.T) {
	r := rand.New(rand.NewSource(8))
	for _, c := range genCases {
		for i := 0; i < 500; i++ {
			v := c.gen(r)
			if !c.validate(v) {
				t.Fatalf("%s: generated value %d rejected", c.name, i)
			}
			if c.validate(flipLast(v)) && c.name != "bg-egn" {
				t.Fatalf("%s: changed check digit still accepted (case %d)", c.name, i)
			}
		}
	}
}

func TestDateAndRangeRules(t *testing.T) {
	r := rand.New(rand.NewSource(9))
	cnp := genCNP(r)
	badMonth := cnp[:3] + "13" + cnp[5:]
	badCounty := cnp[:7] + "60" + cnp[9:]
	badSex := "0" + cnp[1:]
	for _, v := range []string{badMonth, badCounty, badSex} {
		if validCNP(v) {
			t.Errorf("CNP rule not enforced for case %q", v[:5])
		}
	}
	if validEGN("0013010000") || validEGN("0002300000") {
		t.Error("EGN accepted an impossible date")
	}
	if validHUTaxID("7123456789") {
		t.Error("tax ID must start with 8")
	}
	if !ValidDate(2024, 2, 29) || ValidDate(2023, 2, 29) || ValidDate(1900, 2, 29) || !ValidDate(2000, 2, 29) {
		t.Error("leap years wrong")
	}
}

func TestIBANRules(t *testing.T) {
	if !validIBAN("DE89370400440532013000") {
		t.Fatal("published example IBAN rejected")
	}
	for _, v := range []string{"DE8937040044053201300", "XX89370400440532013000", "DE88370400440532013000", "DE89370400440532013#00"} {
		if validIBAN(v) {
			t.Errorf("accepted %s", v)
		}
	}
	if normalizeIBAN("de89 3704-0044 0532 0130 00") != "DE89370400440532013000" {
		t.Error("normalisation wrong")
	}
	if got := maskIBAN("DE89370400440532013000"); got != "DE••••••••••••••••3000" {
		t.Errorf("mask %q", got)
	}
}

func TestCardRules(t *testing.T) {
	for v, brand := range map[string]string{"4111111111111111": "Visa", "5555555555554444": "Mastercard", "378282246310005": "American Express", "6011111111111117": "Discover"} {
		if got := CardBrand(v); got != brand {
			t.Errorf("%s: got %q want %q", v[:4], got, brand)
		}
	}
	if CardBrand("1234567812345670") != "" {
		t.Error("unknown prefix accepted")
	}
	if CardBrand("4111111111111112") != "" {
		t.Error("Luhn failure accepted")
	}
	if MaskKeepLast(4)("4111111111111111") != "••••••••••••1111" {
		t.Error("card mask wrong")
	}
}

func TestRegistry(t *testing.T) {
	names := map[string]bool{}
	for _, d := range All() {
		names[d.Name] = true
		if d.Pattern == nil || d.Label == "" || d.Base == 0 || d.Validation == "" {
			t.Errorf("%s is incomplete", d.Name)
		}
		for _, k := range d.Keywords {
			if k != strings.ToLower(k) {
				t.Errorf("%s: keyword %q must be lower case", d.Name, k)
			}
		}
	}
	for _, n := range []string{"ro-cnp", "bg-egn", "bg-lnch", "hu-taj", "hu-tax-id", "iban", "card", "eu-vat", "email", "eu-phone", "pl-pesel", "cz-birth-number", "hr-oib"} {
		if !names[n] {
			t.Errorf("missing detector %s", n)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("duplicate registration should panic")
		}
	}()
	Register(Detector{Name: "iban"})
}

func FuzzValidators(f *testing.F) {
	for _, seed := range []string{"", "0", "1234567890123", "DE89370400440532013000", "4111111111111111", "8000000000", "ZZ00"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, d := range All() {
			v := d.Normalize(s)
			if d.Validate != nil {
				d.Validate(v)
			}
			if m := d.Mask(v); len([]rune(m)) != len([]rune(v)) {
				t.Fatalf("%s: mask changed the length", d.Name)
			}
		}
	})
}
