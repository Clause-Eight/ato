package detectors

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/Clause-Eight/ato/v2/internal/synth"
)

func TestEmailTestDomainsMatchOnALabelBoundary(t *testing.T) {
	for _, v := range []string{"ana@crowbar.com", "ana@mycompany.com", "ana@latest.com", "ana@protest.com"} {
		if !validEmail(v) {
			t.Errorf("%s is a real domain that only ends like a test domain, and was dropped", v[4:])
		}
	}
	for _, v := range []string{"ana@bar.com", "ana@mail.example.com", "ana@host.test", "ana@company.com"} {
		if validEmail(v) {
			t.Errorf("%s is a documentation or placeholder domain and was reported", v[4:])
		}
	}
}

// cnp completes twelve digits with the CNP check digit.
func cnp(body string) string {
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

func TestCNPResidentsMayBeBornInEitherCentury(t *testing.T) {
	// 29 February exists in 2000 but not in 1900, so a resident (7) born then is only valid in the 2000s.
	if v := cnp("700022940123"); !validCNP(v) {
		t.Error("resident born 29 February 2000 rejected")
	}
	if v := cnp("100022940123"); validCNP(v) {
		t.Error("29 February 1900 accepted")
	}
}

func TestBirthDatesInTheFutureAreRejected(t *testing.T) {
	next := time.Now().AddDate(1, 0, 0).Year() % 100
	if v := cnp(fmt.Sprintf("5%02d011540123", next)); validCNP(v) {
		t.Error("CNP born next year accepted")
	}
	body := fmt.Sprintf("%02d2115%04d", next, 1234)
	w := []int{1, 3, 7, 9, 1, 3, 7, 9, 1, 3}
	sum := 0
	for i, x := range w {
		sum += int(body[i]-'0') * x
	}
	if v := body + fmt.Sprint((10-sum%10)%10); validPESEL(v) {
		t.Error("PESEL born next year accepted")
	}
	if ValidBirthDate(time.Now().Year()+1, 1, 1) || !ValidBirthDate(2000, 2, 29) {
		t.Error("ValidBirthDate is wrong about next year or 29 February 2000")
	}
}

func TestFiscalCodePlaceNumberIsNeverZero(t *testing.T) {
	gen, _ := synth.For("it-fiscal-code")
	r := rand.New(rand.NewSource(3))
	for i := 0; i < 50; i++ {
		v := []byte(gen(r))
		v[11], v[12], v[13], v[14] = 'Z', '0', '5', '0'
		sum := 0
		for j := 0; j < 15; j++ {
			x := int(v[j] - 'A')
			if v[j] >= '0' && v[j] <= '9' {
				x = int(v[j] - '0')
			}
			if j%2 == 0 {
				x = fiscalOdd[x]
			}
			sum += x
		}
		v[15] = byte('A' + sum%26)
		if validFiscalCode(string(v)) {
			t.Fatalf("foreign place code Z050 accepted (case %d)", i)
		}
	}
}

func TestKeywordOnlyAfterWeakChecks(t *testing.T) {
	for _, name := range []string{"bg-lnch", "pl-id-card"} {
		if d, _ := Get(name); !d.NeedsKeyword {
			t.Errorf("%s has a single check digit and must need a keyword", name)
		}
	}
}
