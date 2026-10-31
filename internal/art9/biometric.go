package art9

import (
	"fmt"
	"regexp"
	"strings"
)

// A biometric template is what face, fingerprint, iris and voice recognition store for a person: a face
// embedding of 128 or 512 numbers, fingerprint minutiae, an iris code, a voiceprint. A field named for one of
// them is reported when it is given a list of 64 numbers or more, on its line or the lines that follow up to
// the closing bracket, or a base64 or hex string of 100 characters or more. The field's name alone is already
// reported by the schema check; a dimension (face_embedding_dim: 128) or a type (Column(ARRAY(Float))) is not a
// template. A list of one number repeated is a placeholder.

var (
	biometricKey = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(face_?(?:embedding|encoding|descriptor|template|vector)s?|faceprint|fingerprint_?template|finger_?template|minutiae|iris_?(?:code|template)|voice_?print|voice_?embedding|speaker_?embedding|palm_?(?:print|vein)_?template)["'\]]?\s*(?::=|:|=>|=|<-)\s*`)
	blob         = regexp.MustCompile(`^(?:b|base64,)?["'` + "`" + `]([A-Za-z0-9+/_=-]{100,})["'` + "`" + `]`)
	number       = regexp.MustCompile(`-?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?`)
)

// minTemplate is the fewest numbers a template has; maxTemplateLines bounds how far a list is followed.
const (
	minTemplate      = 64
	maxTemplateLines = 2000
)

type biometricScanner struct {
	open *template
}

type template struct {
	field  string
	at     place
	count  int
	first  string
	varied bool
	lines  int
}

func (b *biometricScanner) line(s *Scanner, at place) []Hit {
	if b.open != nil {
		closed := b.open.read(at.text)
		if !closed && b.open.lines < maxTemplateLines {
			return nil
		}
		return b.close(s)
	}
	loc := biometricKey.FindStringSubmatchIndex(at.text)
	if loc == nil {
		return nil
	}
	field, rest := at.text[loc[2]:loc[3]], at.text[loc[1]:]
	at.column = len([]rune(at.text[:loc[2]])) + 1
	if m := blob.FindStringSubmatch(rest); m != nil {
		return []Hit{s.hit(Biometric, at, field, []string{
			fmt.Sprintf("the field %s is given an encoded value of %d characters", field, len(m[1]))}, "")}
	}
	open := strings.IndexAny(rest, "[{(")
	if open < 0 || open > 40 {
		return nil
	}
	b.open = &template{field: field, at: at}
	if !b.open.read(rest[open+1:]) {
		return nil
	}
	return b.close(s)
}

// read counts the numbers of one line of a list and reports whether the list closed on it.
func (t *template) read(text string) bool {
	t.lines++
	end := strings.IndexAny(text, "]})")
	if end >= 0 {
		text = text[:end]
	}
	for _, n := range number.FindAllString(text, -1) {
		if t.count == 0 {
			t.first = n
		} else if n != t.first {
			t.varied = true
		}
		t.count++
	}
	return end >= 0
}

func (b *biometricScanner) close(s *Scanner) []Hit {
	t := b.open
	b.open = nil
	if t.count < minTemplate {
		return nil
	}
	sample := ""
	if !t.varied {
		sample = "every number in the list is the same, as in a placeholder"
	}
	return []Hit{s.hit(Biometric, t.at, t.field, []string{fmt.Sprintf("the field %s is given a list of %d numbers", t.field, t.count)}, sample)}
}

func (b *biometricScanner) end(s *Scanner) []Hit {
	if b.open == nil {
		return nil
	}
	return b.close(s)
}
