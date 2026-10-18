package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// withCheck finds the trailing digit(s) that make a synthetic value valid.
func withCheck(t *testing.T, name, body string, width int) string {
	t.Helper()
	d, _ := detectors.Get(name)
	for c := 0; c < 100; c++ {
		v := body + fmt.Sprintf("%0*d", width, c)
		if width == 2 {
			v = body[:2] + fmt.Sprintf("%02d", c) + body[2:]
		}
		if d.Validate(d.Normalize(v)) {
			return v
		}
	}
	t.Fatalf("no check digit for %s", name)
	return ""
}

func TestRealDetectorsOnTypicalLines(t *testing.T) {
	card := withCheck(t, "card", "453201511283036", 1)
	iban := withCheck(t, "iban", "RO"+"BTRL0000000000000001", 2)
	grouped := iban[:4] + " " + iban[4:8] + " " + iban[8:12] + " " + iban[12:16] + " " + iban[16:20] + " " + iban[20:]
	text := strings.Join([]string{
		"customer.iban = \"" + grouped + "\"",
		"card_number: " + card[:4] + " " + card[4:8] + " " + card[8:12] + " " + card[12:] + " 12/29",
		"order 4532015112830367 shipped",
		"timestamp 1792334309",
		"pan=" + card,
	}, "\n") + "\n"
	fs, err := ScanReader("app/config.yml", strings.NewReader(text), detectors.All())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.Confidence{}
	for _, f := range fs {
		got[fmt.Sprintf("%d:%s", f.Location.Line, f.Detector)] = f.Confidence
		if strings.Contains(f.Masked, card[:12]) {
			t.Fatal("mask leaked the card number")
		}
	}
	if got["1:iban"] != domain.High || got["2:card"] != domain.High || got["5:card"] != domain.High {
		t.Fatalf("got %v", got)
	}
	if _, ok := got["4:bg-egn"]; ok {
		t.Fatal("a timestamp was reported as an EGN")
	}
}

func TestVATAfterAnUppercaseWord(t *testing.T) {
	vat := withCheck(t, "eu-vat", "PL101000123", 1)
	fs, err := ScanReader("a.txt", strings.NewReader("VAT "+vat+"\n"), detectors.All())
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || fs[0].Detector != "eu-vat" {
		t.Fatalf("got %+v", fs)
	}
}
