// Package fix replaces values found in files with synthetic ones from the same generator the tests use. Each
// replacement passes the same detector, keeps the original's separators and length where the format allows,
// and is the same for the same value everywhere. Replacements carry the synthetic mark (detectors.Synthetic),
// so a later scan reports them at low confidence as samples.
package fix

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"unicode"

	"github.com/Clause-Eight/pii-eu/internal/detectors"
	"github.com/Clause-Eight/pii-eu/internal/synth"
)

// maxTries bounds the search for a marked value: about eight times the expected number of tries, so a detector
// with many possible values almost never runs out, and one with few gives up in well under a second.
const maxTries = 1 << 19

// enoughShaped is how many values of the original's shape a search that already has a marked value of another
// shape tries before it stops looking: twice the expected number for one to carry the mark.
const enoughShaped = 1 << 17

// Replacement is the synthetic value written in place of a found one. Marked says it carries the synthetic
// mark; SameShape says it keeps the original's separators, case and length.
type Replacement struct {
	Text      string
	Marked    bool
	SameShape bool
}

// Replace returns a synthetic value for raw, a value as written in text and found by d. The same detector and
// value always give the same replacement. ok is false when d has no generator or no generated value fits.
func Replace(d detectors.Detector, raw string) (Replacement, bool) {
	gen, ok := synth.For(d.Name)
	if !ok {
		return Replacement{}, false
	}
	original := detectors.Canonical(raw)
	r := rand.New(rand.NewSource(seed(d.Name, original)))
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
	shaped := 0
	for i := 0; i < maxTries; i++ {
		cand := gen(r)
		c := Replacement{Text: cand}
		if out, ok := layout(raw, cand); ok {
			c = Replacement{Text: out, SameShape: true}
			shaped++
		}
		c.Marked = detectors.Synthetic(c.Text)
		if better(c) && fits(d, c.Text) && detectors.Canonical(c.Text) != original {
			best, found = c, true
			if best.Marked && best.SameShape {
				return best, true
			}
		}
		if best.Marked && (shaped >= enoughShaped || i >= 1000 && shaped == 0) {
			break
		}
	}
	return best, found
}

// seed derives the generator's seed from the detector and the value, so the same value maps to the same
// replacement in every file and every run.
func seed(detector, canonical string) int64 {
	sum := sha256.Sum256([]byte("pii-eu fix\x00" + detector + "\x00" + canonical))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

// layout writes the letters and digits of cand into the positions of the letters and digits of orig, keeping
// orig's separators and the case of its letters. ok is false when they have different numbers of letters and
// digits.
func layout(orig, cand string) (string, bool) {
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
