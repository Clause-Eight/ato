package art9

import (
	"strings"
	"unicode"
)

// HL7 v2 (HL7 Version 2.5.1 Messaging Standard, chapter 3, segment PID, https://www.hl7.eu/HL7v2x/v251/std251/ch03.html#Heading55)
// identifies the patient of a message in its PID segment. Segments end with a carriage return and fields are
// separated by a vertical bar, components by a caret and repetitions by a tilde. The fields read here are PID-2
// and PID-3 (patient identifiers), PID-5 (patient name: family name, then given name), PID-7 (date and time of
// birth, YYYYMMDD first) and PID-19 (social security number). A segment is reported when it names the patient,
// a family name of two letters or more, and also identifies them by an identifier or a birth date: the whole
// message is then about that person's health care.

// pid finds the PID segments of one line. A line may hold a whole message with carriage returns, or a message in
// a string literal of source code, with the returns written as escapes.
func (s *Scanner) pid(at place) []Hit {
	var out []Hit
	line := at.text
	for from := 0; ; {
		i := strings.Index(line[from:], "PID|")
		if i < 0 {
			return out
		}
		i += from
		from = i + 4
		quote, ok := segmentStart(line, i)
		if !ok {
			continue
		}
		end := segmentEnd(line, i, quote)
		reasons, sample, ok := patientSegment(line[i:end])
		if !ok {
			continue
		}
		at.column = len([]rune(line[:i])) + 1
		out = append(out, s.hit(HL7, at, "PID segment", reasons, sample))
	}
}

// segmentStart reports whether PID at i starts a segment, and returns the quote that closes the string literal
// it is in: the quote before it, or a double quote after an escaped return.
func segmentStart(line string, i int) (quote byte, ok bool) {
	if i == 0 {
		return 0, true
	}
	switch c := line[i-1]; c {
	case '"', '\'', '`':
		return c, true
	case '\r', '\n', ' ', '\t', '>', '[', ',':
		return 0, true
	}
	if i >= 2 && line[i-2] == '\\' && (line[i-1] == 'r' || line[i-1] == 'n') {
		return '"', true
	}
	return 0, false
}

// segmentEnd is where a segment ends: a carriage return, an escape, the closing quote of the string literal it
// is in, or the end of the line. Outside a literal a double quote is data: HL7 writes an explicit null as "".
func segmentEnd(line string, i int, quote byte) int {
	for j := i; j < len(line); j++ {
		c := line[j]
		if c == '\r' || c == '\\' || quote != 0 && c == quote {
			return j
		}
	}
	return len(line)
}

// patientSegment reads a PID segment; ok is true when it names and identifies a patient.
func patientSegment(seg string) (reasons []string, sample string, ok bool) {
	fields := strings.Split(seg, "|")
	field := func(n int) string {
		if n >= len(fields) {
			return ""
		}
		first, _, _ := strings.Cut(fields[n], "~")
		return first
	}
	component := func(f string, n int) string {
		parts := strings.Split(f, "^")
		if n >= len(parts) {
			return ""
		}
		return strings.TrimSpace(parts[n])
	}
	family, given := component(field(5), 0), component(field(5), 1)
	if !hasLetters(family, 2) {
		return nil, "", false
	}
	reasons = []string{"an HL7 v2 PID segment with the patient's name"}
	id := false
	for _, n := range []int{2, 3, 19} {
		if v := component(field(n), 0); strings.IndexFunc(v, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			id = true
		}
	}
	if id {
		reasons = append(reasons, "and an identifier")
	}
	birth := birthYear(component(field(7), 0))
	if birth {
		reasons = append(reasons, "and a date of birth")
	}
	if !id && !birth {
		return nil, "", false
	}
	if placeholderName(family) || given != "" && placeholderName(given) {
		sample = "the name is a placeholder used in examples"
	}
	return reasons, sample, true
}

// birthYear reports whether an HL7 date starts with a plausible year of birth.
func birthYear(v string) bool {
	if len(v) < 8 {
		return false
	}
	for i := 0; i < 8; i++ {
		if v[i] < '0' || v[i] > '9' {
			return false
		}
	}
	return v[:2] == "18" || v[:2] == "19" || v[:2] == "20"
}
