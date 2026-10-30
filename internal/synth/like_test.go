package synth

import (
	"math/rand"
	"sort"
	"testing"
)

func shaperNames() []string {
	var out []string
	for n := range likes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// FuzzLike: a shaper takes any text found in a file without panicking, and its generator returns.
func FuzzLike(f *testing.F) {
	for _, s := range []string{"RO00 AAAA 0000 0000 0000 0000", "4000-0000-0000-0000", "+40 (0)721 000 000", "a.b@c.co.uk", "NL000000000B01", "2000101400000", "0041010000", "2a02:1::1", "93.184.216.34", "02:00:00:00:00:01", "0x", "bc1", "", "@", "ß"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, sample string) {
		for _, name := range shaperNames() {
			if g, ok := Like(name, sample); ok {
				g(rand.New(rand.NewSource(1)))
			}
		}
	})
}

// TestLikeKeepsSeparators: values from a shaper have as many letters and digits as the sample, for samples of
// the usual lengths, so fix can lay them over the sample's separators.
func TestLikeKeepsSeparators(t *testing.T) {
	for _, c := range []struct{ name, sample string }{
		{"iban", "RO00 AAAA 0000 0000 0000 0000"}, {"card", "4000-0000-0000-0000"}, {"eu-phone", "+40 (0)721 000 000"},
		{"email", "ana.pop12@firma.co.uk"}, {"eu-vat", "ATU00000000"}, {"ro-cnp", "8000101400000"}, {"bg-egn", "0021010010"},
		{"ip-address", "2a02:1234::1"}, {"mac-address", "02-00-00-00-00-01"}, {"gps-coordinates", "lat=45.1234 lng=-122.98765"},
		{"advertising-id", "6f9619ff-8b86-4011-b42d-00c04fc964ff"}, {"de-id-card", "L01X00T478"}, {"licence-plate", "AB 07 XYZ"},
	} {
		g, ok := Like(c.name, c.sample)
		if !ok {
			t.Errorf("%s: no generator", c.name)
			continue
		}
		r := rand.New(rand.NewSource(1))
		for range 100 {
			if v := g(r); len(alnums(v)) != len(alnums(c.sample)) {
				t.Errorf("%s: %d letters and digits, want %d", c.name, len(alnums(v)), len(alnums(c.sample)))
				break
			}
		}
	}
}
