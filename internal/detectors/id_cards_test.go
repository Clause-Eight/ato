package detectors

import (
	"fmt"
	"testing"
)

func TestIDCardsNeedAKeyword(t *testing.T) {
	for _, name := range []string{"fr-id-card", "nl-id-card", "be-id-card", "hu-id-card", "gr-id-card", "bg-id-card"} {
		if d, _ := Get(name); !d.NeedsKeyword {
			t.Errorf("%s must need a keyword", name)
		}
	}
}

func TestFrenchIDCard(t *testing.T) {
	checkCases(t, "fr-id-card", map[string]bool{
		"880692310285": true,  // 1995 card, June 1988
		"95012A012345": true,  // Corsican department
		"881392310285": false, // month 13
		"880092310285": false, // month 00
		"8806923102AB": false, // order number with letters
		"X4RTBPFW4":    true,  // 2021 card
		"123456789":    false, // no letter
		"ABCDEFGHJ":    false, // no digit
		"X4RTBPFW":     false, // eight characters
	})
	d, _ := Get("fr-id-card")
	if !d.Needs.Admits("numéro de carte d'identité : X4RTBPFW4", 1) || !d.Needs.Admits("CNI 880692310285", 12) {
		t.Errorf("prefilter rejects a line naming the card")
	}
	if d.Needs.Admits("order_id: 880692310285", 12) {
		t.Errorf("prefilter admits a line that does not name the card")
	}
}

func TestDutchIDCard(t *testing.T) {
	d, _ := Get("nl-id-card")
	cases := map[string]bool{
		"SPECI2014": true,
		"NPKR4F7J3": true,
		"AB1234567": true,
		"OB1234567": false, // O is never used
		"AB12O4567": false,
		"AB123456X": false, // last character must be a digit
		"A12345678": false, // first two must be letters
		"AB123456":  false,
	}
	for v, want := range cases {
		if got := d.Pattern.FindString(v) == v; got != want {
			t.Errorf("case of %d characters, valid=%v: got %v", len(v), want, got)
		}
	}
	if !d.Needs.Admits("identiteitskaartnummer: NPKR4F7J3", 2) || d.Needs.Admits("id: nPKR4F7J3", 2) {
		t.Errorf("prefilter does not look for two capitals at the start of a word")
	}
}

func TestBelgianIDCard(t *testing.T) {
	withMod97 := func(body string) string {
		rem := 0
		for _, c := range body {
			rem = (rem*10 + int(c-'0')) % 97
		}
		if rem == 0 {
			rem = 97
		}
		return fmt.Sprintf("%s%02d", body, rem)
	}
	ok := withMod97("5912345678")
	checkCases(t, "be-id-card", map[string]bool{
		ok:                                      true,
		ok[:3] + "-" + ok[3:10] + "-" + ok[10:]: true,
		withMod97("0000000097"):                 true, // remainder 0 is written 97
		changeDigit(ok):                         false,
		ok[:11]:                                 false,
		"000000000000":                          false, // remainder 0 must be written 97
	})
	d, _ := Get("be-id-card")
	if d.Rival == nil || !d.Rival.Valid(ok) {
		t.Errorf("Belgian account numbers share the check and must be a rival")
	}
}

func TestHungarianIDCard(t *testing.T) {
	d, _ := Get("hu-id-card")
	cases := map[string]bool{
		"123456AB": true,
		"AB123456": true,
		"12345AB":  false,
		"123456A":  false,
		"1234567A": false,
		"ABC12345": false,
	}
	for v, want := range cases {
		if got := d.Pattern.FindString(v) == v; got != want {
			t.Errorf("case of %d characters, valid=%v: got %v", len(v), want, got)
		}
	}
}

func TestGreekIDCard(t *testing.T) {
	checkCases(t, "gr-id-card", map[string]bool{
		"AB-123456":  true,
		"ΑΒ-123456":  true, // the same series in Greek
		"ΑΚ 123456":  true,
		"ΧΤ123456":   true,
		"Σ-123456":   true,  // older card, one Greek letter
		"K-123456":   true,  // older card in Latin look-alike
		"AC-123456":  false, // C has no Greek twin
		"ΣΑ-123456":  false, // Σ is not in the two-letter series
		"AB-12345":   false,
		"AB-1234567": false,
	})
	d, _ := Get("gr-id-card")
	for _, line := range []string{"ΑΔΤ: ΑΒ-123456", "adt AB 123456", "X123456"} {
		if !d.Needs.Admits(line, 6) {
			t.Errorf("prefilter rejects a line with a card number")
		}
	}
	if d.Needs.Admits("order 123456", 6) || d.Needs.Admits("a-123456", 6) {
		t.Errorf("prefilter admits six digits without a capital before them")
	}
	if m := d.Mask(d.Normalize("ΑΒ-123456")); m != "••••••••" {
		t.Errorf("mask must hide the value and keep its length")
	}
}

func TestBulgarianIDCard(t *testing.T) {
	d, _ := Get("bg-id-card")
	if !d.Needs.Admits("номер на лична карта: 123456789", 9) || d.Needs.Admits("номер: 123456789", 9) {
		t.Errorf("prefilter does not require the line to name the card")
	}
	if d.Pattern.FindString("123456789") != "123456789" || d.Pattern.FindString("12345678") != "" {
		t.Errorf("pattern must find nine digits")
	}
}
