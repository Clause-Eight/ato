package synth

import (
	"math/rand"
	"strings"
)

const base64URL = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// fromSet returns n random characters from set.
func fromSet(r *rand.Rand, n int, set string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(set[r.Intn(len(set))])
	}
	return b.String()
}

func init() {
	// An installation ID (header bits 0111, so c-f first), a colon, and a registration part starting APA91b.
	Register("fcm-token", func(r *rand.Rand) string {
		reg := "APA91b" + fromSet(r, 134+r.Intn(20), base64URL)
		if r.Intn(4) == 0 {
			return reg // an older GCM token, without an installation ID
		}
		return fromSet(r, 1, "cdef") + fromSet(r, 21, base64URL) + ":" + reg
	})
	// 32 random bytes in hex, as apps print the token.
	Register("apns-token", func(r *rand.Rand) string {
		t := fromSet(r, 64, "0123456789abcdef")
		if r.Intn(3) == 0 {
			return strings.ToUpper(t)
		}
		return t
	})
	RegisterLike("fcm-token", likeFCM)
	RegisterLike("apns-token", func(sample string) Generator {
		if len(sample) != 64 {
			return nil
		}
		return func(r *rand.Rand) string { return vary(r, sample, "ABCDEF") }
	})
}

// likeFCM keeps the APA91b marker and the installation ID's header character class, and varies the rest.
func likeFCM(sample string) Generator {
	i := strings.Index(sample, "APA91b")
	if i < 0 {
		return nil
	}
	head, tail := sample[:i], sample[i+len("APA91b"):]
	return func(r *rand.Rand) string {
		h := head
		if h != "" {
			h = vary(r, h[:1], "CDEF") + vary(r, h[1:], "")
		}
		return h + "APA91b" + vary(r, tail, "")
	}
}
