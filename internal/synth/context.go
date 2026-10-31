package synth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
)

func init() {
	Register("card-security-code", func(r *rand.Rand) string {
		if r.Intn(5) == 0 {
			return Digits(r, 4)
		}
		return Digits(r, 3)
	})
	Register("card-expiry", func(r *rand.Rand) string {
		month, year := 1+r.Intn(12), 26+r.Intn(10)
		switch r.Intn(3) {
		case 0:
			return fmt.Sprintf("%02d/20%02d", month, year)
		case 1:
			return fmt.Sprintf("%02d%02d", month, year)
		}
		return fmt.Sprintf("%02d/%02d", month, year)
	})
	Register("hashed-contact", func(r *rand.Rand) string {
		n := 64
		if r.Intn(4) == 0 {
			n = 32
		}
		return Hex(r, n)
	})
	Register("jwt-personal", JWT)
}

// Hex returns n random lower-case hex digits.
func Hex(r *rand.Rand, n int) string {
	const digits = "0123456789abcdef"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(digits[r.Intn(16)])
	}
	return b.String()
}

// JWT returns a token whose payload names a made-up person: an email address, and a name or a phone number.
// The signature is random bytes.
func JWT(r *rand.Rand) string {
	seg := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	claims := map[string]any{
		"sub":   Digits(r, 10),
		"email": fmt.Sprintf("%s.%s%d@%s.eu", Letters(r, 5, "abcdefghij"), Letters(r, 4, "klmnoprst"), r.Intn(100), Letters(r, 6, "abcdefgh")),
		"iat":   1_700_000_000 + r.Intn(100_000_000),
	}
	if r.Intn(2) == 0 {
		claims["given_name"] = Letters(r, 1, "") + Letters(r, 5, "aeiounrst")
	} else {
		claims["phone_number"] = "+40 7" + Digits(r, 8)
	}
	sig := make([]byte, 32)
	r.Read(sig)
	return seg(map[string]string{"alg": "HS256", "typ": "JWT"}) + "." + seg(claims) + "." + base64.RawURLEncoding.EncodeToString(sig)
}
