package synth

import (
	"fmt"
	"math/rand"
	"strings"
)

// handleSyllables make up handles and words, so no generated profile or address is meant to be anyone's.
var handleSyllables = []string{"ka", "lo", "mi", "ren", "tas", "vo", "dra", "nel", "pi", "sor", "ul", "bex", "fin", "go"}

// handle joins two or three made-up words with sep, sometimes followed by a number.
func handle(r *rand.Rand, sep string) string {
	var parts []string
	for i := 0; i < 2+r.Intn(2); i++ {
		parts = append(parts, handleSyllables[r.Intn(len(handleSyllables))]+handleSyllables[r.Intn(len(handleSyllables))])
	}
	h := strings.Join(parts, sep)
	if r.Intn(2) == 0 {
		h += fmt.Sprint(r.Intn(100))
	}
	return h
}

func init() {
	Register("linkedin-profile", func(r *rand.Rand) string {
		host := []string{"https://www.linkedin.com", "https://linkedin.com", "linkedin.com", "https://de.linkedin.com", "https://ro.linkedin.com"}[r.Intn(5)]
		slug := handle(r, "-")
		if r.Intn(3) == 0 {
			slug += "-" + fromSet(r, 8, "0123456789abcdef") // the suffix LinkedIn adds to a default custom URL
		}
		return host + "/in/" + slug
	})
	Register("social-profile", func(r *rand.Rand) string {
		scheme := []string{"https://", "https://www.", ""}[r.Intn(3)]
		switch r.Intn(5) {
		case 0:
			return scheme + "facebook.com/" + handle(r, ".")
		case 1:
			return scheme + "facebook.com/profile.php?id=1000" + Digits(r, 11)
		case 2:
			return scheme + "instagram.com/" + handle(r, "_")
		case 3:
			h := handle(r, "_")
			if len(h) > 15 {
				h = strings.TrimRight(h[:15], "_")
			}
			return scheme + []string{"x.com/", "twitter.com/"}[r.Intn(2)] + h
		}
		return scheme + "tiktok.com/@" + handle(r, ".")
	})
	RegisterLike("linkedin-profile", likeAfterLast("/"))
	RegisterLike("social-profile", likeAfterLast("/=@"))
}

// likeAfterLast keeps everything up to the last of the cut characters (the site and the path to the account)
// and varies the letters and digits after it.
func likeAfterLast(cut string) Shaper {
	return func(sample string) Generator {
		i := strings.LastIndexAny(sample, cut)
		if i < 0 || i == len(sample)-1 {
			return nil
		}
		return func(r *rand.Rand) string { return sample[:i+1] + vary(r, sample[i+1:], "") }
	}
}
