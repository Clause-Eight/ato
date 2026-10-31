package detectors

import "testing"

func TestCodesNeedAKeywordAndAPerson(t *testing.T) {
	for _, name := range []string{"icd10", "icd11", "atc-code", "it-aic"} {
		d, ok := Get(name)
		if !ok {
			t.Fatalf("%s is not registered", name)
		}
		if !d.NeedsKeyword || !d.NeedsPerson || d.Special == "" {
			t.Errorf("%s must need a keyword and a person and carry an Art. 9 category", name)
		}
	}
}

func TestICD10Chapters(t *testing.T) {
	for _, v := range []string{"A00", "B99", "C97", "D48", "D50", "D89", "E90", "H95", "K93", "P96", "T98", "U85", "V01", "Y98", "Z99", "E11.9", "J45.90"} {
		if !validICD10(v) {
			t.Errorf("%s rejected", v)
		}
	}
	for _, v := range []string{"C98", "D49", "D90", "E91", "H96", "K94", "P97", "T99", "U86", "V00", "Y99", "E1", "1E1", "e11"} {
		if validICD10(v) {
			t.Errorf("%s accepted", v)
		}
	}
	d, _ := Get("icd10")
	if got := d.Pattern.FindString("code E11.9,"); got != "E11.9" {
		t.Errorf("pattern found %q", got)
	}
}

func TestICD11StemCodes(t *testing.T) {
	d, _ := Get("icd11")
	for _, v := range []string{"5A11", "BA00", "1F0Z", "6A70.1", "CA40.0Z"} {
		if got := d.Pattern.FindString(v); got != v || !validICD11(v) {
			t.Errorf("%s not found whole and valid (%q)", v, got)
		}
	}
	for _, v := range []string{"XN0A", "VA01", "5I11", "BA0O"} {
		if validICD11(v) {
			t.Errorf("%s accepted", v)
		}
	}
	for _, v := range []string{"E11.9", "51A1", "T1AA"} {
		if d.Pattern.MatchString(v) && validICD11(d.Pattern.FindString(v)) && d.Pattern.FindString(v) == v {
			t.Errorf("%s read as ICD-11", v)
		}
	}
}

func TestATCLayout(t *testing.T) {
	for _, v := range []string{"A10BA02", "N02BE01", "V03AB15"} {
		if !validATC(v) {
			t.Errorf("%s rejected", v)
		}
	}
	for _, v := range []string{"E10BA02", "A00BA02", "A10B402", "A10BA0", "A10BA02X"} {
		if validATC(v) {
			t.Errorf("%s accepted", v)
		}
	}
}

func TestAICCheckDigit(t *testing.T) {
	// Made-up pack codes, with check digits worked out by hand.
	for _, v := range []string{"012345676", "000000000", "098765439"} {
		if !validAIC(v) {
			t.Errorf("%s rejected", v)
		}
	}
	for _, v := range []string{"012345675", "112345676", "01234567", "0123456760"} {
		if validAIC(v) {
			t.Errorf("%s accepted", v)
		}
	}
}

func TestLetterThenDigitPrefilter(t *testing.T) {
	if !letterThenDigit("dx E11") || !letterThenDigit("5A11") || letterThenDigit("no codes here 123") {
		t.Error("prefilter")
	}
}
