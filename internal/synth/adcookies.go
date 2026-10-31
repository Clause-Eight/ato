package synth

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// unixBetween returns a random time between the first days of two years, in seconds.
func unixBetween(r *rand.Rand, from, to int) int64 {
	a := time.Date(from, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	b := time.Date(to, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	return a + r.Int63n(b-a)
}

func init() {
	// GA1.<domain depth>.<31-bit random>.<first visit, seconds>
	Register("ga-client-id", func(r *rand.Rand) string {
		return fmt.Sprintf("GA1.%d.%d.%d", 1+r.Intn(3), 1+r.Int63n(2147483646), unixBetween(r, 2013, 2026))
	})
	// fb.<subdomain index>.<creation, milliseconds>.<random number or fbclid>
	Register("meta-pixel-cookie", func(r *rand.Rand) string {
		ms := unixBetween(r, 2018, 2026)*1000 + r.Int63n(1000)
		if r.Intn(2) == 0 {
			return fmt.Sprintf("fb.%d.%d.%d", r.Intn(3), ms, 100000000+r.Int63n(1900000000))
		}
		click := []byte(fromSet(r, 40+r.Intn(30), base64URL))
		click[0] = 'I' // never starts or ends with a separator
		click[len(click)-1] = 'w'
		return fmt.Sprintf("fb.%d.%d.%s", r.Intn(3), ms, click)
	})
	RegisterLike("ga-client-id", likeTimedCookie(3, 4))
	RegisterLike("meta-pixel-cookie", likeTimedCookie(2, 5))
}

// likeTimedCookie keeps the version and domain parts and the first keep digits of the time (dotted part
// timePart), so the time moves by days at most and stays a plausible date; the other parts are varied.
func likeTimedCookie(timePart, keep int) Shaper {
	return func(sample string) Generator {
		parts := strings.SplitN(sample, ".", 4)
		if len(parts) != 4 || len(parts[timePart]) <= keep {
			return nil
		}
		return func(r *rand.Rand) string {
			out := append([]string{}, parts...)
			for i := 2; i < 4; i++ {
				if i == timePart {
					out[i] = parts[i][:keep] + vary(r, parts[i][keep:], "")
				} else {
					out[i] = vary(r, parts[i], "")
				}
			}
			return strings.Join(out, ".")
		}
	}
}
