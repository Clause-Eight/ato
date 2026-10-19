package synth

import "math/rand"

func init() {
	// EHIC card number: 80, an ISO 3166-1 numeric country code, five issuer digits, a ten-digit serial.
	Register("ehic", func(r *rand.Rand) string {
		countries := []string{"040", "056", "100", "191", "203", "250", "276", "348", "380", "528", "616", "642", "724", "752"}
		issuer := "80" + countries[r.Intn(len(countries))] + Digits(r, 5)
		serial := Pad(r.Intn(100_000_000), 10)
		switch r.Intn(3) {
		case 0:
			return issuer + " " + serial
		case 1:
			return issuer[:5] + " " + issuer[5:] + " " + serial
		}
		return issuer + serial
	})
}
