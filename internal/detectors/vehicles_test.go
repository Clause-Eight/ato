package detectors

import (
	"strings"
	"testing"
)

func TestDrivingLicenceFormats(t *testing.T) {
	d, _ := Get("driving-licence")
	if d.OnByDefault || !d.NeedsKeyword {
		t.Fatal("the driving licence detector must be off by default and keyword-gated")
	}
	valid := []string{
		"K12034XY701", "M45177AB231", // DE, check character 0 and 3
		"40404040D", "Y1234567X", // ES DNI and NIE
		"150385-1230", "010203A456R", // FI
		"850709-1232",                       // SE
		"EB 123456", "ET123456", "AB123456", // CZ, EE, HU
		"ABC123456", "123456ABCD", "U1H123456Z", "U1-H123456-Z", // LV, IE, IT
		"AB 123 456", "12345/12/1234", "12345/12/1234567", // MT, PL
		"AB-123456 7", "A-12345678 9", "A12345678", "A1234567", // PT, RO, SK
	}
	for i, v := range valid {
		if !validLicence(d.Normalize(v)) {
			t.Errorf("valid case %d rejected", i)
		}
		if got := d.Pattern.FindString("x " + v + " y"); got != v {
			t.Errorf("valid case %d not found whole (%d of %d characters)", i, len(got), len(v))
		}
	}
	invalid := []string{
		"K12034XY711", "112034XY701", // DE wrong check, no state letter
		"40404040E", "Y1234567A", // ES wrong letter
		"150385-1231", "320185-1234", "150385-0010", // FI wrong control, impossible date, individual number 001
		"850709-1233", "851309-1236", // SE wrong Luhn, month 13
		"12345678", "123456789", "1234567890", "123456789012", // bare numbers are not covered
		"12345/1/1234", "AB-1234 5", // PL and PT lengths
	}
	for i, v := range invalid {
		if validLicence(d.Normalize(v)) {
			t.Errorf("invalid case %d accepted", i)
		}
	}
	if d.Normalize("eb 123456") != "EB 123456" {
		t.Error("normalisation must upper-case")
	}
	if m := d.Mask("K12034XY701"); m != strings.Repeat("•", 11) {
		t.Errorf("mask %q", m)
	}
}

func TestLicencePlateFormats(t *testing.T) {
	d, _ := Get("licence-plate")
	if d.OnByDefault || !d.NeedsKeyword {
		t.Fatal("the plate detector must be off by default and keyword-gated")
	}
	valid := []string{
		"B 123 ABC", "B 12 ABC", "CJ 12 ABC", "M-AB 1234", "HH AB 123E", "AB-123-CD", "AB 123 CD", "1234 BCD",
		"WX 123AC", "WE 1A234", "KR 12345", "GD-12-BF", "12-BBB-3", "1-KBB-00", "ABC-123", "AA BB-123",
		"CA 1234 AB", "PB 1234 KX", "1A2 3456", "1-ABC-234", "AB-12-CD", "ABC 12A", "ZG 123-AB", "AB 12 345",
		"123 ABC", "AB-1234", "191-D-12345", "12-KY-345", "ABC-1234", "AB 1234",
	}
	for i, v := range valid {
		if !validPlate(d.Normalize(v)) {
			t.Errorf("valid case %d rejected", i)
		}
		if got := d.Pattern.FindString(`plate: "` + v + `"`); got != v {
			t.Errorf("valid case %d not found whole (%d of %d characters)", i, len(got), len(v))
		}
	}
	invalid := []string{
		"XX 12 ABC", "B 1234 ABC", "B 12 QAB", // RO unknown county, four digits, Q
		"AB-123-OD", "1234 ABC", // FR letter O, ES vowel
		"QQ 1234 AB",                            // BG unknown province
		"ABCDEF", "123456", "ABCDEFGH 12345678", // no mix, too long
	}
	for i, v := range invalid {
		if validPlate(d.Normalize(v)) {
			t.Errorf("invalid case %d accepted", i)
		}
	}
	if !hasLetterNextToDigit("plate: AB-12") || hasLetterNextToDigit("timeout: 30s") {
		t.Error("prefilter wrong")
	}
	if m := d.Mask("B 123 ABC"); len([]rune(m)) != 9 || strings.ContainsAny(m, "B123AC") {
		t.Errorf("mask %q", m)
	}
}
