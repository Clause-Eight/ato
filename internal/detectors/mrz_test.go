package detectors

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/Clause-Eight/ato/v2/internal/synth"
)

// icao731 is a test-side ICAO 9303 check digit, written apart from the detector's.
func icao731(s string) string {
	sum := 0
	for i, c := range s {
		v := 0
		if c >= '0' && c <= '9' {
			v = int(c - '0')
		} else if c >= 'A' && c <= 'Z' {
			v = int(c-'A') + 10
		}
		sum += v * []int{7, 3, 1}[i%3]
	}
	return fmt.Sprint(sum % 10)
}

// td3 builds a TD3 lower line for Utopia from its fields, with correct check digits.
func td3(number, birth, sex, expiry, optional string) string {
	optional += strings.Repeat("<", 14-len(optional))
	optCheck := icao731(optional)
	if strings.Trim(optional, "<") == "" {
		optCheck = "<"
	}
	doc, b, e := number+icao731(number), birth+icao731(birth), expiry+icao731(expiry)
	return doc + "UTO" + b + sex + e + optional + optCheck + icao731(doc+b+e+optional+optCheck)
}

// td1 builds a TD1 middle line for Utopia; its composite digit is arbitrary, as it also covers the upper line.
func td1(birth, sex, expiry, optional string) string {
	optional += strings.Repeat("<", 11-len(optional))
	return birth + icao731(birth) + sex + expiry + icao731(expiry) + "UTO" + optional + "7"
}

// zeroFillerCheck writes 0 instead of < as the check digit of empty optional data, which ICAO also allows.
func zeroFillerCheck(s string) string {
	s = s[:42] + "0"
	return s + icao731(s[:10]+s[13:20]+s[21:43])
}

func TestMRZValidLines(t *testing.T) {
	d, _ := Get("mrz")
	valid := map[string]string{
		"passport without optional data":     td3("AB1234567", "850412", "F", "310930", ""),
		"passport with a personal number":    td3("ZX0000017", "000229", "M", "290101", "123456789"),
		"passport with a short number":       td3("C1234567<", "991231", "<", "350615", ""),
		"passport with zero as filler check": zeroFillerCheck(td3("AB1234567", "850412", "X", "310930", "")),
		"id card":                            td1("760101", "M", "300228", ""),
		"id card with optional data":         td1("010315", "F", "331120", "AB12"),
	}
	for name, v := range valid {
		if got := d.Pattern.FindString(v); got != v {
			t.Errorf("%s: pattern found %d of %d characters", name, len(got), len(v))
		}
		if !d.Validate(v) {
			t.Errorf("%s: rejected", name)
		}
	}
}

func TestMRZRejectsBrokenLines(t *testing.T) {
	d, _ := Get("mrz")
	good := td3("AB1234567", "850412", "F", "310930", "123456789")
	set := func(s string, i int, c byte) string { return s[:i] + string(c) + s[i+1:] }
	bump := func(s string, i int) string { return set(s, i, '0'+(s[i]-'0'+1)%10) }
	invalid := map[string]string{
		"wrong document number check": bump(good, 9),
		"wrong birth date check":      bump(good, 19),
		"wrong expiry check":          bump(good, 27),
		"wrong optional data check":   bump(good, 42),
		"wrong composite check":       bump(good, 43),
		"changed document number":     set(good, 2, '9'),
		"impossible birth date":       td3("AB1234567", "851332", "F", "310930", ""),
		"impossible expiry date":      td3("AB1234567", "850412", "F", "310230", ""),
		"unknown sex letter":          set(good, 20, 'Q'),
		"digits in the nationality":   set(good, 11, '1'),
		"too short":                   good[:43],
		"too long":                    good + "<",
		"filler check over data":      set(good, 42, '<'),
		"lower case":                  strings.ToLower(good),
		"id card wrong birth check":   bump(td1("760101", "M", "300228", ""), 6),
		"id card wrong expiry check":  bump(td1("760101", "M", "300228", ""), 14),
		"id card impossible date":     td1("760230", "M", "300228", ""),
		"id card without any filler":  td1("760101", "M", "300228", "ABCDEFGHIJK"),
		"id card without nationality": strings.Replace(td1("760101", "M", "300228", ""), "UTO", "<<<", 1),
	}
	for name, v := range invalid {
		if d.Validate(v) {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestMRZSpecimensAreSamples(t *testing.T) {
	d, _ := Get("mrz")
	if len(d.Samples) == 0 {
		t.Fatal("no specimen values")
	}
	for v := range d.Samples {
		if !d.Validate(v) {
			t.Errorf("a specimen of length %d does not validate", len(v))
		}
	}
}

func TestMRZMaskHidesEveryCharacter(t *testing.T) {
	d, _ := Get("mrz")
	r := rand.New(rand.NewSource(5))
	gen, _ := synth.For("mrz")
	for i := 0; i < 50; i++ {
		v := gen(r)
		if m := d.Mask(v); m != strings.Repeat("•", len(v)) {
			t.Fatalf("value %d: the mask leaves characters in clear", i)
		}
	}
}

func TestMRZNormalizeDropsEscapedNewline(t *testing.T) {
	d, _ := Get("mrz")
	v := td3("AB1234567", "850412", "F", "310930", "")
	for _, prefix := range []string{"", `\n`, `\r\n`} {
		if got := d.Normalize(d.Pattern.FindString(prefix + v)); got != v {
			t.Errorf("prefix %q: normalised to %d characters", prefix, len(got))
		}
	}
}

func TestMRZPrefilter(t *testing.T) {
	d, _ := Get("mrz")
	v := td3("AB1234567", "850412", "F", "310930", "")
	if !d.Needs.Admits("mrz = \""+v+"\"", 20) {
		t.Error("a line with an MRZ is filtered out")
	}
	if d.Needs.Admits("order 1234567890 shipped 2024-01-01T10:00:00Z ref 99887766554433", 30) {
		t.Error("an ordinary line passes the prefilter")
	}
}
