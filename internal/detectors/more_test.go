package detectors

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func genPESEL(r *rand.Rand) string {
	year, month := r.Intn(100), 1+r.Intn(12)
	if r.Intn(2) == 1 {
		month += 20
	}
	body := pad(year, 2) + pad(month, 2) + pad(1+r.Intn(28), 2) + pad(r.Intn(10000), 4)
	w := []int{1, 3, 7, 9, 1, 3, 7, 9, 1, 3}
	sum := 0
	for i, x := range w {
		sum += int(body[i]-'0') * x
	}
	return body + fmt.Sprint((10-sum%10)%10)
}

func genBirthNumber(r *rand.Rand) string {
	for {
		month := 1 + r.Intn(12)
		if r.Intn(2) == 1 {
			month += 50
		}
		body := pad(54+r.Intn(46), 2) + pad(month, 2) + pad(1+r.Intn(28), 2) + pad(r.Intn(1000), 3)
		var n int64
		fmt.Sscan(body, &n)
		c := n * 10 % 11
		if c == 0 {
			return body + "0"
		}
		if 11-c < 10 {
			return body + fmt.Sprint(11-c)
		}
	}
}

func genMod1110(r *rand.Rand, n int) string {
	body := pad(r.Intn(1_000_000_000), 9)
	for len(body) < n-1 {
		body += fmt.Sprint(r.Intn(10))
	}
	body = body[:n-1]
	product := 10
	for _, c := range body {
		sum := (int(c-'0') + product) % 10
		if sum == 0 {
			sum = 10
		}
		product = sum * 2 % 11
	}
	return body + fmt.Sprint((11-product)%10)
}

func genVATRO(r *rand.Rand) string {
	body := pad(1+r.Intn(99_999_999), 8)
	padded := "0" + body
	w := []int{7, 5, 3, 2, 1, 7, 5, 3, 2}
	sum := 0
	for i, x := range w {
		sum += int(padded[i]-'0') * x
	}
	return "RO" + strings.TrimLeft(body, "0") + fmt.Sprint(sum*10%11%10)
}

func genVATPL(r *rand.Rand) string {
	for {
		body := pad(r.Intn(1_000_000_000), 9)
		w := []int{6, 5, 7, 2, 3, 4, 5, 6, 7}
		sum := 0
		for i, x := range w {
			sum += int(body[i]-'0') * x
		}
		if sum%11 != 10 {
			return "PL" + body + fmt.Sprint(sum%11)
		}
	}
}

func genVATHU(r *rand.Rand) string {
	body := pad(r.Intn(10_000_000), 7)
	w := []int{9, 7, 3, 1, 9, 7, 3}
	sum := 0
	for i, x := range w {
		sum += int(body[i]-'0') * x
	}
	return "HU" + body + fmt.Sprint((10-sum%10)%10)
}

func TestMoreGeneratedValues(t *testing.T) {
	r := rand.New(rand.NewSource(10))
	cases := []genCase{
		{"pl-pesel", validPESEL, genPESEL},
		{"cz-birth-number", validBirthNumber, genBirthNumber},
		{"hr-oib", func(s string) bool { return mod11_10(s) }, func(r *rand.Rand) string { return genMod1110(r, 11) }},
		{"vat-de", validVAT, func(r *rand.Rand) string { return "DE" + genMod1110(r, 9) }},
		{"vat-ro", validVAT, genVATRO},
		{"vat-pl", validVAT, genVATPL},
		{"vat-hu", validVAT, genVATHU},
	}
	for _, c := range cases {
		for i := 0; i < 300; i++ {
			v := c.gen(r)
			if !c.validate(v) {
				t.Fatalf("%s: generated value %d rejected", c.name, i)
			}
			if c.name != "cz-birth-number" && c.validate(flipLast(v)) {
				t.Fatalf("%s: changed check digit accepted (case %d)", c.name, i)
			}
		}
	}
}

func TestVATFormats(t *testing.T) {
	for _, v := range []string{"ATU12345678", "NL123456789B01", "FRXX123456789", "SE123456789001", "IE1234567T"} {
		if !validVAT(v) {
			t.Errorf("format-only VAT %s rejected", v[:2])
		}
	}
	for _, v := range []string{"ATX12345678", "NL123456789", "SE123456789002", "ZZ123456789", "DE12"} {
		if validVAT(v) {
			t.Errorf("accepted %s", v)
		}
	}
}

func TestContactData(t *testing.T) {
	for v, want := range map[string]bool{
		"ana.pop@firma-test-ro.eu": true, "x@example.com": false, "a@sub.example.org": false, "b@service.test": false, "a..b@mail.eu": false,
	} {
		if validEmail(v) != want {
			t.Errorf("email %q: want %v", v, want)
		}
	}
	if maskEmail("ana.pop@firma.eu") != "•••••••@firma.eu" {
		t.Errorf("email mask %q", maskEmail("ana.pop@firma.eu"))
	}
	for v, want := range map[string]bool{
		"+40712345678": true, "0712345678": true, "+359888123456": true, "+36301234567": true, "+48501234567": true,
		"+4071234567": false, "0612345678": false, "+49151234567": false,
	} {
		if validPhone(normalizePhone(v)) != want {
			t.Errorf("phone %q: want %v", v, want)
		}
	}
}
