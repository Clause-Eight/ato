package synth

import (
	"math/rand"
	"strings"
)

// w3wWord makes up a lower-case word of two or three syllables; three of them make a well-formed address.
func w3wWord(r *rand.Rand) string {
	var b strings.Builder
	for i := 0; i < 2+r.Intn(2); i++ {
		b.WriteString(handleSyllables[r.Intn(len(handleSyllables))])
	}
	return b.String()
}

func init() {
	Register("what3words", func(r *rand.Rand) string {
		triple := w3wWord(r) + "." + w3wWord(r) + "." + w3wWord(r)
		switch r.Intn(4) {
		case 0:
			return "https://w3w.co/" + triple
		case 1:
			return "https://what3words.com/" + triple
		}
		return "///" + triple
	})
	RegisterLike("what3words", likeAfterLast("/"))
}
