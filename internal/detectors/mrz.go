package detectors

import (
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Machine readable zone (MRZ) of passports and ID cards, ICAO Doc 9303 parts 3, 4 and 5
// (https://www.icao.int/publications/pages/publication.aspx?docnum=9303).
//
// Check digits (part 3, 4.9): digits keep their value, A-Z count 10-35, the filler < counts 0; the values are
// weighted 7, 3, 1 repeating, and the check digit is the sum mod 10.
//
// The engine reads one line at a time, so the detector reports the one MRZ line that carries the document's
// numbers and dates:
//   - TD3 (passport, part 4, 2 lines of 44): the lower line, validated in full. Document number 1-9 and check 10,
//     nationality 11-13, birth date 14-19 and check 20, sex 21, expiry 22-27 and check 28, optional data 29-42
//     and check 43 (0 or < when the optional data is all filler), composite check 44 over 1-10, 14-20 and 22-43.
//   - TD1 (ID card, part 5, 3 lines of 30): the middle line. Birth date 1-6 and check 7, sex 8, expiry 9-14 and
//     check 15, nationality 16-18, optional data 19-29, composite 30. The composite also covers the upper line,
//     so it cannot be checked here; the two date check digits, the dates, the layout and at least one filler are.
//
// Not covered: the TD1 upper line (document number and its check), the TD1 composite, TD2 and MRV visas, and
// birth dates written with fillers because they are unknown. The name lines (TD3 upper, TD1 lower) are never
// reported, so names never appear in a finding; the reported line is masked in full.
func init() {
	Register(Detector{
		Name:        "mrz",
		Label:       "Machine readable zone of a passport or ID card (MRZ)",
		Country:     domain.CrossBorder,
		OnByDefault: true,
		Pattern:     regexp.MustCompile(`(?:\\r)?(?:\\n)?(?:` + mrzTD3Lower + `|` + mrzTD1Middle + `)`),
		Normalize:   trimEscapedNewline,
		Validate:    validMRZ,
		Mask:        MaskAll,
		Keywords:    []string{"mrz", "machine readable zone"},
		Concepts:    []Concept{Passport, IDCard},
		Base:        domain.Medium,
		Samples: map[string]bool{
			"L898902C36UTO7408122F1204159ZE184226B<<<<<10": true,
			"HA672242<6YTO5802254M9601086<<<<<<<<<<<<<<08": true,
			"7408122F1204159UTO<<<<<<<<<<<6":               true,
			"3407127M9507122YTO<<<<<<<<<<<2":               true,
		},
		Validation: "dates and check digits are valid",
		Needs:      Needs{Check: hasMRZRun},
	})
}

const (
	mrzTD3Lower  = `[A-Z0-9<]{9}[0-9][A-Z<]{3}[0-9]{7}[MFX<][0-9]{7}[A-Z0-9<]{14}[0-9<][0-9]`
	mrzTD1Middle = `[0-9]{7}[MFX<][0-9]{7}[A-Z<]{3}[A-Z0-9<]{11}[0-9]`
)

var (
	td3Lower  = regexp.MustCompile(`^` + mrzTD3Lower + `$`)
	td1Middle = regexp.MustCompile(`^` + mrzTD1Middle + `$`)
)

// trimEscapedNewline drops a literal \n or \r\n written before the line, as in a string constant holding a whole MRZ.
func trimEscapedNewline(s string) string {
	s = strings.TrimPrefix(s, `\r`)
	return strings.TrimPrefix(s, `\n`)
}

// hasMRZRun reports whether the line holds 30 or more MRZ characters in a row; every reported line does.
func hasMRZRun(line string) bool {
	run := 0
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '<' {
			if run++; run >= 30 {
				return true
			}
		} else {
			run = 0
		}
	}
	return false
}

func validMRZ(s string) bool {
	switch len(s) {
	case 44:
		return validTD3Lower(s)
	case 30:
		return validTD1Middle(s)
	}
	return false
}

func validTD3Lower(s string) bool {
	if !td3Lower.MatchString(s) {
		return false
	}
	if !mrzCheck(s[0:9], s[9]) || !mrzDate(s[13:19]) || !mrzCheck(s[13:19], s[19]) || !mrzDate(s[21:27]) || !mrzCheck(s[21:27], s[27]) {
		return false
	}
	if s[42] == '<' {
		if strings.Trim(s[28:42], "<") != "" {
			return false
		}
	} else if !mrzCheck(s[28:42], s[42]) {
		return false
	}
	return mrzCheck(s[0:10]+s[13:20]+s[21:43], s[43])
}

func validTD1Middle(s string) bool {
	if !td1Middle.MatchString(s) || !strings.Contains(s[15:29], "<") || strings.Trim(s[15:18], "<") == "" {
		return false
	}
	return mrzDate(s[0:6]) && mrzCheck(s[0:6], s[6]) && mrzDate(s[8:14]) && mrzCheck(s[8:14], s[14])
}

// mrzValue is a character's value in a check digit: digits as is, A-Z 10-35, the filler 0.
func mrzValue(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10, true
	case c == '<':
		return 0, true
	}
	return 0, false
}

var mrzWeights = [3]int{7, 3, 1}

// mrzCheck reports whether check is the ICAO 7-3-1 check digit of field.
func mrzCheck(field string, check byte) bool {
	if check < '0' || check > '9' {
		return false
	}
	sum := 0
	for i := 0; i < len(field); i++ {
		v, ok := mrzValue(field[i])
		if !ok {
			return false
		}
		sum += v * mrzWeights[i%3]
	}
	return sum%10 == int(check-'0')
}

// mrzDate checks YYMMDD; the century is not written, so a date valid in either the 1900s or the 2000s passes.
func mrzDate(s string) bool {
	d, ok := Digits(s)
	if !ok || len(d) != 6 {
		return false
	}
	yy, mm, dd := d[0]*10+d[1], d[2]*10+d[3], d[4]*10+d[5]
	return ValidDate(1900+yy, mm, dd) || ValidDate(2000+yy, mm, dd)
}
