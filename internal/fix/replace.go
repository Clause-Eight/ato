// Package fix replaces values found in files with synthetic ones from the same generator the tests use. Each
// replacement passes the same detector, keeps the original's separators, case and length where the format
// allows, keeps the original's country, brand or class where the identifier has one, and is the same for the
// same value everywhere. Replacements carry the synthetic mark (detectors.Synthetic), so a later scan reports
// them at low confidence as samples.
package fix

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"unicode"

	"github.com/Clause-Eight/ato/v2/internal/detectors"
	"github.com/Clause-Eight/ato/v2/internal/synth"
)

// maxTries bounds the search for a marked value: about eight times the expected number of tries, so a detector
// with many possible values almost never runs out, and one with few gives up in well under a second.
const maxTries = 1 << 19

// likeTries bounds the search over values like the original, where only a few in ten may pass the detector (a
// plate's letters, a phone number's plan).
const likeTries = 4 * maxTries

// enoughShaped is how many values of the original's shape a search that already has a marked value of another
// shape tries before it stops looking: four times the expected number for one to carry the mark, so a value
// whose shape has room for the mark keeps its shape about 98 times in 100.
const enoughShaped = 1 << 18

// shapeTries is how many values a generator without a shaper draws, per candidate, looking for one with
// letters and digits where the original has them.
const shapeTries = 64

// shapeBudget bounds the extra values those draws cost in one search: enough for a shape one generated value in
// fifty has.
const shapeBudget = 8 * maxTries

// unmarkable are detectors whose generated values cannot carry the synthetic mark (names from a list, dates of
// birth, street addresses); a search for one would run to maxTries without finding a marked value, so theirs
// stop after unmarkedTries.
var unmarkable = map[string]bool{"person-name": true, "birth-date": true, "street-address": true}

const unmarkedTries = 1 << 12

// Replacement is the synthetic value written in place of a found one. Marked says it carries the synthetic
// mark; SameShape says it keeps the original's separators, case, length and the positions of its letters and
// digits.
type Replacement struct {
	Text      string
	Marked    bool
	SameShape bool
}

// Replace returns a synthetic value for raw, a value as written in text and found by d. The same detector and
// value always give the same replacement. ok is false when d has no generator or no generated value fits.
//
// Where the identifier has a country, brand or class (an IBAN's country, a card's brand, a phone number's
// country code, the sex and century of a personal number), candidates come from synth.Like and keep it. The
// other detectors draw from their generator, preferring values with letters and digits in the original's
// positions.
func Replace(d detectors.Detector, raw string) (Replacement, bool) {
	gen, ok := synth.For(d.Name)
	if !ok {
		return Replacement{}, false
	}
	original := detectors.Canonical(raw)
	tries, triesLike := maxTries, likeTries
	if unmarkable[d.Name] {
		tries, triesLike = unmarkedTries, unmarkedTries
	}
	if like, ok := synth.Like(d.Name, raw); ok {
		r := rand.New(rand.NewSource(seed(d.Name, original)))
		if best, found := search(d, raw, original, like, r, triesLike); found && best.Marked {
			return best, true
		}
	}
	plain, found := search(d, raw, original, gen, rand.New(rand.NewSource(seed(d.Name, original))), tries)
	if !found || plain.SameShape || !plain.Marked {
		// Values that never carry the mark (names from a list) are not worth a longer search.
		return plain, found
	}
	if best, ok := search(d, raw, original, shaped(gen, raw), rand.New(rand.NewSource(seed(d.Name, original))), tries); ok && best.Marked {
		return best, true
	}
	return plain, true
}

// search draws candidates from gen until one fits, carries the mark and keeps the shape of raw, or the tries
// run out, and returns the best one found.
func search(d detectors.Detector, raw, original string, gen synth.Generator, r *rand.Rand, tries int) (Replacement, bool) {
	var best Replacement
	found := false
	better := func(c Replacement) bool {
		if !found {
			return true
		}
		if c.Marked != best.Marked {
			return c.Marked
		}
		return c.SameShape && !best.SameShape
	}
	shapedCount := 0
	for i := 0; i < tries; i++ {
		cand := gen(r)
		c := Replacement{Text: cand}
		if out, ok := layout(raw, cand, true); ok && classes(out) == classes(raw) {
			c = Replacement{Text: out, SameShape: true}
			shapedCount++
			if !fits(d, out) {
				// The case of the letters is part of a checksum (EIP-55): keep the candidate's.
				c.Text, _ = layout(raw, cand, false)
			}
		}
		c.Marked = detectors.Synthetic(c.Text)
		if better(c) && fits(d, c.Text) && detectors.Canonical(c.Text) != original {
			best, found = c, true
			if best.Marked && best.SameShape {
				return best, true
			}
		}
		if best.Marked && (shapedCount >= enoughShaped || i >= 1000 && shapedCount == 0) {
			break
		}
	}
	return best, found
}

// shaped wraps gen to return, of up to shapeTries values, the first with letters and digits where raw has
// them, else the first with as many, else the last. Once it has drawn shapeBudget values more than it
// returned, it returns gen's, so a shape the generator rarely writes costs a bounded search.
func shaped(gen synth.Generator, raw string) synth.Generator {
	want := classes(raw)
	extra := 0
	return func(r *rand.Rand) string {
		if extra >= shapeBudget {
			return gen(r)
		}
		var count, cand string
		for i := 0; i < shapeTries; i++ {
			if i > 0 {
				extra++
			}
			cand = gen(r)
			c := classes(cand)
			if c == want {
				return cand
			}
			if count == "" && len(c) == len(want) {
				count = cand
			}
		}
		if count != "" {
			return count
		}
		return cand
	}
}

// classes writes each letter of s as A and each digit as 9, and drops everything else.
func classes(s string) string {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case unicode.IsDigit(r):
			b = append(b, '9')
		case unicode.IsLetter(r):
			b = append(b, 'A')
		}
	}
	return string(b)
}

// seed derives the generator's seed from the detector and the value, so the same value maps to the same
// replacement in every file and every run. The salt keeps the former product name so replacements stay the same
// as in 1.x.
func seed(detector, canonical string) int64 {
	sum := sha256.Sum256([]byte("pii-eu fix\x00" + detector + "\x00" + canonical))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

// layout writes the letters and digits of cand into the positions of the letters and digits of orig, keeping
// orig's separators and, when keepCase is set, the case of its letters. ok is false when they have different
// numbers of letters and digits.
func layout(orig, cand string, keepCase bool) (string, bool) {
	var src []rune
	for _, r := range cand {
		if isAlnum(r) {
			src = append(src, r)
		}
	}
	out := []rune(orig)
	k := 0
	for i, r := range out {
		if !isAlnum(r) {
			continue
		}
		if k == len(src) {
			return "", false
		}
		c := src[k]
		k++
		switch {
		case !keepCase:
		case unicode.IsLower(r):
			c = unicode.ToLower(c)
		case unicode.IsUpper(r):
			c = unicode.ToUpper(c)
		}
		out[i] = c
	}
	if k != len(src) {
		return "", false
	}
	return string(out), true
}

// fits reports whether the detector finds text whole and accepts it.
func fits(d detectors.Detector, text string) bool {
	if d.Pattern == nil {
		return false
	}
	loc := d.Pattern.FindStringIndex(text)
	if loc == nil || loc[0] != 0 || loc[1] != len(text) {
		return false
	}
	return d.Validate == nil || d.Validate(d.Normalize(text))
}

func isAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
