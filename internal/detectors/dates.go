package detectors

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// Date of birth next to a birth-date word. There is no check digit, so the date itself is the only test: a
// real calendar day, a year from 1900 and not in the future. Layouts are the ones used across the EU
// (ISO 8601 YYYY-MM-DD, the Hungarian YYYY.MM.DD. and YYYY. MM. DD., and day first with dots, slashes or
// dashes, as in DD.MM.YYYY, D.M.YYYY, DD/MM/YYYY, DD-MM-YYYY and the Czech and Croatian "12. 3. 1985."), plus English month
// names ("12 March 1985", "March 12, 1985", "12-Mar-1985"). Other languages' month names are left out:
// most of them are inflected (Polish "marca", Czech "března", Finnish "maaliskuuta") and need native review.
// Bare eight-digit YYYYMMDD is left out too: it is indistinguishable from order numbers and amounts, and the
// keyword gate also accepts a keyword on the line before. Day first is always read as day first, the EU
// convention; a US month-first date with a day above 12 is rejected.
// Formats: ISO 8601 (https://www.iso.org/iso-8601-date-and-time-format.html) and the national short date
// formats listed at https://en.wikipedia.org/wiki/List_of_date_formats_by_country.
func init() {
	Register(Detector{
		Name:         "birth-date",
		Label:        "Date of birth",
		Country:      "EU",
		OnByDefault:  false,
		Pattern:      birthDatePattern,
		Validate:     validBirthDate,
		Keywords:     []string{"date_of_birth", "dateofbirth", "birth_date", "birthdate", "dob"},
		Concepts:     []Concept{BirthDate},
		Base:         domain.Low,
		Validation:   "a real calendar date between 1900 and today",
		NeedsKeyword: true,
		Needs:        Needs{Check: hasBirthYear},
	})
}

const monthNames = `(?:[Jj]an(?:uary)?|[Ff]eb(?:ruary)?|[Mm]ar(?:ch)?|[Aa]pr(?:il)?|[Mm]ay|[Jj]une?|[Jj]uly?|[Aa]ug(?:ust)?|[Ss]ep(?:t(?:ember)?)?|[Oo]ct(?:ober)?|[Nn]ov(?:ember)?|[Dd]ec(?:ember)?)`

var birthDatePattern = regexp.MustCompile(
	`(?:19|20)\d{2}(?:-\d{2}-\d{2}|/\d{2}/\d{2}|\.\d{1,2}\.\d{1,2}\.?|\. \d{1,2}\. \d{1,2}\.?)` +
		`|\d{1,2}(?:\.\d{1,2}\.|\. \d{1,2}\. |/\d{1,2}/|-\d{1,2}-)(?:19|20)\d{2}\.?` +
		`|\d{1,2}(?:\.? |-)` + monthNames + `\.?(?: |-)(?:19|20)\d{2}` +
		`|` + monthNames + `\.? \d{1,2}(?:st|nd|rd|th)?, (?:19|20)\d{2}`)

var englishMonths = []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"}

// monthNumber reads a full English month name or an abbreviation of at least three letters.
func monthNumber(w string) (int, bool) {
	w = strings.ToLower(w)
	for i, m := range englishMonths {
		if strings.HasPrefix(m, w) {
			return i + 1, true
		}
	}
	return 0, false
}

var dateNumber = regexp.MustCompile(`\d+`)
var dateWord = regexp.MustCompile(`[A-Za-z]{3,}`)

// validBirthDate reads the year, month and day out of any of the layouts and checks the date.
func validBirthDate(s string) bool {
	nums := dateNumber.FindAllString(s, -1)
	var year, month, day int
	if w := dateWord.FindString(s); w != "" {
		m, ok := monthNumber(w)
		if !ok || len(nums) != 2 {
			return false
		}
		month = m
		day, _ = strconv.Atoi(nums[0])
		year, _ = strconv.Atoi(nums[1])
	} else {
		if len(nums) != 3 || !consistentSeparators(s) {
			return false
		}
		a, _ := strconv.Atoi(nums[0])
		b, _ := strconv.Atoi(nums[1])
		c, _ := strconv.Atoi(nums[2])
		if len(nums[0]) == 4 {
			year, month, day = a, b, c
		} else {
			day, month, year = a, b, c
		}
	}
	now := time.Now()
	if year < 1900 || !ValidDate(year, month, day) {
		return false
	}
	return !time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).After(now)
}

// consistentSeparators rejects mixed layouts such as 12.03-1985.
func consistentSeparators(s string) bool {
	seps := strings.Map(func(r rune) rune {
		if r == '.' || r == '/' || r == '-' {
			return r
		}
		return -1
	}, s)
	return len(seps) >= 2 && strings.Count(seps, seps[:1]) == len(seps)
}

// hasBirthYear is the prefilter: a run of exactly four digits starting with 19 or 20.
func hasBirthYear(line string) bool {
	for i := 0; i+4 <= len(line); i++ {
		if (i == 0 || !isASCIIDigit(line[i-1])) && (line[i] == '1' && line[i+1] == '9' || line[i] == '2' && line[i+1] == '0') &&
			isASCIIDigit(line[i+2]) && isASCIIDigit(line[i+3]) && (i+4 == len(line) || !isASCIIDigit(line[i+4])) {
			return true
		}
	}
	return false
}
